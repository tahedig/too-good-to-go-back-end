package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tahedig/too-good-to-go/backend/internal/domain/enum"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/model"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/repository"
	"github.com/tahedig/too-good-to-go/backend/pkg/security"
)

type vendorApprovalRepository struct {
	pool *pgxpool.Pool
}

// NewVendorApprovalRepository reads and decides on stores (stores.status)
// and records each decision in store_status_history.
func NewVendorApprovalRepository(pool *pgxpool.Pool) repository.VendorApprovalRepository {
	return &vendorApprovalRepository{pool: pool}
}

// vendorStoreSelect loads stores with their owner, category, last status
// change and, for a rejected or suspended store, the decision that put it
// there. Callers append WHERE conditions after "s.deleted_at IS NULL".
const vendorStoreSelect = `
SELECT s.public_id::text, s.name, coalesce(u.full_name, ''), coalesce(c.name_fa, ''), s.address_line,
       coalesce(s.phone, v.contact_phone, u.phone, ''), s.biography, coalesce(s.cover_image_url, s.logo_url),
       s.created_at, s.status::text,
       last.created_at,
       d.action, d.reason_code, d.message, d.created_at, d.decided_by
FROM stores s
JOIN vendors v ON v.id = s.vendor_id
JOIN users u ON u.id = v.owner_user_id
LEFT JOIN categories c ON c.id = s.category_id
LEFT JOIN LATERAL (
    SELECT h.created_at FROM store_status_history h
    WHERE h.store_id = s.id
    ORDER BY h.created_at DESC, h.id DESC LIMIT 1
) last ON true
LEFT JOIN LATERAL (
    SELECT h.action, h.reason_code, h.message, h.created_at,
           coalesce(nullif(a.full_name, ''), a.phone, '') AS decided_by
    FROM store_status_history h
    LEFT JOIN users a ON a.id = h.admin_id
    WHERE h.store_id = s.id AND h.to_status = s.status AND s.status IN ('rejected', 'suspended')
    ORDER BY h.created_at DESC, h.id DESC LIMIT 1
) d ON true
WHERE s.deleted_at IS NULL`

// listFilter is shared by the page query and its total count.
// $1: statuses as text[] (NULL = all); $2: LIKE pattern (” = no search).
const listFilter = `
  AND ($1::text[] IS NULL OR s.status::text = ANY($1::text[]))
  AND ($2 = '' OR s.name ILIKE $2 OR coalesce(u.full_name, '') ILIKE $2
       OR coalesce(c.name_fa, '') ILIKE $2 OR s.address_line ILIKE $2 OR s.city ILIKE $2)`

const countFiltered = `
SELECT count(*)
FROM stores s
JOIN vendors v ON v.id = s.vendor_id
JOIN users u ON u.id = v.owner_user_id
LEFT JOIN categories c ON c.id = s.category_id
WHERE s.deleted_at IS NULL` + listFilter

const countByStatus = `SELECT status::text, count(*) FROM stores WHERE deleted_at IS NULL GROUP BY status`

func (r *vendorApprovalRepository) List(ctx context.Context, q repository.VendorStoreQuery) (*repository.VendorStorePage, error) {
	var statuses []string
	for _, s := range q.Statuses {
		statuses = append(statuses, string(s))
	}
	pattern := ""
	if q.Search != "" {
		pattern = "%" + escapeLike(q.Search) + "%"
	}

	page := &repository.VendorStorePage{Items: []model.VendorStore{}, Counts: map[enum.StoreStatus]int{}}
	for _, s := range enum.StoreStatuses {
		page.Counts[s] = 0
	}
	// One snapshot, so the items, total and counts agree with each other.
	err := pgx.BeginTxFunc(ctx, r.pool, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly},
		func(tx pgx.Tx) error {
			rows, err := tx.Query(ctx, vendorStoreSelect+listFilter+`
ORDER BY s.created_at DESC, s.id DESC
LIMIT $3 OFFSET $4`, statuses, pattern, q.Limit, q.Offset)
			if err != nil {
				return err
			}
			if page.Items, err = pgx.CollectRows(rows, scanVendorStore); err != nil {
				return err
			}
			if err := tx.QueryRow(ctx, countFiltered, statuses, pattern).Scan(&page.Total); err != nil {
				return err
			}
			rows, err = tx.Query(ctx, countByStatus)
			if err != nil {
				return err
			}
			defer rows.Close()
			for rows.Next() {
				var status string
				var n int
				if err := rows.Scan(&status, &n); err != nil {
					return err
				}
				page.Counts[enum.StoreStatus(status)] = n
			}
			return rows.Err()
		})
	if err != nil {
		return nil, fmt.Errorf("list vendor stores: %w", err)
	}
	return page, nil
}

func (r *vendorApprovalRepository) Get(ctx context.Context, storeID string) (*model.VendorStore, error) {
	return getVendorStore(ctx, r.pool, storeID)
}

func getVendorStore(ctx context.Context, q querier, storeID string) (*model.VendorStore, error) {
	if !security.IsID(storeID) {
		return nil, repository.ErrStoreNotFound
	}
	rows, err := q.Query(ctx, vendorStoreSelect+` AND s.public_id = $1::uuid`, storeID)
	if err != nil {
		return nil, fmt.Errorf("get vendor store: %w", err)
	}
	store, err := pgx.CollectExactlyOneRow(rows, scanVendorStore)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, repository.ErrStoreNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get vendor store: %w", err)
	}
	return &store, nil
}

// transitionSQL changes the status only if the store is still in the
// expected one, so of two admins deciding at once exactly one succeeds.
// Approving records the approving admin; reactivating keeps the original
// approval (or fills it for a store that never had one).
const transitionSQL = `
WITH admin AS (SELECT id FROM users WHERE public_id = $5::uuid)
UPDATE stores SET
    status           = $3::store_status,
    rejection_reason = CASE WHEN $3 = 'rejected' THEN $6 ELSE rejection_reason END,
    approved_by      = CASE WHEN $4 = 'approve' THEN (SELECT id FROM admin)
                            WHEN $3 = 'approved' THEN coalesce(approved_by, (SELECT id FROM admin))
                            ELSE approved_by END,
    approved_at      = CASE WHEN $4 = 'approve' THEN now()
                            WHEN $3 = 'approved' THEN coalesce(approved_at, now())
                            ELSE approved_at END
WHERE public_id = $1::uuid AND status = $2::store_status AND deleted_at IS NULL
RETURNING id`

const recordDecisionSQL = `
INSERT INTO store_status_history (store_id, from_status, to_status, action, reason_code, message, admin_id)
VALUES ($1, $2::store_status, $3::store_status, $4, $5, $6, (SELECT id FROM users WHERE public_id = $7::uuid))`

func (r *vendorApprovalRepository) Transition(ctx context.Context, t model.StoreTransition) (*model.VendorStore, error) {
	if !security.IsID(t.StoreID) {
		return nil, repository.ErrStoreNotFound
	}
	var reasonCode, message *string
	if t.ReasonCode != "" {
		reasonCode, message = &t.ReasonCode, &t.Message
	}

	var store *model.VendorStore
	err := pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		var id int64
		err := tx.QueryRow(ctx, transitionSQL, t.StoreID, string(t.From), string(t.To), string(t.Action),
			t.AdminUserID, message).Scan(&id)
		if errors.Is(err, pgx.ErrNoRows) {
			var exists bool
			if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM stores WHERE public_id = $1::uuid AND deleted_at IS NULL)`,
				t.StoreID).Scan(&exists); err != nil {
				return err
			}
			if !exists {
				return repository.ErrStoreNotFound
			}
			return repository.ErrStoreStatusChanged
		}
		if err != nil {
			return fmt.Errorf("update store status: %w", err)
		}
		if _, err := tx.Exec(ctx, recordDecisionSQL, id, string(t.From), string(t.To), string(t.Action),
			reasonCode, message, t.AdminUserID); err != nil {
			return fmt.Errorf("record store decision: %w", err)
		}
		store, err = getVendorStore(ctx, tx, t.StoreID)
		return err
	})
	if err != nil {
		return nil, err
	}
	return store, nil
}

func scanVendorStore(row pgx.CollectableRow) (model.VendorStore, error) {
	var (
		s                                      model.VendorStore
		status                                 string
		action, reasonCode, message, decidedBy *string
		decidedAt                              *time.Time
	)
	if err := row.Scan(&s.ID, &s.StoreName, &s.OwnerName, &s.Category, &s.Address, &s.Phone, &s.Description,
		&s.ImageURL, &s.SubmittedAt, &status, &s.StatusUpdatedAt,
		&action, &reasonCode, &message, &decidedAt, &decidedBy); err != nil {
		return s, err
	}
	s.Status = enum.StoreStatus(status)
	s.SubmittedAt = s.SubmittedAt.UTC()
	if s.StatusUpdatedAt != nil {
		t := s.StatusUpdatedAt.UTC()
		s.StatusUpdatedAt = &t
	}
	if action != nil && reasonCode != nil && message != nil && decidedAt != nil {
		s.Decision = &model.StoreDecision{
			Action: enum.StoreAction(*action), ReasonCode: *reasonCode, Message: *message,
			DecidedAt: decidedAt.UTC(), DecidedBy: deref(decidedBy),
		}
	}
	return s, nil
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// escapeLike makes s match literally inside an ILIKE pattern.
func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}

package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tahedig/too-good-to-go/backend/internal/domain/model"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/repository"
)

type storeRequestRepository struct {
	pool *pgxpool.Pool
}

// NewStoreRequestRepository returns a PostgreSQL-backed StoreRequestRepository.
func NewStoreRequestRepository(pool *pgxpool.Pool) repository.StoreRequestRepository {
	return &storeRequestRepository{pool: pool}
}

func storeRequestStatusFa(status string) string {
	switch status {
	case "pending_review":
		return "در انتظار بررسی"
	case "approved":
		return "تأییدشده"
	case "rejected":
		return "رد شده"
	default:
		return status
	}
}

func (r *storeRequestRepository) ListByUser(ctx context.Context, userID int64) ([]*model.StoreRequest, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT sr.id, sr.public_id::text, sr.vendor_id, sr.store_name, sr.category_id,
		       COALESCE(c.name_fa, ''), sr.owner_name, sr.address, sr.status::text,
		       sr.created_at, sr.updated_at
		FROM store_requests sr
		JOIN vendors v ON v.id = sr.vendor_id
		LEFT JOIN categories c ON c.id = sr.category_id
		WHERE v.owner_user_id = $1
		ORDER BY sr.created_at DESC, sr.id DESC`, userID)
	if err != nil {
		return nil, fmt.Errorf("list store requests: %w", err)
	}
	defer rows.Close()

	var out []*model.StoreRequest
	for rows.Next() {
		var sr model.StoreRequest
		if err := rows.Scan(
			&sr.ID, &sr.PublicID, &sr.VendorID, &sr.StoreName, &sr.CategoryID,
			&sr.CategoryName, &sr.OwnerName, &sr.Address, &sr.Status,
			&sr.CreatedAt, &sr.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan store request: %w", err)
		}
		sr.StatusFa = storeRequestStatusFa(sr.Status)
		out = append(out, &sr)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate store requests: %w", err)
	}
	return out, nil
}

func (r *storeRequestRepository) GetByPublicIDAndUser(ctx context.Context, publicID string, userID int64) (*model.StoreRequest, error) {
	if _, err := uuid.Parse(publicID); err != nil {
		return nil, nil
	}
	var sr model.StoreRequest
	err := r.pool.QueryRow(ctx, `
		SELECT sr.id, sr.public_id::text, sr.vendor_id, sr.store_name, sr.category_id,
		       COALESCE(c.name_fa, ''), sr.owner_name, sr.address, sr.status::text,
		       sr.created_at, sr.updated_at
		FROM store_requests sr
		JOIN vendors v ON v.id = sr.vendor_id
		LEFT JOIN categories c ON c.id = sr.category_id
		WHERE sr.public_id = $1::uuid AND v.owner_user_id = $2`,
		publicID, userID,
	).Scan(
		&sr.ID, &sr.PublicID, &sr.VendorID, &sr.StoreName, &sr.CategoryID,
		&sr.CategoryName, &sr.OwnerName, &sr.Address, &sr.Status,
		&sr.CreatedAt, &sr.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get store request: %w", err)
	}
	sr.StatusFa = storeRequestStatusFa(sr.Status)
	return &sr, nil
}

func (r *storeRequestRepository) Create(ctx context.Context, req *model.StoreRequest) error {
	return pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		// StoreRequestService sets req.VendorID = users.id; resolve or create the vendors row.
		var vendorID int64
		err := tx.QueryRow(ctx, `SELECT id FROM vendors WHERE owner_user_id = $1 ORDER BY id ASC LIMIT 1`, req.VendorID).Scan(&vendorID)
		if errors.Is(err, pgx.ErrNoRows) {
			bizName := req.StoreName
			if bizName == "" {
				bizName = "فروشگاه جدید"
			}
			if err := tx.QueryRow(ctx, `
				INSERT INTO vendors (owner_user_id, business_name)
				VALUES ($1, $2)
				RETURNING id`, req.VendorID, bizName).Scan(&vendorID); err != nil {
				return fmt.Errorf("create vendor for user: %w", err)
			}
		} else if err != nil {
			return fmt.Errorf("lookup vendor for user: %w", err)
		}

		ownerName := req.OwnerName
		if ownerName == "" {
			ownerName = req.StoreName
		}

		if err := tx.QueryRow(ctx, `
			INSERT INTO store_requests (vendor_id, store_name, owner_name, category_id, address, status)
			VALUES ($1, $2, $3, $4, $5, 'pending_review')
			RETURNING id, public_id::text, created_at, updated_at`,
			vendorID, req.StoreName, ownerName, req.CategoryID, req.Address,
		).Scan(&req.ID, &req.PublicID, &req.CreatedAt, &req.UpdatedAt); err != nil {
			return fmt.Errorf("insert store request: %w", err)
		}
		req.VendorID = vendorID
		req.Status = "pending_review"
		req.StatusFa = storeRequestStatusFa(req.Status)
		return nil
	})
}

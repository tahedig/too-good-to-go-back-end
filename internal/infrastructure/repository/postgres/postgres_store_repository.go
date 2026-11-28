package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tahedig/too-good-to-go/backend/internal/domain/model"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/repository"
	"github.com/tahedig/too-good-to-go/backend/pkg/security"
)

type storeRepository struct {
	pool *pgxpool.Pool
}

var (
	_ repository.StoreRepository                = (*storeRepository)(nil)
	_ repository.AdminStoreManagementRepository = (*storeRepository)(nil)
)

func NewStoreRepository(pool *pgxpool.Pool) repository.StoreRepository {
	return &storeRepository{pool: pool}
}

func NewAdminStoreManagementRepository(pool *pgxpool.Pool) repository.AdminStoreManagementRepository {
	return &storeRepository{pool: pool}
}

// IsActive is derived from stores.status = 'approved'. accepts_orders is kept
// independent because it is the store's pause switch, not approval state.
const storeSelect = `
SELECT s.id, s.public_id::text, s.vendor_id, s.status::text, s.category_id,
       coalesce(c.name_fa, ''), coalesce(nullif(s.owner_name, ''), u.full_name, ''),
       s.name, s.slug, s.biography, s.pickup_instructions, s.phone,
       coalesce(s.phone, v.contact_phone, u.phone, ''),
       s.city, s.district, s.address_line,
       ST_Y(s.location::geometry), ST_X(s.location::geometry),
       s.logo_url, s.cover_image_url,
       coalesce((SELECT count(*)::int FROM offers o WHERE o.store_id = s.id AND o.status = 'active'), 0),
       (s.status = 'approved'), s.accepts_orders, s.rating_avg::float8, s.rating_count,
       s.trust_score::float8, s.meals_saved, s.created_at, s.updated_at, s.deleted_at
FROM stores s
JOIN vendors v ON v.id = s.vendor_id
JOIN users u ON u.id = v.owner_user_id
LEFT JOIN categories c ON c.id = s.category_id`

func (r *storeRepository) ListByUser(ctx context.Context, userID int64) ([]*model.Store, error) {
	rows, err := r.pool.Query(ctx, storeSelect+`
WHERE v.owner_user_id = $1 AND s.deleted_at IS NULL
ORDER BY s.created_at DESC, s.id DESC`, userID)
	if err != nil {
		return nil, fmt.Errorf("list stores by vendor: %w", err)
	}
	stores, err := pgx.CollectRows(rows, scanStore)
	if err != nil {
		return nil, fmt.Errorf("scan stores by vendor: %w", err)
	}
	if err := loadStoreRelations(ctx, r.pool, stores); err != nil {
		return nil, err
	}
	return stores, nil
}

func (r *storeRepository) GetByPublicIDAndUser(ctx context.Context, publicID string, userID int64) (*model.Store, error) {
	if !security.IsID(publicID) {
		return nil, nil
	}
	store, err := getStore(ctx, r.pool, storeSelect+`
WHERE s.public_id = $1::uuid AND v.owner_user_id = $2 AND s.deleted_at IS NULL`, publicID, userID)
	if err != nil || store == nil {
		return store, err
	}
	if err := loadStoreRelations(ctx, r.pool, []*model.Store{store}); err != nil {
		return nil, err
	}
	return store, nil
}

func (r *storeRepository) GetPublicByPublicID(ctx context.Context, publicID string) (*model.Store, error) {
	if !security.IsID(publicID) {
		return nil, nil
	}
	store, err := getStore(ctx, r.pool, storeSelect+`
WHERE s.public_id = $1::uuid AND s.status = 'approved' AND s.deleted_at IS NULL`, publicID)
	if err != nil || store == nil {
		return store, err
	}
	if err := loadStoreRelations(ctx, r.pool, []*model.Store{store}); err != nil {
		return nil, err
	}
	return store, nil
}

func (r *storeRepository) GetByPublicID(ctx context.Context, publicID string) (*model.Store, error) {
	if !security.IsID(publicID) {
		return nil, repository.ErrStoreNotFound
	}
	store, err := getStore(ctx, r.pool, storeSelect+`
WHERE s.public_id = $1::uuid AND s.deleted_at IS NULL`, publicID)
	if err != nil {
		return nil, err
	}
	if store == nil {
		return nil, repository.ErrStoreNotFound
	}
	if err := loadStoreRelations(ctx, r.pool, []*model.Store{store}); err != nil {
		return nil, err
	}
	return store, nil
}

func getStore(ctx context.Context, db DB, query string, args ...any) (*model.Store, error) {
	rows, err := db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("get store: %w", err)
	}
	store, err := pgx.CollectExactlyOneRow(rows, scanStore)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scan store: %w", err)
	}
	return store, nil
}

func (r *storeRepository) ListAdmin(ctx context.Context, filter repository.AdminStoreFilter) (*repository.AdminStorePage, error) {
	if filter.Sort != "oldest" {
		filter.Sort = "newest"
	}
	search := ""
	if filter.Search != "" {
		search = "%" + escapeLike(strings.TrimSpace(filter.Search)) + "%"
	}
	params := []any{
		filter.Status,
		filter.CategoryID,
		strings.TrimSpace(filter.City),
		filter.AcceptsOrders,
		search,
		filter.IncludeDeleted,
	}
	page := &repository.AdminStorePage{Items: []*model.Store{}}

	err := pgx.BeginTxFunc(ctx, r.pool, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly},
		func(tx pgx.Tx) error {
			if err := tx.QueryRow(ctx, `
SELECT count(*)
FROM stores s
JOIN vendors v ON v.id = s.vendor_id
JOIN users u ON u.id = v.owner_user_id
WHERE ($1::text = '' OR s.status::text = $1)
  AND ($2::smallint IS NULL OR s.category_id = $2)
  AND ($3::text = '' OR s.city = $3)
  AND ($4::boolean IS NULL OR s.accepts_orders = $4)
  AND ($5::text = '' OR s.name ILIKE $5 ESCAPE '\' OR s.slug ILIKE $5 ESCAPE '\'
       OR s.city ILIKE $5 ESCAPE '\' OR coalesce(s.owner_name, '') ILIKE $5 ESCAPE '\'
       OR coalesce(u.full_name, '') ILIKE $5 ESCAPE '\')
  AND ($6::boolean OR s.deleted_at IS NULL)`, params...).Scan(&page.Total); err != nil {
				return fmt.Errorf("count admin stores: %w", err)
			}

			cursorTime := (*string)(nil)
			var cursorID *int64
			if filter.Cursor != nil {
				value := filter.Cursor.CreatedAt.UTC().Format("2006-01-02T15:04:05.999999Z07:00")
				cursorTime = &value
				cursorID = &filter.Cursor.ID
			}
			order := "DESC"
			operator := "<"
			if filter.Sort == "oldest" {
				order = "ASC"
				operator = ">"
			}
			query := fmt.Sprintf(`%s
WHERE ($1::text = '' OR s.status::text = $1)
  AND ($2::smallint IS NULL OR s.category_id = $2)
  AND ($3::text = '' OR s.city = $3)
  AND ($4::boolean IS NULL OR s.accepts_orders = $4)
  AND ($5::text = '' OR s.name ILIKE $5 ESCAPE '\' OR s.slug ILIKE $5 ESCAPE '\'
       OR s.city ILIKE $5 ESCAPE '\' OR coalesce(s.owner_name, '') ILIKE $5 ESCAPE '\'
       OR coalesce(u.full_name, '') ILIKE $5 ESCAPE '\')
  AND ($6::boolean OR s.deleted_at IS NULL)
  AND ($7::timestamptz IS NULL OR (s.created_at, s.id) %s ($7::timestamptz, $8::bigint))
ORDER BY s.created_at %s, s.id %s
LIMIT $9`, storeSelect, operator, order, order)
			args := append(append([]any{}, params...), cursorTime, cursorID, filter.Limit)
			rows, err := tx.Query(ctx, query, args...)
			if err != nil {
				return fmt.Errorf("list admin stores: %w", err)
			}
			items, err := pgx.CollectRows(rows, scanStore)
			if err != nil {
				return fmt.Errorf("scan admin stores: %w", err)
			}
			if err := loadStoreRelations(ctx, tx, items); err != nil {
				return err
			}
			page.Items = items
			return nil
		})
	if err != nil {
		return nil, err
	}
	return page, nil
}

func (r *storeRepository) CategoryIsActive(ctx context.Context, categoryID int16) (bool, error) {
	var active bool
	if err := r.pool.QueryRow(ctx, `SELECT coalesce((SELECT is_active FROM categories WHERE id = $1), false)`, categoryID).Scan(&active); err != nil {
		return false, fmt.Errorf("check category active: %w", err)
	}
	return active, nil
}

// Update writes only permitted profile fields. In particular it never updates
// vendor ownership, public identity, approval state, or computed metrics.
func (r *storeRepository) Update(ctx context.Context, store *model.Store) error {
	if store == nil || store.ID <= 0 {
		return repository.ErrStoreNotFound
	}
	err := pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `
UPDATE stores
SET category_id = $2, name = $3, biography = $4, pickup_instructions = $5,
    phone = $6, city = $7, district = $8, address_line = $9,
    location = ST_SetSRID(ST_MakePoint($11, $10), 4326)::geography,
    logo_url = $12, cover_image_url = $13, accepts_orders = $14
WHERE id = $1 AND deleted_at IS NULL`,
			store.ID, store.CategoryID, store.Name, store.Biography, store.PickupInstructions,
			store.Phone, store.City, store.District, store.AddressLine,
			store.Location.Latitude, store.Location.Longitude, store.LogoURL, store.CoverImageURL, store.AcceptsOrders)
		if err != nil {
			return fmt.Errorf("update store profile: %w", err)
		}
		if tag.RowsAffected() == 0 {
			return repository.ErrStoreNotFound
		}

		if _, err := tx.Exec(ctx, `DELETE FROM store_working_hours WHERE store_id = $1`, store.ID); err != nil {
			return fmt.Errorf("replace store working hours: %w", err)
		}
		for _, hour := range store.WorkingHours {
			if _, err := tx.Exec(ctx, `
INSERT INTO store_working_hours (store_id, day_of_week, open_time, close_time, is_closed)
VALUES ($1, $2, NULLIF($3, '')::time, NULLIF($4, '')::time, $5)`,
				store.ID, hour.DayOfWeek, hour.OpenTime, hour.CloseTime, hour.IsClosed); err != nil {
				return fmt.Errorf("insert store working hour: %w", err)
			}
		}

		if _, err := tx.Exec(ctx, `DELETE FROM store_images WHERE store_id = $1`, store.ID); err != nil {
			return fmt.Errorf("replace store images: %w", err)
		}
		for _, image := range store.Images {
			if _, err := tx.Exec(ctx, `
INSERT INTO store_images (store_id, image_url, is_primary, sort_order)
VALUES ($1, $2, $3, $4)`, store.ID, strings.TrimSpace(image.URL), image.IsPrimary, int16(image.SortOrder)); err != nil {
				return fmt.Errorf("insert store image: %w", err)
			}
		}
		return nil
	})
	return err
}

func scanStore(row pgx.CollectableRow) (*model.Store, error) {
	store := &model.Store{}
	var status string
	if err := row.Scan(
		&store.ID, &store.PublicID, &store.VendorID, &status, &store.CategoryID,
		&store.CategoryName, &store.OwnerName, &store.Name, &store.Slug,
		&store.Biography, &store.PickupInstructions, &store.Phone, &store.ContactPhone,
		&store.City, &store.District, &store.AddressLine,
		&store.Location.Latitude, &store.Location.Longitude,
		&store.LogoURL, &store.CoverImageURL, &store.ActiveProductsCount,
		&store.IsActive, &store.AcceptsOrders, &store.RatingAvg, &store.RatingCount,
		&store.TrustScore, &store.MealsSaved, &store.CreatedAt, &store.UpdatedAt, &store.DeletedAt,
	); err != nil {
		return nil, err
	}
	store.VendorStatus = status
	return store, nil
}

func loadStoreRelations(ctx context.Context, db DB, stores []*model.Store) error {
	if len(stores) == 0 {
		return nil
	}
	ids := make([]int64, 0, len(stores))
	byID := make(map[int64]*model.Store, len(stores))
	for _, store := range stores {
		ids = append(ids, store.ID)
		byID[store.ID] = store
		store.WorkingHours = []model.WorkingHour{}
		store.Images = []model.StoreImage{}
	}

	rows, err := db.Query(ctx, `
SELECT store_id, day_of_week, coalesce(to_char(open_time, 'HH24:MI'), ''),
       coalesce(to_char(close_time, 'HH24:MI'), ''), is_closed
FROM store_working_hours
WHERE store_id = ANY($1::bigint[])
ORDER BY store_id, day_of_week`, ids)
	if err != nil {
		return fmt.Errorf("load store working hours: %w", err)
	}
	for rows.Next() {
		var id int64
		var hour model.WorkingHour
		if err := rows.Scan(&id, &hour.DayOfWeek, &hour.OpenTime, &hour.CloseTime, &hour.IsClosed); err != nil {
			rows.Close()
			return fmt.Errorf("scan store working hour: %w", err)
		}
		store := byID[id]
		if store != nil {
			store.WorkingHours = append(store.WorkingHours, hour)
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return fmt.Errorf("iterate store working hours: %w", err)
	}
	rows.Close()

	rows, err = db.Query(ctx, `
SELECT store_id, public_id::text, image_url, is_primary, sort_order
FROM store_images
WHERE store_id = ANY($1::bigint[])
ORDER BY store_id, sort_order, id`, ids)
	if err != nil {
		return fmt.Errorf("load store images: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var image model.StoreImage
		if err := rows.Scan(&id, &image.ID, &image.URL, &image.IsPrimary, &image.SortOrder); err != nil {
			return fmt.Errorf("scan store image: %w", err)
		}
		store := byID[id]
		if store != nil {
			store.Images = append(store.Images, image)
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate store images: %w", err)
	}
	return nil
}

package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tahedig/too-good-to-go/backend/internal/domain/model"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/repository"
)

type storeRepository struct {
	pool *pgxpool.Pool
}

// NewStoreRepository returns a PostgreSQL-backed StoreRepository.
func NewStoreRepository(pool *pgxpool.Pool) repository.StoreRepository {
	return &storeRepository{pool: pool}
}

const selectStoreCols = `
	s.id, s.public_id::text, s.vendor_id, s.status::text,
	s.category_id, COALESCE(c.name_fa, ''), s.owner_name,
	s.name, s.slug, s.biography, s.pickup_instructions,
	s.phone, s.city, s.district, s.address_line,
	ST_Y(s.location::geometry), ST_X(s.location::geometry),
	s.logo_url, s.cover_image_url,
	(s.deleted_at IS NULL) AS is_active, s.accepts_orders,
	s.rating_avg::float8, s.rating_count, s.trust_score::float8, s.meals_saved,
	s.created_at, s.updated_at, s.deleted_at,
	(SELECT count(*)::int FROM offers o
	  WHERE o.store_id = s.id AND o.deleted_at IS NULL AND o.status = 'active'
	    AND o.quantity_available > 0 AND o.pickup_end > now()) AS active_offers_count`

func scanStoreRow(row pgx.Row) (*model.Store, error) {
	var st model.Store
	err := row.Scan(
		&st.ID, &st.PublicID, &st.VendorID, &st.VendorStatus,
		&st.CategoryID, &st.CategoryName, &st.OwnerName,
		&st.Name, &st.Slug, &st.Biography, &st.PickupInstructions,
		&st.Phone, &st.City, &st.District, &st.AddressLine,
		&st.Location.Latitude, &st.Location.Longitude,
		&st.LogoURL, &st.CoverImageURL,
		&st.IsActive, &st.AcceptsOrders,
		&st.RatingAvg, &st.RatingCount, &st.TrustScore, &st.MealsSaved,
		&st.CreatedAt, &st.UpdatedAt, &st.DeletedAt,
		&st.ActiveProductsCount,
	)
	if err != nil {
		return nil, err
	}
	return &st, nil
}

func (r *storeRepository) ListByUser(ctx context.Context, userID int64) ([]*model.Store, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT `+selectStoreCols+`
		FROM stores s
		JOIN vendors v ON v.id = s.vendor_id
		LEFT JOIN categories c ON c.id = s.category_id
		WHERE v.owner_user_id = $1 AND s.deleted_at IS NULL
		ORDER BY s.created_at DESC, s.id DESC`, userID)
	if err != nil {
		return nil, fmt.Errorf("list stores by user: %w", err)
	}
	defer rows.Close()

	var stores []*model.Store
	for rows.Next() {
		st, err := scanStoreRow(rows)
		if err != nil {
			return nil, fmt.Errorf("scan store: %w", err)
		}
		stores = append(stores, st)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate stores: %w", err)
	}
	for _, st := range stores {
		if err := r.hydrateStoreDetails(ctx, st); err != nil {
			return nil, err
		}
	}
	return stores, nil
}

func (r *storeRepository) GetByPublicIDAndUser(ctx context.Context, publicID string, userID int64) (*model.Store, error) {
	if _, err := uuid.Parse(publicID); err != nil {
		return nil, nil
	}
	row := r.pool.QueryRow(ctx, `
		SELECT `+selectStoreCols+`
		FROM stores s
		JOIN vendors v ON v.id = s.vendor_id
		LEFT JOIN categories c ON c.id = s.category_id
		WHERE s.public_id = $1::uuid AND v.owner_user_id = $2 AND s.deleted_at IS NULL`,
		publicID, userID,
	)
	st, err := scanStoreRow(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get store by public_id and user: %w", err)
	}
	if err := r.hydrateStoreDetails(ctx, st); err != nil {
		return nil, err
	}
	return st, nil
}

func (r *storeRepository) GetPublicByPublicID(ctx context.Context, publicID string) (*model.Store, error) {
	if _, err := uuid.Parse(publicID); err != nil {
		return nil, nil
	}
	row := r.pool.QueryRow(ctx, `
		SELECT `+selectStoreCols+`
		FROM stores s
		LEFT JOIN categories c ON c.id = s.category_id
		WHERE s.public_id = $1::uuid AND s.deleted_at IS NULL`,
		publicID,
	)
	st, err := scanStoreRow(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get public store: %w", err)
	}
	if err := r.hydrateStoreDetails(ctx, st); err != nil {
		return nil, err
	}
	return st, nil
}

func (r *storeRepository) Update(ctx context.Context, st *model.Store) error {
	return pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `
			UPDATE stores SET
				name = $2,
				category_id = $3,
				owner_name = $4,
				biography = $5,
				pickup_instructions = $6,
				phone = $7,
				city = $8,
				district = $9,
				address_line = $10,
				location = ST_SetSRID(ST_MakePoint($11, $12), 4326)::geography,
				logo_url = $13,
				cover_image_url = $14,
				accepts_orders = $15
			WHERE id = $1 AND deleted_at IS NULL`,
			st.ID, st.Name, st.CategoryID, st.OwnerName, st.Biography,
			st.PickupInstructions, st.Phone, st.City, st.District, st.AddressLine,
			st.Location.Longitude, st.Location.Latitude,
			st.LogoURL, st.CoverImageURL, st.AcceptsOrders,
		)
		if err != nil {
			return fmt.Errorf("update store: %w", err)
		}
		if tag.RowsAffected() == 0 {
			return fmt.Errorf("store %d not found", st.ID)
		}

		for _, wh := range st.WorkingHours {
			var openTime, closeTime *string
			if !wh.IsClosed {
				ot, ct := wh.OpenTime, wh.CloseTime
				openTime, closeTime = &ot, &ct
			}
			if _, err := tx.Exec(ctx, `
				INSERT INTO store_working_hours (store_id, day_of_week, open_time, close_time, is_closed)
				VALUES ($1, $2, $3::time, $4::time, $5)
				ON CONFLICT (store_id, day_of_week) DO UPDATE SET
					open_time = EXCLUDED.open_time,
					close_time = EXCLUDED.close_time,
					is_closed = EXCLUDED.is_closed`,
				st.ID, wh.DayOfWeek, openTime, closeTime, wh.IsClosed,
			); err != nil {
				return fmt.Errorf("upsert working hours: %w", err)
			}
		}
		return nil
	})
}

var (
	dayNamesEn = [7]string{"saturday", "sunday", "monday", "tuesday", "wednesday", "thursday", "friday"}
	dayNamesFa = [7]string{"شنبه", "یکشنبه", "دوشنبه", "سه‌شنبه", "چهارشنبه", "پنج‌شنبه", "جمعه"}
)

func (r *storeRepository) hydrateStoreDetails(ctx context.Context, st *model.Store) error {
	whRows, err := r.pool.Query(ctx, `
		SELECT day_of_week, COALESCE(to_char(open_time, 'HH24:MI'), ''), COALESCE(to_char(close_time, 'HH24:MI'), ''), is_closed
		FROM store_working_hours
		WHERE store_id = $1
		ORDER BY day_of_week ASC`, st.ID)
	if err != nil {
		return fmt.Errorf("list store working hours: %w", err)
	}
	defer whRows.Close()

	for whRows.Next() {
		var wh model.WorkingHour
		if err := whRows.Scan(&wh.DayOfWeek, &wh.OpenTime, &wh.CloseTime, &wh.IsClosed); err != nil {
			return fmt.Errorf("scan working hour: %w", err)
		}
		if wh.DayOfWeek >= 0 && wh.DayOfWeek < 7 {
			wh.DayName = dayNamesEn[wh.DayOfWeek]
			wh.DayNameFa = dayNamesFa[wh.DayOfWeek]
		}
		st.WorkingHours = append(st.WorkingHours, wh)
	}
	if err := whRows.Err(); err != nil {
		return fmt.Errorf("iterate working hours: %w", err)
	}

	imgRows, err := r.pool.Query(ctx, `
		SELECT public_id::text, image_url, is_primary, sort_order
		FROM store_images
		WHERE store_id = $1
		ORDER BY sort_order ASC, id ASC`, st.ID)
	if err != nil {
		return fmt.Errorf("list store images: %w", err)
	}
	defer imgRows.Close()

	for imgRows.Next() {
		var img model.StoreImage
		if err := imgRows.Scan(&img.ID, &img.URL, &img.IsPrimary, &img.SortOrder); err != nil {
			return fmt.Errorf("scan store image: %w", err)
		}
		st.Images = append(st.Images, img)
	}
	if err := imgRows.Err(); err != nil {
		return fmt.Errorf("iterate store images: %w", err)
	}
	return nil
}

var _ = time.UTC

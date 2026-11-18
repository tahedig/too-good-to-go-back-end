package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tahedig/too-good-to-go/backend/internal/domain/enum"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/model"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/repository"
)

type adminStoreRepository struct {
	pool *pgxpool.Pool
}

func NewAdminStoreRepository(pool *pgxpool.Pool) repository.AdminStoreRepository {
	return &adminStoreRepository{pool: pool}
}

const selectAdminStoreQuery = `
SELECT s.public_id::text, s.name, coalesce(u.full_name, ''), coalesce(c.name_fa, ''),
       coalesce(s.phone, v.contact_phone, u.phone, ''), s.biography,
       coalesce(s.cover_image_url, s.logo_url), s.accepts_orders, s.status::text,
       s.created_at, ST_Y(s.location::geometry), ST_X(s.location::geometry),
       s.city, s.district, s.address_line, s.id
FROM stores s
JOIN vendors v ON v.id = s.vendor_id
JOIN users u ON u.id = v.owner_user_id
LEFT JOIN categories c ON c.id = s.category_id
WHERE s.deleted_at IS NULL AND s.public_id = $1::uuid`

const selectStoreWorkingHoursQuery = `
SELECT day_of_week, to_char(open_time, 'HH24:MI'), to_char(close_time, 'HH24:MI'), is_closed
FROM store_working_hours
WHERE store_id = $1
ORDER BY day_of_week ASC`

func (r *adminStoreRepository) GetStore(ctx context.Context, storeID string) (*model.AdminStoreDetail, error) {
	var detail model.AdminStoreDetail
	var internalID int64
	var statusStr string

	err := r.pool.QueryRow(ctx, selectAdminStoreQuery, storeID).Scan(
		&detail.ID,
		&detail.Name,
		&detail.OwnerName,
		&detail.Category,
		&detail.Phone,
		&detail.Biography,
		&detail.ImageURL,
		&detail.IsActive,
		&statusStr,
		&detail.CreatedAt,
		&detail.Location.Latitude,
		&detail.Location.Longitude,
		&detail.Location.City,
		&detail.Location.District,
		&detail.Location.AddressLine,
		&internalID,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, repository.ErrAdminStoreNotFound
		}
		return nil, err
	}
	detail.VendorStatus = enum.StoreStatus(statusStr)

	rows, err := r.pool.Query(ctx, selectStoreWorkingHoursQuery, internalID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	detail.WorkingHours = []model.AdminStoreWorkingHour{}
	for rows.Next() {
		var wh model.AdminStoreWorkingHour
		if err := rows.Scan(&wh.DayOfWeek, &wh.OpenTime, &wh.CloseTime, &wh.IsClosed); err != nil {
			return nil, err
		}
		detail.WorkingHours = append(detail.WorkingHours, wh)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return &detail, nil
}

func (r *adminStoreRepository) UpdateStoreStatus(ctx context.Context, storeID string, isActive bool) (*model.AdminStoreDetail, error) {
	tag, err := r.pool.Exec(ctx, `
UPDATE stores
SET accepts_orders = $2
WHERE public_id = $1::uuid AND deleted_at IS NULL`, storeID, isActive)
	if err != nil {
		return nil, err
	}
	if tag.RowsAffected() == 0 {
		return nil, repository.ErrAdminStoreNotFound
	}
	return r.GetStore(ctx, storeID)
}

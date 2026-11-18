package postgres

import (
	"context"
	"errors"
	"time"

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

const selectStoreSubscriptionQuery = `
SELECT ss.public_id::text, s.public_id::text, ss.status::text, p.name_fa,
       ss.commission_rate::double precision, ss.period_start, ss.period_end,
       GREATEST(0, CEIL(EXTRACT(EPOCH FROM (ss.period_end - now())) / 86400))::int,
       up.name_fa
FROM stores s
LEFT JOIN LATERAL (
    SELECT * FROM store_subscriptions
    WHERE store_id = s.id
    ORDER BY created_at DESC, id DESC
    LIMIT 1
) ss ON true
LEFT JOIN subscription_plans p ON p.id = ss.plan_id
LEFT JOIN subscription_plans up ON up.id = ss.upgrade_plan_id
WHERE s.deleted_at IS NULL AND s.public_id = $1::uuid`

func (r *adminStoreRepository) GetStoreSubscription(ctx context.Context, storeID string) (*model.AdminStoreSubscription, error) {
	var subID, sID, status, planName, upgradePlan *string
	var commission *float64
	var periodStart, periodEnd *time.Time
	var remainingDays *int

	err := r.pool.QueryRow(ctx, selectStoreSubscriptionQuery, storeID).Scan(
		&subID, &sID, &status, &planName, &commission,
		&periodStart, &periodEnd, &remainingDays, &upgradePlan,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, repository.ErrAdminStoreNotFound
		}
		return nil, err
	}
	if subID == nil {
		return nil, repository.ErrAdminSubscriptionNotFound
	}
	return &model.AdminStoreSubscription{
		ID:             *subID,
		StoreID:        *sID,
		Status:         *status,
		CurrentPlan:    *planName,
		CommissionRate: *commission,
		PeriodStart:    *periodStart,
		PeriodEnd:      *periodEnd,
		RemainingDays:  *remainingDays,
		UpgradeRequest: upgradePlan,
	}, nil
}

const selectAdminOfferBase = `
SELECT o.public_id::text, s.public_id::text, s.name, coalesce(u.full_name, ''),
       coalesce(oc.name_fa, sc.name_fa, ''), o.title, o.description, o.offer_type::text,
       o.image_url, o.original_price, o.discount_price, o.quantity_total, o.quantity_available,
       o.status::text, (o.status = 'active'), o.pickup_start, o.pickup_end, o.created_at
FROM offers o
JOIN stores s ON s.id = o.store_id
JOIN vendors v ON v.id = s.vendor_id
JOIN users u ON u.id = v.owner_user_id
LEFT JOIN categories oc ON oc.id = o.category_id
LEFT JOIN categories sc ON sc.id = s.category_id
WHERE s.deleted_at IS NULL`

func (r *adminStoreRepository) ListStoreOffers(ctx context.Context, storeID string) ([]model.AdminOffer, error) {
	var exists bool
	if err := r.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM stores WHERE public_id = $1::uuid AND deleted_at IS NULL)`, storeID).Scan(&exists); err != nil {
		return nil, err
	}
	if !exists {
		return nil, repository.ErrAdminStoreNotFound
	}

	rows, err := r.pool.Query(ctx, selectAdminOfferBase+` AND s.public_id = $1::uuid ORDER BY o.created_at DESC, o.id DESC`, storeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]model.AdminOffer, 0)
	for rows.Next() {
		var o model.AdminOffer
		if err := rows.Scan(
			&o.ID, &o.StoreID, &o.StoreName, &o.OwnerName, &o.Category,
			&o.Title, &o.Description, &o.OfferType, &o.ImageURL,
			&o.OriginalPrice, &o.DiscountPrice, &o.QuantityTotal, &o.QuantityAvailable,
			&o.Status, &o.IsActive, &o.PickupStart, &o.PickupEnd, &o.CreatedAt,
		); err != nil {
			return nil, err
		}
		items = append(items, o)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (r *adminStoreRepository) GetOffer(ctx context.Context, offerID string) (*model.AdminOffer, error) {
	var o model.AdminOffer
	err := r.pool.QueryRow(ctx, selectAdminOfferBase+` AND o.public_id = $1::uuid`, offerID).Scan(
		&o.ID, &o.StoreID, &o.StoreName, &o.OwnerName, &o.Category,
		&o.Title, &o.Description, &o.OfferType, &o.ImageURL,
		&o.OriginalPrice, &o.DiscountPrice, &o.QuantityTotal, &o.QuantityAvailable,
		&o.Status, &o.IsActive, &o.PickupStart, &o.PickupEnd, &o.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, repository.ErrAdminOfferNotFound
		}
		return nil, err
	}
	return &o, nil
}

func (r *adminStoreRepository) UpdateOfferStatus(ctx context.Context, offerID string, isActive bool) (*model.AdminOffer, error) {
	newStatus := "cancelled"
	if isActive {
		newStatus = "active"
	}
	tag, err := r.pool.Exec(ctx, `
UPDATE offers o
SET status = $2::offer_status
FROM stores s
WHERE o.store_id = s.id AND o.public_id = $1::uuid AND s.deleted_at IS NULL`, offerID, newStatus)
	if err != nil {
		return nil, err
	}
	if tag.RowsAffected() == 0 {
		return nil, repository.ErrAdminOfferNotFound
	}
	return r.GetOffer(ctx, offerID)
}

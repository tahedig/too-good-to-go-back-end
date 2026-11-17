package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tahedig/too-good-to-go/backend/internal/domain/enum"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/exception"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/model"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/repository"
	"github.com/tahedig/too-good-to-go/backend/pkg/security"
)

type orderRepository struct {
	pool *pgxpool.Pool
}

// NewOrderRepository reads orders from the orders, order_items and payments
// tables.
func NewOrderRepository(pool *pgxpool.Pool) repository.OrderRepository {
	return &orderRepository{pool: pool}
}

// listOrdersSQL returns one page of a user's orders, newest first. The user
// is resolved inside the query, so a public ID that does not exist simply
// matches nothing. Pagination is keyset on (created_at, public_id), which
// idx_orders_user covers. The pickup window and offer type come from the
// ordered offers; the payment status is the latest attempt's.
//
// $2: statuses as text[] (NULL = all); $3/$4: cursor (NULL = first page).
const listOrdersSQL = `
SELECT o.public_id::text, o.status::text, o.items_total, o.service_fee, o.total_amount,
       o.reserved_until, o.created_at,
       s.public_id::text, s.name, s.city, s.address_line,
       p.status::text,
       it.pickup_start, it.pickup_end, it.items
FROM orders o
JOIN stores s ON s.id = o.store_id
LEFT JOIN LATERAL (
    SELECT pay.status FROM payments pay
    WHERE pay.order_id = o.id
    ORDER BY pay.created_at DESC, pay.id DESC
    LIMIT 1
) p ON true
LEFT JOIN LATERAL (
    SELECT min(f.pickup_start) AS pickup_start,
           max(f.pickup_end)   AS pickup_end,
           jsonb_agg(jsonb_build_object(
               'title',      oi.title_snapshot,
               'offer_type', f.offer_type,
               'quantity',   oi.quantity,
               'unit_price', oi.unit_price) ORDER BY oi.id) AS items
    FROM order_items oi
    JOIN offers f ON f.id = oi.offer_id
    WHERE oi.order_id = o.id
) it ON true
WHERE o.user_id = (SELECT u.id FROM users u WHERE u.public_id = $1::uuid)
  AND ($2::text[] IS NULL OR o.status::text = ANY($2::text[]))
  AND ($3::timestamptz IS NULL OR (o.created_at, o.public_id) < ($3::timestamptz, $4::uuid))
ORDER BY o.created_at DESC, o.public_id DESC
LIMIT $5`

type orderItemRow struct {
	Title     string `json:"title"`
	OfferType string `json:"offer_type"`
	Quantity  int    `json:"quantity"`
	UnitPrice int64  `json:"unit_price"`
}

func (r *orderRepository) ListByUser(ctx context.Context, q repository.OrderListQuery) ([]model.Order, error) {
	if !security.IsID(q.UserID) || q.Limit <= 0 {
		return []model.Order{}, nil
	}

	var statuses []string
	if q.Statuses != nil {
		statuses = make([]string, len(q.Statuses))
		for i, s := range q.Statuses {
			statuses[i] = string(s)
		}
	}
	var afterTime *time.Time
	var afterID *string
	if q.After != nil {
		if !security.IsID(q.After.ID) {
			return []model.Order{}, nil
		}
		afterTime, afterID = &q.After.CreatedAt, &q.After.ID
	}

	rows, err := r.pool.Query(ctx, listOrdersSQL, q.UserID, statuses, afterTime, afterID, q.Limit)
	if err != nil {
		return nil, fmt.Errorf("list orders: %w", err)
	}
	orders, err := pgx.CollectRows(rows, scanOrder)
	if err != nil {
		return nil, fmt.Errorf("list orders: %w", err)
	}
	return orders, nil
}

func scanOrder(row pgx.CollectableRow) (model.Order, error) {
	var (
		o             model.Order
		status        string
		paymentStatus *string
		items         []byte
	)
	if err := row.Scan(&o.ID, &status, &o.ItemsTotal, &o.ServiceFee, &o.TotalAmount,
		&o.ReservedUntil, &o.CreatedAt,
		&o.Store.ID, &o.Store.Name, &o.Store.City, &o.Store.AddressLine,
		&paymentStatus,
		&o.PickupStart, &o.PickupEnd, &items); err != nil {
		return o, err
	}
	o.Status = enum.OrderStatus(status)
	if paymentStatus != nil {
		o.PaymentStatus = *paymentStatus
	}
	o.ReservedUntil = o.ReservedUntil.UTC()
	o.CreatedAt = o.CreatedAt.UTC()
	if o.PickupStart != nil {
		t := o.PickupStart.UTC()
		o.PickupStart = &t
	}
	if o.PickupEnd != nil {
		t := o.PickupEnd.UTC()
		o.PickupEnd = &t
	}

	o.Items = []model.OrderItem{}
	if items != nil {
		var parsed []orderItemRow
		if err := json.Unmarshal(items, &parsed); err != nil {
			return o, fmt.Errorf("decode order items: %w", err)
		}
		for _, it := range parsed {
			o.Items = append(o.Items, model.OrderItem{
				Title: it.Title, OfferType: it.OfferType, Quantity: it.Quantity, UnitPrice: it.UnitPrice,
			})
		}
	}
	return o, nil
}

func (r *orderRepository) GetActiveStoreByVendorUser(ctx context.Context, vendorUserID string) (*model.VendorStoreActive, error) {
	if !security.IsID(vendorUserID) {
		return nil, exception.ErrStoreNotFound
	}

	const query = `
SELECT s.public_id::text, s.name, s.address_line, COALESCE(s.phone, ''), s.is_active
FROM stores s
JOIN users u ON u.id = s.vendor_id
WHERE u.public_id = $1::uuid AND s.deleted_at IS NULL
ORDER BY s.is_active DESC, s.created_at DESC
LIMIT 1`

	var s model.VendorStoreActive
	err := r.pool.QueryRow(ctx, query, vendorUserID).Scan(
		&s.ID, &s.Name, &s.Address, &s.Phone, &s.IsActive,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, exception.ErrStoreNotFound
		}
		return nil, fmt.Errorf("get active store by vendor user: %w", err)
	}
	return &s, nil
}

func (r *orderRepository) ListVendorOrders(ctx context.Context, vendorUserID string, filter repository.VendorOrderFilter) ([]model.VendorOrder, int, error) {
	if !security.IsID(vendorUserID) {
		return []model.VendorOrder{}, 0, nil
	}

	var statusFilter []string
	if filter.Status != "" && filter.Status != "all" {
		dbStatuses := enum.VendorFilterToOrderStatuses(filter.Status)
		for _, s := range dbStatuses {
			statusFilter = append(statusFilter, string(s))
		}
	}

	offset := (filter.Page - 1) * filter.Limit

	const query = `
WITH filtered AS (
    SELECT o.id, o.public_id::text, o.status::text, o.items_total, o.pickup_code, o.completed_at, o.created_at,
           u.public_id::text AS cust_id, u.full_name AS cust_name, u.phone AS cust_phone,
           it.offer_public_id, it.title_snapshot, it.image_url, it.quantity, it.unit_price,
           it.pickup_start, it.pickup_end
    FROM orders o
    JOIN stores s ON s.id = o.store_id
    JOIN users v ON v.id = s.vendor_id
    JOIN users u ON u.id = o.user_id
    LEFT JOIN LATERAL (
        SELECT f.public_id::text AS offer_public_id, oi.title_snapshot, f.image_url,
               oi.quantity, oi.unit_price, f.pickup_start, f.pickup_end
        FROM order_items oi
        JOIN offers f ON f.id = oi.offer_id
        WHERE oi.order_id = o.id
        ORDER BY oi.id ASC
        LIMIT 1
    ) it ON true
    WHERE v.public_id = $1::uuid AND s.deleted_at IS NULL
      AND ($2::text = '' OR TO_CHAR(COALESCE(it.pickup_start, o.created_at) AT TIME ZONE 'UTC', 'YYYY-MM-DD') = $2)
      AND ($3::text[] IS NULL OR o.status::text = ANY($3::text[]))
)
SELECT id, public_id, status, items_total, pickup_code, completed_at, created_at,
       cust_id, cust_name, cust_phone,
       COALESCE(offer_public_id, ''), COALESCE(title_snapshot, ''), COALESCE(image_url, ''),
       COALESCE(quantity, 1), COALESCE(unit_price, items_total),
       pickup_start, pickup_end,
       COUNT(*) OVER() AS total_count
FROM filtered
ORDER BY created_at DESC, id DESC
LIMIT $4 OFFSET $5`

	rows, err := r.pool.Query(ctx, query, vendorUserID, filter.Date, statusFilter, filter.Limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list vendor orders: %w", err)
	}
	defer rows.Close()

	var orders []model.VendorOrder
	totalCount := 0

	for rows.Next() {
		var (
			numericID      int64
			publicID       string
			statusStr      string
			itemsTotal     int64
			pickupCode     string
			completedAt    *time.Time
			createdAt      time.Time
			custID         string
			custName       string
			custPhone      string
			offerID        string
			offerTitle     string
			offerImage     string
			quantity       int
			unitPrice      int64
			pickupStart    *time.Time
			pickupEnd      *time.Time
			count          int
		)

		if err := rows.Scan(
			&numericID, &publicID, &statusStr, &itemsTotal, &pickupCode, &completedAt, &createdAt,
			&custID, &custName, &custPhone,
			&offerID, &offerTitle, &offerImage,
			&quantity, &unitPrice,
			&pickupStart, &pickupEnd,
			&count,
		); err != nil {
			return nil, 0, fmt.Errorf("scan vendor order: %w", err)
		}

		totalCount = count
		dbStatus := enum.OrderStatus(statusStr)
		vendorStatus := dbStatus.ToVendorStatus()

		pickupDate := createdAt.UTC().Format("2006-01-02")
		startTime := "00:00"
		endTime := "23:59"
		if pickupStart != nil {
			pickupDate = pickupStart.UTC().Format("2006-01-02")
			startTime = pickupStart.UTC().Format("15:04")
		}
		if pickupEnd != nil {
			endTime = pickupEnd.UTC().Format("15:04")
		}

		orderNumber := fmt.Sprintf("TG-%04d", numericID%10000)

		orders = append(orders, model.VendorOrder{
			ID:          publicID,
			NumericID:   numericID,
			OrderNumber: orderNumber,
			Customer: model.VendorOrderCustomer{
				ID:    custID,
				Name:  custName,
				Phone: custPhone,
			},
			Package: model.VendorOrderPackage{
				ID:       offerID,
				Title:    offerTitle,
				ImageURL: offerImage,
			},
			Quantity:   quantity,
			UnitPrice:  unitPrice,
			TotalPrice: itemsTotal,
			Status:     vendorStatus,
			PickupDate: pickupDate,
			PickupTimeSlot: model.VendorOrderTimeSlot{
				StartTime: startTime,
				EndTime:   endTime,
			},
			PickupCode:  pickupCode,
			DeliveredAt: completedAt,
			CreatedAt:   createdAt.UTC(),
		})
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("rows error: %w", err)
	}

	return orders, totalCount, nil
}

func (r *orderRepository) GetVendorOrderByID(ctx context.Context, vendorUserID string, orderID string) (*model.VendorOrder, error) {
	if !security.IsID(vendorUserID) {
		return nil, exception.ErrOrderNotFound
	}

	const query = `
SELECT o.id, o.public_id::text, o.status::text, o.items_total, o.pickup_code, o.completed_at, o.created_at,
       u.public_id::text AS cust_id, u.full_name AS cust_name, u.phone AS cust_phone,
       COALESCE(it.offer_public_id, ''), COALESCE(it.title_snapshot, ''), COALESCE(it.image_url, ''),
       COALESCE(it.quantity, 1), COALESCE(it.unit_price, o.items_total),
       it.pickup_start, it.pickup_end
FROM orders o
JOIN stores s ON s.id = o.store_id
JOIN users v ON v.id = s.vendor_id
JOIN users u ON u.id = o.user_id
LEFT JOIN LATERAL (
    SELECT f.public_id::text AS offer_public_id, oi.title_snapshot, f.image_url,
           oi.quantity, oi.unit_price, f.pickup_start, f.pickup_end
    FROM order_items oi
    JOIN offers f ON f.id = oi.offer_id
    WHERE oi.order_id = o.id
    ORDER BY oi.id ASC
    LIMIT 1
) it ON true
WHERE v.public_id = $1::uuid AND s.deleted_at IS NULL
  AND (o.public_id::text = $2 OR o.id::text = $2)
LIMIT 1`

	var (
		numericID   int64
		publicID    string
		statusStr   string
		itemsTotal  int64
		pickupCode  string
		completedAt *time.Time
		createdAt   time.Time
		custID      string
		custName    string
		custPhone   string
		offerID     string
		offerTitle  string
		offerImage  string
		quantity    int
		unitPrice   int64
		pickupStart *time.Time
		pickupEnd   *time.Time
	)

	err := r.pool.QueryRow(ctx, query, vendorUserID, orderID).Scan(
		&numericID, &publicID, &statusStr, &itemsTotal, &pickupCode, &completedAt, &createdAt,
		&custID, &custName, &custPhone,
		&offerID, &offerTitle, &offerImage,
		&quantity, &unitPrice,
		&pickupStart, &pickupEnd,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, exception.ErrOrderNotFound
		}
		return nil, fmt.Errorf("get vendor order by id: %w", err)
	}

	dbStatus := enum.OrderStatus(statusStr)
	vendorStatus := dbStatus.ToVendorStatus()

	pickupDate := createdAt.UTC().Format("2006-01-02")
	startTime := "00:00"
	endTime := "23:59"
	if pickupStart != nil {
		pickupDate = pickupStart.UTC().Format("2006-01-02")
		startTime = pickupStart.UTC().Format("15:04")
	}
	if pickupEnd != nil {
		endTime = pickupEnd.UTC().Format("15:04")
	}

	orderNumber := fmt.Sprintf("TG-%04d", numericID%10000)

	return &model.VendorOrder{
		ID:          publicID,
		NumericID:   numericID,
		OrderNumber: orderNumber,
		Customer: model.VendorOrderCustomer{
			ID:    custID,
			Name:  custName,
			Phone: custPhone,
		},
		Package: model.VendorOrderPackage{
			ID:       offerID,
			Title:    offerTitle,
			ImageURL: offerImage,
		},
		Quantity:   quantity,
		UnitPrice:  unitPrice,
		TotalPrice: itemsTotal,
		Status:     vendorStatus,
		PickupDate: pickupDate,
		PickupTimeSlot: model.VendorOrderTimeSlot{
			StartTime: startTime,
			EndTime:   endTime,
		},
		PickupCode:  pickupCode,
		DeliveredAt: completedAt,
		CreatedAt:   createdAt.UTC(),
	}, nil
}

func (r *orderRepository) GetVendorOrderSummary(ctx context.Context, vendorUserID string, date string) (*model.VendorOrderSummary, error) {
	if !security.IsID(vendorUserID) {
		return &model.VendorOrderSummary{Date: date}, nil
	}

	const query = `
SELECT
    COALESCE(SUM(CASE WHEN o.status IN ('paid', 'ready', 'completed') THEN COALESCE(it.quantity, 1) ELSE 0 END), 0) AS total_reserved_packages,
    COALESCE(COUNT(CASE WHEN o.status IN ('paid', 'ready') THEN 1 END), 0) AS active_reservations_count,
    COALESCE(COUNT(CASE WHEN o.status = 'completed' THEN 1 END), 0) AS delivered_packages_count,
    COALESCE(SUM(CASE WHEN o.status IN ('paid', 'ready', 'completed') THEN o.items_total ELSE 0 END), 0) AS total_revenue
FROM orders o
JOIN stores s ON s.id = o.store_id
JOIN users v ON v.id = s.vendor_id
LEFT JOIN LATERAL (
    SELECT SUM(oi.quantity) AS quantity, MIN(f.pickup_start) AS pickup_start
    FROM order_items oi
    JOIN offers f ON f.id = oi.offer_id
    WHERE oi.order_id = o.id
) it ON true
WHERE v.public_id = $1::uuid AND s.deleted_at IS NULL
  AND TO_CHAR(COALESCE(it.pickup_start, o.created_at) AT TIME ZONE 'UTC', 'YYYY-MM-DD') = $2`

	var (
		totalReservedPackages   int
		activeReservationsCount int
		deliveredPackagesCount  int
		totalRevenue            int64
	)

	err := r.pool.QueryRow(ctx, query, vendorUserID, date).Scan(
		&totalReservedPackages,
		&activeReservationsCount,
		&deliveredPackagesCount,
		&totalRevenue,
	)
	if err != nil {
		return nil, fmt.Errorf("get vendor order summary: %w", err)
	}

	return &model.VendorOrderSummary{
		Date:                    date,
		TotalReservedPackages:   totalReservedPackages,
		ActiveReservationsCount: activeReservationsCount,
		DeliveredPackagesCount:  deliveredPackagesCount,
		TotalRevenue:            totalRevenue,
	}, nil
}

func (r *orderRepository) VerifyPickup(ctx context.Context, vendorUserID string, orderID string, pickupCode string) (*model.VerifyPickupResult, error) {
	if !security.IsID(vendorUserID) {
		return nil, exception.ErrOrderNotFound
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin verify pickup tx: %w", err)
	}
	defer tx.Rollback(ctx)

	const selectQuery = `
SELECT o.id, o.public_id::text, o.status::text, o.pickup_code, v.id AS vendor_id
FROM orders o
JOIN stores s ON s.id = o.store_id
JOIN users v ON v.id = s.vendor_id
WHERE v.public_id = $1::uuid AND s.deleted_at IS NULL
  AND (o.public_id::text = $2 OR o.id::text = $2)
FOR UPDATE OF o`

	var (
		orderNumericID int64
		orderPublicID  string
		statusStr      string
		dbPickupCode   string
		vendorNumID    int64
	)

	err = tx.QueryRow(ctx, selectQuery, vendorUserID, orderID).Scan(
		&orderNumericID, &orderPublicID, &statusStr, &dbPickupCode, &vendorNumID,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, exception.ErrOrderNotFound
		}
		return nil, fmt.Errorf("select order for pickup verify: %w", err)
	}

	dbStatus := enum.OrderStatus(statusStr)
	if dbStatus == enum.OrderStatusCompleted {
		return nil, exception.ErrOrderAlreadyDelivered
	}
	if !enum.CanVendorTransition(dbStatus, enum.OrderStatusCompleted) {
		return nil, exception.ErrOrderCannotBeDelivered
	}

	if pickupCode != "" && dbPickupCode != "" && pickupCode != dbPickupCode {
		return nil, exception.ErrInvalidPickupCode
	}

	now := time.Now().UTC()

	if _, err = tx.Exec(ctx, `SELECT set_config('app.user_id', $1::text, true)`, vendorNumID); err != nil {
		return nil, fmt.Errorf("set actor user_id: %w", err)
	}

	const updateQuery = `
UPDATE orders
SET status = 'completed', confirmed_by = $1, completed_at = $2, updated_at = $2
WHERE id = $3`

	_, err = tx.Exec(ctx, updateQuery, vendorNumID, now, orderNumericID)
	if err != nil {
		return nil, fmt.Errorf("update order completed: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit verify pickup tx: %w", err)
	}

	return &model.VerifyPickupResult{
		Success:     true,
		OrderID:     orderPublicID,
		Status:      enum.VendorOrderStatusDelivered,
		DeliveredAt: now,
		Message:     "تحویل بسته با موفقیت ثبت شد",
	}, nil
}

func (r *orderRepository) UpdateVendorOrderStatus(ctx context.Context, vendorUserID string, orderID string, targetStatus enum.OrderStatus, reason string) (*model.VendorOrderStatusUpdateResult, error) {
	if !security.IsID(vendorUserID) {
		return nil, exception.ErrOrderNotFound
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin update vendor order status tx: %w", err)
	}
	defer tx.Rollback(ctx)

	const selectQuery = `
SELECT o.id, o.public_id::text, o.status::text, v.id AS vendor_id
FROM orders o
JOIN stores s ON s.id = o.store_id
JOIN users v ON v.id = s.vendor_id
WHERE v.public_id = $1::uuid AND s.deleted_at IS NULL
  AND (o.public_id::text = $2 OR o.id::text = $2)
FOR UPDATE OF o`

	var (
		orderNumericID int64
		orderPublicID  string
		statusStr      string
		vendorNumID    int64
	)

	err = tx.QueryRow(ctx, selectQuery, vendorUserID, orderID).Scan(
		&orderNumericID, &orderPublicID, &statusStr, &vendorNumID,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, exception.ErrOrderNotFound
		}
		return nil, fmt.Errorf("select order for status transition: %w", err)
	}

	dbStatus := enum.OrderStatus(statusStr)
	if !enum.CanVendorTransition(dbStatus, targetStatus) {
		return nil, exception.ErrInvalidOrderTransition
	}

	if _, err = tx.Exec(ctx, `SELECT set_config('app.user_id', $1::text, true)`, vendorNumID); err != nil {
		return nil, fmt.Errorf("set actor user_id: %w", err)
	}

	now := time.Now().UTC()

	if targetStatus == enum.OrderStatusCancelled {
		const cancelQuery = `
UPDATE orders
SET status = 'cancelled', cancelled_by = 'vendor', cancel_reason = NULLIF($1, ''), cancelled_at = $2, updated_at = $2
WHERE id = $3`
		if _, err = tx.Exec(ctx, cancelQuery, reason, now, orderNumericID); err != nil {
			return nil, fmt.Errorf("cancel vendor order: %w", err)
		}

		// Restore offer inventory atomically in the same transaction.
		const restoreInventoryQuery = `
UPDATE offers f
SET quantity_available = f.quantity_available + s.q,
    status = CASE WHEN f.status = 'sold_out' AND f.pickup_end > now()
                  THEN 'active'::offer_status ELSE f.status END
FROM (
    SELECT oi.offer_id, SUM(oi.quantity)::int AS q
    FROM order_items oi
    WHERE oi.order_id = $1
    GROUP BY oi.offer_id
) s
WHERE f.id = s.offer_id`
		if _, err = tx.Exec(ctx, restoreInventoryQuery, orderNumericID); err != nil {
			return nil, fmt.Errorf("restore offer inventory: %w", err)
		}
	} else {
		const updateQuery = `
UPDATE orders
SET status = $1::order_status, updated_at = $2
WHERE id = $3`
		if _, err = tx.Exec(ctx, updateQuery, string(targetStatus), now, orderNumericID); err != nil {
			return nil, fmt.Errorf("update vendor order status: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit update vendor order status tx: %w", err)
	}

	return &model.VendorOrderStatusUpdateResult{
		Success:   true,
		OrderID:   orderPublicID,
		Status:    targetStatus.ToVendorStatus(),
		UpdatedAt: now,
		Message:   "وضعیت سفارش با موفقیت به‌روزرسانی شد",
	}, nil
}


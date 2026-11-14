package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tahedig/too-good-to-go/backend/internal/domain/enum"
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

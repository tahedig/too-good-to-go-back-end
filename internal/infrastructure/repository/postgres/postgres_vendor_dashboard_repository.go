package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tahedig/too-good-to-go/backend/internal/domain/enum"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/model"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/repository"
	"github.com/tahedig/too-good-to-go/backend/pkg/security"
)

const (
	vendorDefaultPlanName       = "پایه · رایگان"
	vendorDefaultCommissionRate = 15.0
)

type vendorDashboardRepository struct {
	pool *pgxpool.Pool
}

// NewVendorDashboardRepository aggregates vendor dashboard metrics from
// PostgreSQL using a single read-only repeatable-read snapshot.
func NewVendorDashboardRepository(pool *pgxpool.Pool) repository.VendorDashboardRepository {
	return &vendorDashboardRepository{pool: pool}
}

const vendorOwnedStoresSQL = `
SELECT s.id, s.public_id::text, s.name, s.city, s.address_line, s.status::text, s.accepts_orders, s.created_at
FROM stores s
JOIN vendors v ON v.id = s.vendor_id
JOIN users u ON u.id = v.owner_user_id
WHERE u.public_id = $1::uuid AND s.deleted_at IS NULL
ORDER BY CASE WHEN s.status = 'approved' THEN 0 WHEN s.status = 'pending' THEN 1 ELSE 2 END,
         s.created_at ASC, s.id ASC`

const vendorCommissionRateSQL = `
SELECT COALESCE(
    (SELECT v.commission_rate FROM vendors v JOIN users u ON u.id = v.owner_user_id
      WHERE u.public_id = $1::uuid AND v.commission_rate IS NOT NULL
      ORDER BY v.id ASC LIMIT 1),
    (SELECT (value #>> '{}')::numeric FROM platform_settings WHERE key = 'commission_default_percent'),
    15
)::float8`

const vendorOfferKPIsSQL = `
SELECT
    COUNT(*) FILTER (WHERE f.status = 'active' AND f.quantity_available > 0 AND f.pickup_end > $2)::int,
    COALESCE(SUM(f.quantity_available) FILTER (WHERE f.status = 'active' AND f.pickup_end > $2), 0)::int
FROM offers f
JOIN stores s ON s.id = f.store_id
WHERE f.store_id = ANY($1::bigint[])
  AND s.status = 'approved'
  AND s.deleted_at IS NULL`

const vendorReservationSummarySQL = `
SELECT
    COUNT(*) FILTER (WHERE o.status = 'pending_payment')::int,
    COUNT(*) FILTER (WHERE o.status = 'paid')::int,
    COUNT(*) FILTER (WHERE o.status = 'ready')::int,
    COUNT(*) FILTER (WHERE o.status = 'completed')::int,
    COUNT(*) FILTER (WHERE o.status = 'cancelled')::int,
    COUNT(*) FILTER (WHERE o.status = 'expired')::int,
    COUNT(*) FILTER (WHERE o.status = 'no_show')::int,
    COALESCE(SUM(o.vendor_payout) FILTER (WHERE o.status = 'completed'), 0)::bigint,
    COALESCE(SUM(o.vendor_payout) FILTER (WHERE o.status IN ('paid', 'ready')), 0)::bigint,
    COALESCE(SUM(it.qty) FILTER (WHERE o.status IN ('pending_payment', 'paid', 'ready')), 0)::int
FROM orders o
LEFT JOIN LATERAL (
    SELECT COALESCE(SUM(oi.quantity), 0)::int AS qty
    FROM order_items oi
    WHERE oi.order_id = o.id
) it ON true
WHERE o.store_id = ANY($1::bigint[])`

const vendorTodayActivitySQL = `
WITH bounds AS (
    SELECT (date_trunc('day', $2::timestamptz AT TIME ZONE 'Asia/Tehran') AT TIME ZONE 'Asia/Tehran') AS day_start,
           ((date_trunc('day', $2::timestamptz AT TIME ZONE 'Asia/Tehran') + interval '1 day') AT TIME ZONE 'Asia/Tehran') AS day_end
)
SELECT
    COUNT(*) FILTER (WHERE o.created_at >= b.day_start AND o.created_at < b.day_end)::int,
    COALESCE(SUM(it.qty) FILTER (
        WHERE o.created_at >= b.day_start AND o.created_at < b.day_end
          AND o.status NOT IN ('cancelled', 'expired')
    ), 0)::int,
    COUNT(*) FILTER (
        WHERE o.status = 'completed'
          AND COALESCE(o.completed_at, o.updated_at, o.created_at) >= b.day_start
          AND COALESCE(o.completed_at, o.updated_at, o.created_at) < b.day_end
    )::int,
    COALESCE(SUM(o.vendor_payout) FILTER (
        WHERE o.created_at >= b.day_start AND o.created_at < b.day_end
          AND o.status IN ('paid', 'ready', 'completed')
    ), 0)::bigint
FROM orders o
CROSS JOIN bounds b
LEFT JOIN LATERAL (
    SELECT COALESCE(SUM(oi.quantity), 0)::int AS qty
    FROM order_items oi
    WHERE oi.order_id = o.id
) it ON true
WHERE o.store_id = ANY($1::bigint[])`

const vendorNearestReservationSQL = `
SELECT o.public_id::text, o.status::text, o.pickup_code,
       COALESCE(NULLIF(u.full_name, ''), u.phone, '') AS customer_name,
       s.public_id::text, s.name,
       COALESCE(first_item.title_snapshot, ''),
       first_item.image_url,
       COALESCE(agg.total_qty, 0)::int,
       o.total_amount,
       agg.pickup_start, agg.pickup_end,
       COALESCE(first_item.quantity_available, 0)::int,
       GREATEST(COALESCE(first_item.quantity_total, 0) - COALESCE(first_item.quantity_available, 0), 0)::int,
       COALESCE(first_item.quantity_total, 0)::int,
       o.created_at
FROM orders o
JOIN users u ON u.id = o.user_id
JOIN stores s ON s.id = o.store_id
LEFT JOIN LATERAL (
    SELECT MIN(f.pickup_start) AS pickup_start,
           MAX(f.pickup_end)   AS pickup_end,
           COALESCE(SUM(oi.quantity), 0)::int AS total_qty
    FROM order_items oi
    JOIN offers f ON f.id = oi.offer_id
    WHERE oi.order_id = o.id
) agg ON true
LEFT JOIN LATERAL (
    SELECT oi.title_snapshot, f.image_url, f.quantity_available, f.quantity_total
    FROM order_items oi
    JOIN offers f ON f.id = oi.offer_id
    WHERE oi.order_id = o.id
    ORDER BY oi.id ASC
    LIMIT 1
) first_item ON true
WHERE o.store_id = ANY($1::bigint[])
  AND o.status IN ('pending_payment', 'paid', 'ready')
ORDER BY CASE WHEN agg.pickup_end IS NOT NULL AND agg.pickup_end >= $2 THEN 0 ELSE 1 END,
         agg.pickup_start ASC NULLS LAST,
         o.created_at ASC,
         o.id ASC
LIMIT 1`

const vendorRecentReservationsSQL = `
SELECT o.public_id::text, o.status::text, o.pickup_code,
       COALESCE(NULLIF(u.full_name, ''), u.phone, '') AS customer_name,
       s.public_id::text, s.name,
       o.items_total, o.total_amount, o.vendor_payout, o.created_at,
       p.status::text,
       it.pickup_start, it.pickup_end, it.items
FROM orders o
JOIN users u ON u.id = o.user_id
JOIN stores s ON s.id = o.store_id
LEFT JOIN LATERAL (
    SELECT pay.status FROM payments pay
    WHERE pay.order_id = o.id
    ORDER BY pay.created_at DESC, pay.id DESC
    LIMIT 1
) p ON true
LEFT JOIN LATERAL (
    SELECT MIN(f.pickup_start) AS pickup_start,
           MAX(f.pickup_end)   AS pickup_end,
           jsonb_agg(jsonb_build_object(
               'title',      oi.title_snapshot,
               'offer_type', f.offer_type,
               'quantity',   oi.quantity,
               'unit_price', oi.unit_price) ORDER BY oi.id) AS items
    FROM order_items oi
    JOIN offers f ON f.id = oi.offer_id
    WHERE oi.order_id = o.id
) it ON true
WHERE o.store_id = ANY($1::bigint[])
ORDER BY o.created_at DESC, o.public_id DESC
LIMIT $2`

type ownedStoreRow struct {
	dbID  int64
	store model.VendorDashboardStore
}

func (r *vendorDashboardRepository) GetDashboard(ctx context.Context, q repository.VendorDashboardQuery) (*model.VendorDashboard, error) {
	now := q.Now.UTC()
	if now.IsZero() {
		now = time.Now().UTC()
	}
	periodStart, periodEnd, remainingDays := vendorPlanWindow(now)

	dash := &model.VendorDashboard{
		Stores:               []model.VendorDashboardStore{},
		PendingStoreRequests: []model.VendorPendingStoreBanner{},
		Financial: model.VendorDashboardFinancial{
			Plan: model.VendorDashboardPlan{
				Name:           vendorDefaultPlanName,
				CommissionRate: vendorDefaultCommissionRate,
				PeriodStart:    periodStart,
				PeriodEnd:      periodEnd,
				RemainingDays:  remainingDays,
			},
		},
		RecentReservations: []model.VendorDashboardReservation{},
	}

	if !security.IsID(q.VendorUserID) {
		if q.StoreID != "" {
			return nil, repository.ErrStoreNotFound
		}
		return dash, nil
	}

	err := pgx.BeginTxFunc(ctx, r.pool, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly},
		func(tx pgx.Tx) error {
			rows, err := tx.Query(ctx, vendorOwnedStoresSQL, q.VendorUserID)
			if err != nil {
				return fmt.Errorf("load vendor stores: %w", err)
			}
			owned, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (ownedStoreRow, error) {
				var (
					rec    ownedStoreRow
					status string
				)
				if err := row.Scan(&rec.dbID, &rec.store.ID, &rec.store.Name, &rec.store.City, &rec.store.AddressLine,
					&status, &rec.store.AcceptsOrders, &rec.store.CreatedAt); err != nil {
					return rec, err
				}
				rec.store.Status = enum.StoreStatus(status)
				rec.store.CreatedAt = rec.store.CreatedAt.UTC()
				return rec, nil
			})
			if err != nil {
				return fmt.Errorf("scan vendor stores: %w", err)
			}

			var rate float64
			if err := tx.QueryRow(ctx, vendorCommissionRateSQL, q.VendorUserID).Scan(&rate); err != nil {
				return fmt.Errorf("load commission rate: %w", err)
			}
			dash.Financial.Plan.CommissionRate = rate

			scopedIDs := make([]int64, 0, len(owned))
			filterStoreID := strings.ToLower(q.StoreID)
			for _, rec := range owned {
				dash.Stores = append(dash.Stores, rec.store)
				if rec.store.Status == enum.StoreStatusPending {
					dash.PendingStoreRequests = append(dash.PendingStoreRequests, model.VendorPendingStoreBanner{
						ID:          rec.store.ID,
						StoreName:   rec.store.Name,
						City:        rec.store.City,
						AddressLine: rec.store.AddressLine,
						Status:      rec.store.Status,
						CreatedAt:   rec.store.CreatedAt,
					})
				}
				if filterStoreID == "" {
					scopedIDs = append(scopedIDs, rec.dbID)
				} else if strings.EqualFold(rec.store.ID, filterStoreID) {
					scopedIDs = append(scopedIDs, rec.dbID)
					cp := rec.store
					dash.ActiveStore = &cp
				}
			}

			if filterStoreID != "" && len(scopedIDs) == 0 {
				return repository.ErrStoreNotFound
			}
			if filterStoreID == "" && len(owned) > 0 {
				cp := owned[0].store
				dash.ActiveStore = &cp
			}
			if len(scopedIDs) == 0 {
				return nil
			}

			if err := tx.QueryRow(ctx, vendorOfferKPIsSQL, scopedIDs, now).Scan(
				&dash.KPIs.ActiveOffers,
				&dash.KPIs.AvailablePackages,
			); err != nil {
				return fmt.Errorf("load offer kpis: %w", err)
			}

			if err := tx.QueryRow(ctx, vendorReservationSummarySQL, scopedIDs).Scan(
				&dash.ReservationCounts.PendingPayment,
				&dash.ReservationCounts.Paid,
				&dash.ReservationCounts.Ready,
				&dash.ReservationCounts.Completed,
				&dash.ReservationCounts.Cancelled,
				&dash.ReservationCounts.Expired,
				&dash.ReservationCounts.NoShow,
				&dash.Financial.ReceivedPayout,
				&dash.Financial.PendingPayout,
				&dash.KPIs.ReservedPackages,
			); err != nil {
				return fmt.Errorf("load reservation summary: %w", err)
			}
			dash.ReservationCounts.Active = dash.ReservationCounts.PendingPayment + dash.ReservationCounts.Paid + dash.ReservationCounts.Ready
			dash.ReservationCounts.Total = dash.ReservationCounts.Active + dash.ReservationCounts.Completed +
				dash.ReservationCounts.Cancelled + dash.ReservationCounts.Expired + dash.ReservationCounts.NoShow
			dash.KPIs.ReceivedRevenue = dash.Financial.ReceivedPayout
			dash.Financial.NetPeriodRevenue = dash.Financial.ReceivedPayout + dash.Financial.PendingPayout
			dash.Financial.AvailableBalance = dash.Financial.ReceivedPayout

			if err := tx.QueryRow(ctx, vendorTodayActivitySQL, scopedIDs, now).Scan(
				&dash.Today.ReservationsCount,
				&dash.Today.PackagesReserved,
				&dash.Today.CompletedCount,
				&dash.Today.Revenue,
			); err != nil {
				return fmt.Errorf("load today activity: %w", err)
			}

			nearest, err := loadNearestReservation(ctx, tx, scopedIDs, now)
			if err != nil {
				return err
			}
			dash.NearestReservation = nearest

			if q.RecentLimit > 0 {
				recent, err := loadRecentVendorReservations(ctx, tx, scopedIDs, q.RecentLimit)
				if err != nil {
					return err
				}
				dash.RecentReservations = recent
			}
			return nil
		})
	if err != nil {
		return nil, err
	}
	return dash, nil
}

func loadNearestReservation(ctx context.Context, tx pgx.Tx, storeIDs []int64, now time.Time) (*model.VendorNearestReservation, error) {
	var (
		n      model.VendorNearestReservation
		status string
	)
	err := tx.QueryRow(ctx, vendorNearestReservationSQL, storeIDs, now).Scan(
		&n.ID, &status, &n.PickupCode, &n.CustomerName,
		&n.StoreID, &n.StoreName, &n.OfferTitle, &n.OfferImageURL,
		&n.Quantity, &n.TotalAmount, &n.PickupStart, &n.PickupEnd,
		&n.OfferQuantityAvailable, &n.OfferQuantityReserved, &n.OfferQuantityTotal,
		&n.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("load nearest reservation: %w", err)
	}
	n.Status = enum.OrderStatus(status)
	n.CreatedAt = n.CreatedAt.UTC()
	if n.PickupStart != nil {
		t := n.PickupStart.UTC()
		n.PickupStart = &t
	}
	if n.PickupEnd != nil {
		t := n.PickupEnd.UTC()
		n.PickupEnd = &t
	}
	return &n, nil
}

func loadRecentVendorReservations(ctx context.Context, tx pgx.Tx, storeIDs []int64, limit int) ([]model.VendorDashboardReservation, error) {
	rows, err := tx.Query(ctx, vendorRecentReservationsSQL, storeIDs, limit)
	if err != nil {
		return nil, fmt.Errorf("load recent reservations: %w", err)
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (model.VendorDashboardReservation, error) {
		var (
			r             model.VendorDashboardReservation
			status        string
			paymentStatus *string
			itemsRaw      []byte
		)
		if err := row.Scan(
			&r.ID, &status, &r.PickupCode, &r.CustomerName,
			&r.StoreID, &r.StoreName,
			&r.ItemsTotal, &r.TotalAmount, &r.VendorPayout, &r.CreatedAt,
			&paymentStatus,
			&r.PickupStart, &r.PickupEnd, &itemsRaw,
		); err != nil {
			return r, err
		}
		r.Status = enum.OrderStatus(status)
		if paymentStatus != nil {
			r.PaymentStatus = *paymentStatus
		}
		r.CreatedAt = r.CreatedAt.UTC()
		if r.PickupStart != nil {
			t := r.PickupStart.UTC()
			r.PickupStart = &t
		}
		if r.PickupEnd != nil {
			t := r.PickupEnd.UTC()
			r.PickupEnd = &t
		}
		r.Items = []model.OrderItem{}
		if itemsRaw != nil {
			var parsed []orderItemRow
			if err := json.Unmarshal(itemsRaw, &parsed); err != nil {
				return r, fmt.Errorf("decode recent reservation items: %w", err)
			}
			for _, it := range parsed {
				r.Items = append(r.Items, model.OrderItem{
					Title:     it.Title,
					OfferType: it.OfferType,
					Quantity:  it.Quantity,
					UnitPrice: it.UnitPrice,
				})
			}
		}
		return r, nil
	})
}

func vendorPlanWindow(now time.Time) (time.Time, time.Time, int) {
	u := now.UTC()
	start := time.Date(u.Year(), u.Month(), 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, 0)
	rem := int(math.Ceil(end.Sub(u).Hours() / 24.0))
	if rem < 0 {
		rem = 0
	}
	return start, end, rem
}

package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tahedig/too-good-to-go/backend/internal/domain/model"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/repository"
)

type adminDashboardRepository struct {
	pool *pgxpool.Pool
}

// NewAdminDashboardRepository aggregates platform-wide admin dashboard metrics
// and recent activity from PostgreSQL inside a single read-only snapshot.
func NewAdminDashboardRepository(pool *pgxpool.Pool) repository.AdminDashboardRepository {
	return &adminDashboardRepository{pool: pool}
}

const adminCustomersVendorsSQL = `
SELECT
    (SELECT COUNT(DISTINCT u.id)::int
       FROM users u
       JOIN user_roles ur ON ur.user_id = u.id
       JOIN roles r ON r.id = ur.role_id
      WHERE r.code = 'customer' AND u.status <> 'deleted'),
    (SELECT COUNT(*)::int FROM vendors)`

const adminStoresSummarySQL = `
SELECT
    COUNT(*)::int,
    COUNT(*) FILTER (WHERE status = 'approved' AND accepts_orders)::int,
    COUNT(*) FILTER (WHERE status = 'pending')::int,
    COUNT(*) FILTER (WHERE status = 'approved')::int,
    COUNT(*) FILTER (WHERE status = 'rejected')::int,
    COUNT(*) FILTER (WHERE status = 'suspended')::int
FROM stores
WHERE deleted_at IS NULL`

const adminActiveOffersSQL = `
SELECT COUNT(*)::int
FROM offers f
JOIN stores s ON s.id = f.store_id
WHERE f.status = 'active'
  AND f.quantity_available > 0
  AND f.pickup_end > $1
  AND s.status = 'approved'
  AND s.deleted_at IS NULL`

const adminReservationsSummarySQL = `
SELECT
    COUNT(*) FILTER (WHERE status = 'pending_payment')::int,
    COUNT(*) FILTER (WHERE status = 'paid')::int,
    COUNT(*) FILTER (WHERE status = 'ready')::int,
    COUNT(*) FILTER (WHERE status = 'completed')::int,
    COUNT(*) FILTER (WHERE status = 'cancelled')::int,
    COUNT(*) FILTER (WHERE status = 'expired')::int,
    COUNT(*) FILTER (WHERE status = 'no_show')::int,
    COALESCE(SUM(commission_amount) FILTER (WHERE status IN ('paid', 'ready', 'completed')), 0)::bigint
FROM orders`

const adminRecentActivitySQL = `
WITH feed AS (
    SELECT 'store_application'::text AS activity_type,
           s.public_id::text AS entity_id,
           s.name AS title,
           COALESCE(NULLIF(u.full_name, ''), u.phone, '') AS actor_name,
           s.status::text AS status,
           ''::text AS action,
           NULL::bigint AS amount,
           s.created_at AS occurred_at
    FROM stores s
    JOIN vendors v ON v.id = s.vendor_id
    JOIN users u ON u.id = v.owner_user_id
    WHERE s.deleted_at IS NULL
    UNION ALL
    SELECT 'store_decision'::text AS activity_type,
           s.public_id::text AS entity_id,
           s.name AS title,
           COALESCE(NULLIF(a.full_name, ''), a.phone, '') AS actor_name,
           h.to_status::text AS status,
           h.action AS action,
           NULL::bigint AS amount,
           h.created_at AS occurred_at
    FROM store_status_history h
    JOIN stores s ON s.id = h.store_id
    LEFT JOIN users a ON a.id = h.admin_id
    WHERE s.deleted_at IS NULL
    UNION ALL
    SELECT 'reservation'::text AS activity_type,
           o.public_id::text AS entity_id,
           s.name AS title,
           COALESCE(NULLIF(u.full_name, ''), u.phone, '') AS actor_name,
           o.status::text AS status,
           ''::text AS action,
           o.total_amount AS amount,
           o.created_at AS occurred_at
    FROM orders o
    JOIN stores s ON s.id = o.store_id
    JOIN users u ON u.id = o.user_id
)
SELECT activity_type, entity_id, title, actor_name, status, action, amount, occurred_at
FROM feed
WHERE $1::text[] IS NULL OR activity_type = ANY($1::text[])
ORDER BY occurred_at DESC, entity_id DESC
LIMIT $2`

func (r *adminDashboardRepository) GetDashboard(ctx context.Context, q repository.AdminDashboardQuery) (*model.AdminDashboard, error) {
	now := q.Now.UTC()
	if now.IsZero() {
		now = time.Now().UTC()
	}

	var typesFilter []string
	if len(q.ActivityTypes) > 0 {
		typesFilter = make([]string, len(q.ActivityTypes))
		for i, t := range q.ActivityTypes {
			typesFilter[i] = string(t)
		}
	}

	dash := &model.AdminDashboard{
		RecentActivity: []model.AdminActivityItem{},
	}

	err := pgx.BeginTxFunc(ctx, r.pool, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly},
		func(tx pgx.Tx) error {
			if err := tx.QueryRow(ctx, adminCustomersVendorsSQL).Scan(
				&dash.Totals.Customers,
				&dash.Totals.Vendors,
			); err != nil {
				return fmt.Errorf("load customers and vendors totals: %w", err)
			}

			if err := tx.QueryRow(ctx, adminStoresSummarySQL).Scan(
				&dash.Totals.RegisteredStores,
				&dash.Totals.ActiveStores,
				&dash.Applications.Pending,
				&dash.Applications.Approved,
				&dash.Applications.Rejected,
				&dash.Applications.Suspended,
			); err != nil {
				return fmt.Errorf("load stores summary: %w", err)
			}
			dash.Totals.PendingApplications = dash.Applications.Pending

			if err := tx.QueryRow(ctx, adminActiveOffersSQL, now).Scan(&dash.Totals.ActiveOffers); err != nil {
				return fmt.Errorf("load active offers count: %w", err)
			}

			if err := tx.QueryRow(ctx, adminReservationsSummarySQL).Scan(
				&dash.Reservations.PendingPayment,
				&dash.Reservations.Paid,
				&dash.Reservations.Ready,
				&dash.Reservations.Completed,
				&dash.Reservations.Cancelled,
				&dash.Reservations.Expired,
				&dash.Reservations.NoShow,
				&dash.Totals.PlatformCommission,
			); err != nil {
				return fmt.Errorf("load reservations summary: %w", err)
			}
			dash.Reservations.Active = dash.Reservations.PendingPayment + dash.Reservations.Paid + dash.Reservations.Ready
			dash.Reservations.Total = dash.Reservations.Active + dash.Reservations.Completed +
				dash.Reservations.Cancelled + dash.Reservations.Expired + dash.Reservations.NoShow
			dash.Totals.TotalReservations = dash.Reservations.Total
			dash.Totals.ActiveReservations = dash.Reservations.Active
			dash.Totals.CompletedReservations = dash.Reservations.Completed

			if q.ActivityLimit > 0 {
				rows, err := tx.Query(ctx, adminRecentActivitySQL, typesFilter, q.ActivityLimit)
				if err != nil {
					return fmt.Errorf("load recent activity: %w", err)
				}
				items, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (model.AdminActivityItem, error) {
					var (
						item         model.AdminActivityItem
						activityType string
					)
					if err := row.Scan(
						&activityType, &item.EntityID, &item.Title, &item.ActorName,
						&item.Status, &item.Action, &item.Amount, &item.OccurredAt,
					); err != nil {
						return item, err
					}
					item.Type = model.AdminActivityType(activityType)
					item.OccurredAt = item.OccurredAt.UTC()
					return item, nil
				})
				if err != nil {
					return fmt.Errorf("scan recent activity: %w", err)
				}
				dash.RecentActivity = items
			}
			return nil
		})
	if err != nil {
		return nil, err
	}
	return dash, nil
}

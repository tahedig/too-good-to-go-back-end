package postgres

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"math/big"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tahedig/too-good-to-go/backend/internal/domain/enum"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/model"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/repository"
	"github.com/tahedig/too-good-to-go/backend/pkg/security"
)

const (
	pgUniqueViolationCode  = "23505"
	idemConstraintName     = "orders_idem_uq"
	liveCodeConstraintName = "uq_orders_live_code"
	maxPickupCodeAttempts  = 5
	defaultCurrencyToman   = "تومان"
)

type reservationRepository struct {
	pool *pgxpool.Pool
}

// NewReservationRepository persists customer reservations in the orders and
// order_items tables with atomic inventory updates on offers.
func NewReservationRepository(pool *pgxpool.Pool) repository.ReservationRepository {
	return &reservationRepository{pool: pool}
}

const findUserIDByPublicIDSQL = `
SELECT id FROM users WHERE public_id = $1::uuid AND deleted_at IS NULL`

const findIdempotentOrderSQL = `
SELECT o.public_id::text, COALESCE(o.idempotency_payload_hash, '')
FROM orders o
WHERE o.user_id = $1 AND o.idempotency_key = $2`

const loadOfferForCheckoutSQL = `
SELECT f.id, f.public_id::text, f.store_id, f.offer_type::text, f.title, f.image_url,
       f.original_price, f.discount_price, f.quantity_available, f.max_per_user,
       f.pickup_start, f.pickup_end, f.cancellation_deadline, f.status::text,
       s.public_id::text, s.name, s.city, s.address_line,
       COALESCE(s.pickup_instructions, ''),
       ST_Y(s.location::geometry), ST_X(s.location::geometry),
       s.status::text, s.accepts_orders, s.deleted_at IS NOT NULL,
       COALESCE(c.name_fa, ''),
       COALESCE(v.commission_rate, (SELECT (value #>> '{}')::numeric FROM platform_settings WHERE key = 'commission_default_percent'), 15),
       COALESCE((SELECT (value #>> '{}')::bigint FROM platform_settings WHERE key = 'customer_service_fee'), 0)
FROM offers f
JOIN stores s ON s.id = f.store_id
JOIN vendors v ON v.id = s.vendor_id
LEFT JOIN categories c ON c.id = COALESCE(f.category_id, s.category_id)
WHERE f.public_id = $1::uuid`

const atomicDecrementOfferStockSQL = `
UPDATE offers
   SET quantity_available = quantity_available - $1,
       status = CASE WHEN quantity_available - $1 = 0 THEN 'sold_out'::offer_status ELSE status END
 WHERE id = $2
   AND status = 'active'
   AND quantity_available >= $1
   AND now() < pickup_end`

const insertReservationOrderSQL = `
INSERT INTO orders (
    user_id, store_id, address_id, status,
    idempotency_key, idempotency_payload_hash,
    items_total, service_fee, total_amount,
    commission_rate, commission_amount, vendor_payout,
    pickup_code, qr_token, reserved_until, paid_at
)
VALUES (
    $1, $2, NULL, 'paid',
    $3, $4,
    $5, $6, $7,
    $8, $9, $10,
    $11, $12, now(), now()
)
RETURNING id, public_id::text, created_at`

const insertReservationItemSQL = `
INSERT INTO order_items (
    order_id, store_id, offer_id, title_snapshot,
    quantity, unit_original_price, unit_price
)
VALUES ($1, $2, $3, $4, $5, $6, $7)`

const getReservationByPublicIDAndUserSQL = `
SELECT o.public_id::text, COALESCE(o.pickup_code, ''), o.status::text,
       o.items_total, o.service_fee, o.total_amount, o.created_at,
       s.public_id::text, s.name, s.city, s.address_line,
       COALESCE(s.pickup_instructions, ''),
       ST_Y(s.location::geometry), ST_X(s.location::geometry),
       f.public_id::text, oi.title_snapshot, f.offer_type::text, f.image_url,
       COALESCE(c.name_fa, ''),
       oi.quantity, oi.unit_price, oi.unit_original_price,
       f.pickup_start, f.pickup_end, f.cancellation_deadline
FROM orders o
JOIN stores s ON s.id = o.store_id
JOIN order_items oi ON oi.order_id = o.id
JOIN offers f ON f.id = oi.offer_id
LEFT JOIN categories c ON c.id = COALESCE(f.category_id, s.category_id)
WHERE o.public_id = $1::uuid
  AND o.user_id = (SELECT u.id FROM users u WHERE u.public_id = $2::uuid AND u.deleted_at IS NULL)
ORDER BY oi.id
LIMIT 1`

var errConcurrentIdempotencyRace = errors.New("concurrent idempotency race")

func (r *reservationRepository) Create(
	ctx context.Context,
	params repository.CreateReservationParams,
) (*repository.CreateReservationResult, error) {
	if !security.IsID(params.UserPublicID) {
		return nil, repository.ErrUserNotFound
	}
	if !security.IsID(params.OfferPublicID) {
		return nil, repository.ErrOfferNotFound
	}

	var result *repository.CreateReservationResult
	err := pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		var userID int64
		if err := tx.QueryRow(ctx, findUserIDByPublicIDSQL, params.UserPublicID).Scan(&userID); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return repository.ErrUserNotFound
			}
			return fmt.Errorf("resolve user: %w", err)
		}

		// Check idempotency key inside the transaction before touching inventory.
		if params.IdempotencyKey != "" {
			var existingPublicID, existingHash string
			err := tx.QueryRow(ctx, findIdempotentOrderSQL, userID, params.IdempotencyKey).
				Scan(&existingPublicID, &existingHash)
			if err == nil {
				if existingHash != "" && existingHash != params.IdempotencyPayloadHash {
					return repository.ErrIdempotencyConflict
				}
				existing, loadErr := r.getByPublicIDAndUser(ctx, tx, existingPublicID, params.UserPublicID)
				if loadErr != nil {
					return loadErr
				}
				result = &repository.CreateReservationResult{Reservation: existing, Replayed: true}
				return nil
			}
			if !errors.Is(err, pgx.ErrNoRows) {
				return fmt.Errorf("check idempotency key: %w", err)
			}
		}

		var (
			offerID, storeID                int64
			offerPublicID, offerType, title string
			imageURL                        *string
			originalPrice, discountPrice    int64
			qtyAvailable, maxPerUser        int
			pickupStart, pickupEnd          time.Time
			cancellationDeadline            *time.Time
			offerStatus                     string
			storePublicID, storeName        string
			storeCity, storeAddress         string
			storePickupInstructions         string
			storeLat, storeLng              float64
			storeStatus                     string
			acceptsOrders, storeDeleted     bool
			categoryName                    string
			commissionRate                  float64
			serviceFee                      int64
		)
		if err := tx.QueryRow(ctx, loadOfferForCheckoutSQL, params.OfferPublicID).Scan(
			&offerID, &offerPublicID, &storeID, &offerType, &title, &imageURL,
			&originalPrice, &discountPrice, &qtyAvailable, &maxPerUser,
			&pickupStart, &pickupEnd, &cancellationDeadline, &offerStatus,
			&storePublicID, &storeName, &storeCity, &storeAddress,
			&storePickupInstructions,
			&storeLat, &storeLng,
			&storeStatus, &acceptsOrders, &storeDeleted,
			&categoryName, &commissionRate, &serviceFee,
		); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return repository.ErrOfferNotFound
			}
			return fmt.Errorf("load offer: %w", err)
		}

		if storeDeleted || storeStatus != string(enum.StoreStatusApproved) || !acceptsOrders {
			return repository.ErrOfferUnavailable
		}
		if offerStatus != "active" || !pickupEnd.After(time.Now()) {
			return repository.ErrOfferUnavailable
		}
		if maxPerUser > 0 && params.Quantity > maxPerUser {
			return repository.ErrExceedsMaxPerUser
		}
		if qtyAvailable < params.Quantity {
			return repository.ErrInsufficientStock
		}

		// Atomic inventory decrement prevents overselling under concurrency.
		tag, err := tx.Exec(ctx, atomicDecrementOfferStockSQL, params.Quantity, offerID)
		if err != nil {
			return fmt.Errorf("decrement offer stock: %w", err)
		}
		if tag.RowsAffected() != 1 {
			return repository.ErrInsufficientStock
		}

		itemsTotal := discountPrice * int64(params.Quantity)
		totalAmount := itemsTotal + serviceFee
		commissionAmount := int64(math.Round(float64(itemsTotal) * commissionRate / 100.0))
		if commissionAmount > itemsTotal {
			commissionAmount = itemsTotal
		}
		vendorPayout := itemsTotal - commissionAmount

		var idemKeyArg, idemHashArg any
		if params.IdempotencyKey != "" {
			idemKeyArg = params.IdempotencyKey
			idemHashArg = params.IdempotencyPayloadHash
		}

		var (
			orderID       int64
			orderPublicID string
			pickupCode    string
			createdAt     time.Time
		)
		for attempt := 0; attempt < maxPickupCodeAttempts; attempt++ {
			code, err := generatePickupCode()
			if err != nil {
				return fmt.Errorf("generate pickup code: %w", err)
			}
			qrToken, err := generateQRToken()
			if err != nil {
				return fmt.Errorf("generate qr token: %w", err)
			}

			// Savepoint lets us retry a live pickup_code collision without aborting the tx.
			if _, err := tx.Exec(ctx, "SAVEPOINT sp_insert_order"); err != nil {
				return fmt.Errorf("savepoint: %w", err)
			}
			err = tx.QueryRow(ctx, insertReservationOrderSQL,
				userID, storeID, idemKeyArg, idemHashArg,
				itemsTotal, serviceFee, totalAmount,
				commissionRate, commissionAmount, vendorPayout,
				code, qrToken,
			).Scan(&orderID, &orderPublicID, &createdAt)
			if err == nil {
				pickupCode = code
				_, _ = tx.Exec(ctx, "RELEASE SAVEPOINT sp_insert_order")
				break
			}
			_, _ = tx.Exec(ctx, "ROLLBACK TO SAVEPOINT sp_insert_order")

			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolationCode {
				if pgErr.ConstraintName == idemConstraintName {
					return errConcurrentIdempotencyRace
				}
				if pgErr.ConstraintName == liveCodeConstraintName && attempt < maxPickupCodeAttempts-1 {
					continue
				}
			}
			return fmt.Errorf("insert order: %w", err)
		}

		if _, err := tx.Exec(ctx, insertReservationItemSQL,
			orderID, storeID, offerID, title,
			params.Quantity, originalPrice, discountPrice,
		); err != nil {
			return fmt.Errorf("insert order item: %w", err)
		}

		ps := pickupStart.UTC()
		pe := pickupEnd.UTC()
		var cd *time.Time
		if cancellationDeadline != nil {
			t := cancellationDeadline.UTC()
			cd = &t
		}
		lat, lng := storeLat, storeLng
		result = &repository.CreateReservationResult{
			Reservation: &model.Reservation{
				ID:         orderPublicID,
				Reference:  model.ReservationReference(orderPublicID),
				PickupCode: pickupCode,
				Status:     enum.OrderStatusPaid,
				Store: model.ReservationStore{
					ID:                 storePublicID,
					Name:               storeName,
					City:               storeCity,
					AddressLine:        storeAddress,
					PickupInstructions: storePickupInstructions,
					Latitude:           &lat,
					Longitude:          &lng,
				},
				Offer: model.ReservationOffer{
					ID:                offerPublicID,
					Title:             title,
					OfferType:         offerType,
					ImageURL:          imageURL,
					CategoryName:      categoryName,
					Quantity:          params.Quantity,
					UnitPrice:         discountPrice,
					UnitOriginalPrice: originalPrice,
				},
				ItemsTotal:           itemsTotal,
				ServiceFee:           serviceFee,
				TotalAmount:          totalAmount,
				Currency:             defaultCurrencyToman,
				PickupStart:          &ps,
				PickupEnd:            &pe,
				CancellationDeadline: cd,
				CreatedAt:            createdAt.UTC(),
			},
			Replayed: false,
		}
		return nil
	})
	if errors.Is(err, errConcurrentIdempotencyRace) {
		// Another concurrent request with the same Idempotency-Key committed first;
		// our transaction rolled back its stock decrement. Return the winning record.
		return r.resolveIdempotentReplay(ctx, params)
	}
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (r *reservationRepository) resolveIdempotentReplay(
	ctx context.Context,
	params repository.CreateReservationParams,
) (*repository.CreateReservationResult, error) {
	var userID int64
	if err := r.pool.QueryRow(ctx, findUserIDByPublicIDSQL, params.UserPublicID).Scan(&userID); err != nil {
		return nil, fmt.Errorf("resolve user after idempotency race: %w", err)
	}
	var existingPublicID, existingHash string
	if err := r.pool.QueryRow(ctx, findIdempotentOrderSQL, userID, params.IdempotencyKey).
		Scan(&existingPublicID, &existingHash); err != nil {
		return nil, fmt.Errorf("load idempotent order after race: %w", err)
	}
	if existingHash != "" && existingHash != params.IdempotencyPayloadHash {
		return nil, repository.ErrIdempotencyConflict
	}
	res, err := r.GetByPublicIDAndUser(ctx, existingPublicID, params.UserPublicID)
	if err != nil {
		return nil, err
	}
	return &repository.CreateReservationResult{Reservation: res, Replayed: true}, nil
}

func (r *reservationRepository) GetByPublicIDAndUser(
	ctx context.Context,
	reservationPublicID, userPublicID string,
) (*model.Reservation, error) {
	if !security.IsID(reservationPublicID) || !security.IsID(userPublicID) {
		return nil, repository.ErrReservationNotFound
	}
	return r.getByPublicIDAndUser(ctx, r.pool, reservationPublicID, userPublicID)
}

func (r *reservationRepository) getByPublicIDAndUser(
	ctx context.Context,
	db DB,
	reservationPublicID, userPublicID string,
) (*model.Reservation, error) {
	var (
		res                    model.Reservation
		status                 string
		lat, lng               float64
		pickupStart, pickupEnd time.Time
		cancellationDeadline   *time.Time
	)
	err := db.QueryRow(ctx, getReservationByPublicIDAndUserSQL, reservationPublicID, userPublicID).Scan(
		&res.ID, &res.PickupCode, &status,
		&res.ItemsTotal, &res.ServiceFee, &res.TotalAmount, &res.CreatedAt,
		&res.Store.ID, &res.Store.Name, &res.Store.City, &res.Store.AddressLine,
		&res.Store.PickupInstructions,
		&lat, &lng,
		&res.Offer.ID, &res.Offer.Title, &res.Offer.OfferType, &res.Offer.ImageURL,
		&res.Offer.CategoryName,
		&res.Offer.Quantity, &res.Offer.UnitPrice, &res.Offer.UnitOriginalPrice,
		&pickupStart, &pickupEnd, &cancellationDeadline,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, repository.ErrReservationNotFound
		}
		return nil, fmt.Errorf("get reservation: %w", err)
	}
	res.Reference = model.ReservationReference(res.ID)
	res.Status = enum.OrderStatus(status)
	res.Currency = defaultCurrencyToman
	res.Store.Latitude = &lat
	res.Store.Longitude = &lng
	ps, pe := pickupStart.UTC(), pickupEnd.UTC()
	res.PickupStart = &ps
	res.PickupEnd = &pe
	if cancellationDeadline != nil {
		cd := cancellationDeadline.UTC()
		res.CancellationDeadline = &cd
	}
	res.CreatedAt = res.CreatedAt.UTC()
	return &res, nil
}

func generatePickupCode() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(10000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%04d", n.Int64()), nil
}

func generateQRToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

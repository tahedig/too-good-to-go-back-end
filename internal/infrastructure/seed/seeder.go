package seed

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tahedig/too-good-to-go/backend/internal/bootstrap"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/enum"
	"github.com/tahedig/too-good-to-go/backend/pkg/validation"
)

// Summary reports the counts of seeded records after a Run.
type Summary struct {
	AdminUsers    int
	VendorUsers   int
	CustomerUsers int
	Stores        int
	Offers        int
	Orders        int
}

// Run seeds demo accounts, vendor profiles, stores, offers, and reservations
// idempotently inside a single transaction. It refuses to run in production.
func Run(ctx context.Context, pool *pgxpool.Pool, cfg Config) (*Summary, error) {
	if strings.EqualFold(strings.TrimSpace(cfg.AppEnv), bootstrap.EnvProduction) {
		return nil, errors.New("refusing to seed demo data when APP_ENV=production")
	}

	adminPhone, err := validation.NormalizePhone(cfg.AdminPhone)
	if err != nil {
		return nil, fmt.Errorf("invalid admin phone: %w", err)
	}
	if len(cfg.VendorPhones) < 2 {
		return nil, errors.New("at least 2 vendor phones are required")
	}
	if len(cfg.CustomerPhones) < 2 {
		return nil, errors.New("at least 2 customer phones are required")
	}

	vendorPhones := make([]string, len(cfg.VendorPhones))
	for i, raw := range cfg.VendorPhones {
		p, err := validation.NormalizePhone(raw)
		if err != nil {
			return nil, fmt.Errorf("invalid vendor phone[%d]: %w", i, err)
		}
		vendorPhones[i] = p
	}

	customerPhones := make([]string, len(cfg.CustomerPhones))
	for i, raw := range cfg.CustomerPhones {
		p, err := validation.NormalizePhone(raw)
		if err != nil {
			return nil, fmt.Errorf("invalid customer phone[%d]: %w", i, err)
		}
		customerPhones[i] = p
	}

	now := time.Now().UTC()
	summary := &Summary{}

	err = pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		adminID, err := upsertUserWithRole(ctx, tx, adminPhone, "ادمین دمو", "admin")
		if err != nil {
			return fmt.Errorf("seed admin: %w", err)
		}
		summary.AdminUsers = 1

		vendorIDs := make([]int64, len(vendorPhones))
		vendorNames := []string{"فروشنده اول دمو (علی رضایی)", "فروشنده دوم دمو (مریم کریمی)"}
		businessNames := []string{"گروه غذایی ولیعصر", "هلدینگ کافه و سلامت رشد"}
		for i, phone := range vendorPhones {
			name := fmt.Sprintf("فروشنده دمو %d", i+1)
			if i < len(vendorNames) {
				name = vendorNames[i]
			}
			uid, err := upsertUserWithRole(ctx, tx, phone, name, "vendor_owner")
			if err != nil {
				return fmt.Errorf("seed vendor user %d: %w", i, err)
			}
			biz := fmt.Sprintf("کسب‌وکار دمو %d", i+1)
			if i < len(businessNames) {
				biz = businessNames[i]
			}
			vid, err := ensureVendor(ctx, tx, uid, biz, phone)
			if err != nil {
				return fmt.Errorf("seed vendor %d: %w", i, err)
			}
			vendorIDs[i] = vid
		}
		summary.VendorUsers = len(vendorIDs)

		customerIDs := make([]int64, len(customerPhones))
		customerNames := []string{"سارا محمدی (مشتری دمو)", "امیر حسینی (مشتری دمو)"}
		for i, phone := range customerPhones {
			name := fmt.Sprintf("مشتری دمو %d", i+1)
			if i < len(customerNames) {
				name = customerNames[i]
			}
			uid, err := upsertUserWithRole(ctx, tx, phone, name, "customer")
			if err != nil {
				return fmt.Errorf("seed customer user %d: %w", i, err)
			}
			if err := ensureCustomerAddress(ctx, tx, uid, i); err != nil {
				return fmt.Errorf("seed customer address %d: %w", i, err)
			}
			customerIDs[i] = uid
		}
		summary.CustomerUsers = len(customerIDs)

		storeIDs := make(map[string]int64, len(defaultStores))
		for i, st := range defaultStores {
			vid := vendorIDs[st.VendorIndex%len(vendorIDs)]
			sid, err := upsertStore(ctx, tx, st, vid, adminID, now, i)
			if err != nil {
				return fmt.Errorf("seed store %s: %w", st.Slug, err)
			}
			storeIDs[st.Slug] = sid
		}
		summary.Stores = len(storeIDs)

		type offerMeta struct {
			id            int64
			storeID       int64
			title         string
			originalPrice int64
			discountPrice int64
		}
		offerMetas := make(map[string]offerMeta, len(defaultOffers))
		for _, off := range defaultOffers {
			sid, ok := storeIDs[off.StoreSlug]
			if !ok {
				return fmt.Errorf("unknown store slug %q for offer %s", off.StoreSlug, off.PublicID)
			}
			oid, err := upsertOffer(ctx, tx, off, sid, now)
			if err != nil {
				return fmt.Errorf("seed offer %s: %w", off.PublicID, err)
			}
			offerMetas[off.PublicID] = offerMeta{
				id:            oid,
				storeID:       sid,
				title:         off.Title,
				originalPrice: off.OriginalPrice,
				discountPrice: off.DiscountPrice,
			}
		}
		summary.Offers = len(offerMetas)

		for _, ord := range defaultOrders {
			cid := customerIDs[ord.CustomerIndex%len(customerIDs)]
			om, ok := offerMetas[ord.OfferPublicID]
			if !ok {
				return fmt.Errorf("unknown offer %q for order %s", ord.OfferPublicID, ord.PublicID)
			}
			if err := ensureOrder(ctx, tx, ord, cid, om.storeID, om.id, om.title, om.originalPrice, om.discountPrice, adminID, now); err != nil {
				return fmt.Errorf("seed order %s: %w", ord.PublicID, err)
			}
		}
		summary.Orders = len(defaultOrders)

		return nil
	})
	if err != nil {
		return nil, err
	}
	return summary, nil
}

func upsertUserWithRole(ctx context.Context, tx pgx.Tx, phone, fullName, roleCode string) (int64, error) {
	var userID int64
	err := tx.QueryRow(ctx, `
		INSERT INTO users (phone, full_name, status, last_login_at)
		VALUES ($1, $2, 'active', now())
		ON CONFLICT (phone) DO UPDATE
			SET full_name = COALESCE(NULLIF(users.full_name, ''), EXCLUDED.full_name)
		RETURNING id`,
		phone, fullName,
	).Scan(&userID)
	if err != nil {
		return 0, err
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO user_roles (user_id, role_id)
		SELECT $1, id FROM roles WHERE code = $2
		ON CONFLICT DO NOTHING`,
		userID, roleCode,
	); err != nil {
		return 0, err
	}
	return userID, nil
}

func ensureVendor(ctx context.Context, tx pgx.Tx, ownerUserID int64, businessName, contactPhone string) (int64, error) {
	var vendorID int64
	err := tx.QueryRow(ctx, `
		SELECT id FROM vendors WHERE owner_user_id = $1 AND business_name = $2 LIMIT 1`,
		ownerUserID, businessName,
	).Scan(&vendorID)
	if err == nil {
		return vendorID, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return 0, err
	}

	err = tx.QueryRow(ctx, `
		INSERT INTO vendors (owner_user_id, business_name, contact_phone)
		VALUES ($1, $2, $3)
		RETURNING id`,
		ownerUserID, businessName, contactPhone,
	).Scan(&vendorID)
	return vendorID, err
}

func ensureCustomerAddress(ctx context.Context, tx pgx.Tx, userID int64, index int) error {
	var exists bool
	if err := tx.QueryRow(ctx, `
		SELECT EXISTS(SELECT 1 FROM user_addresses WHERE user_id = $1 AND deleted_at IS NULL)`,
		userID,
	).Scan(&exists); err != nil {
		return err
	}
	if exists {
		return nil
	}
	_, err := tx.Exec(ctx, `
		INSERT INTO user_addresses (user_id, label, street_name, plaque_number, unit_number, is_default)
		VALUES ($1, 'خانه', $2, $3, $4, true)`,
		userID,
		fmt.Sprintf("خیابان ولیعصر، کوچه %d", index+1),
		fmt.Sprintf("%d", 10+index*2),
		fmt.Sprintf("%d", index+1),
	)
	return err
}

func upsertStore(ctx context.Context, tx pgx.Tx, st seedStore, vendorID, adminID int64, now time.Time, idx int) (int64, error) {
	var approvedBy *int64
	var approvedAt *time.Time
	if st.Status == enum.StoreStatusApproved {
		approvedBy = &adminID
		t := now.Add(-7 * 24 * time.Hour)
		approvedAt = &t
	}

	var storeID int64
	err := tx.QueryRow(ctx, `
		INSERT INTO stores (
			public_id, vendor_id, category_id, owner_name, name, slug, biography,
			phone, city, district, address_line, pickup_instructions, location,
			cover_image_url, status, approved_by, approved_at, accepts_orders,
			rating_avg, rating_count, meals_saved
		) VALUES (
			$1::uuid, $2, $3, $4, $5, $6, $7,
			$8, $9, $10, $11, $12, ST_SetSRID(ST_MakePoint($13, $14), 4326)::geography,
			$15, $16::store_status, $17, $18, $19,
			$20, $21, $22
		)
		ON CONFLICT (slug) DO UPDATE SET
			name = EXCLUDED.name,
			biography = EXCLUDED.biography,
			pickup_instructions = EXCLUDED.pickup_instructions
		RETURNING id`,
		st.PublicID, vendorID, st.CategoryID, st.OwnerName, st.Name, st.Slug, st.Biography,
		st.Phone, st.City, st.District, st.AddressLine, st.PickupInstructions, st.Lng, st.Lat,
		st.ImageURL, string(st.Status), approvedBy, approvedAt, st.AcceptsOrders,
		st.RatingAvg, st.RatingCount, st.MealsSaved,
	).Scan(&storeID)
	if err != nil {
		return 0, err
	}

	for day := int16(0); day <= 6; day++ {
		if _, err := tx.Exec(ctx, `
			INSERT INTO store_working_hours (store_id, day_of_week, open_time, close_time, is_closed)
			VALUES ($1, $2, '09:00', '22:00', false)
			ON CONFLICT (store_id, day_of_week) DO NOTHING`,
			storeID, day,
		); err != nil {
			return 0, err
		}
	}

	imgPublicID := fmt.Sprintf("44444444-4444-4444-8444-444444444%03d", idx+1)
	if _, err := tx.Exec(ctx, `
		INSERT INTO store_images (public_id, store_id, image_url, is_primary, sort_order)
		VALUES ($1::uuid, $2, $3, true, 0)
		ON CONFLICT (public_id) DO NOTHING`,
		imgPublicID, storeID, st.ImageURL,
	); err != nil {
		return 0, err
	}

	if st.Status == enum.StoreStatusApproved {
		var histExists bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM store_status_history WHERE store_id = $1)`, storeID).Scan(&histExists); err != nil {
			return 0, err
		}
		if !histExists {
			if _, err := tx.Exec(ctx, `
				INSERT INTO store_status_history (store_id, from_status, to_status, action, admin_id)
				VALUES ($1, 'pending', 'approved', 'approve', $2)`,
				storeID, adminID,
			); err != nil {
				return 0, err
			}
		}
	}

	return storeID, nil
}

func upsertOffer(ctx context.Context, tx pgx.Tx, off seedOffer, storeID int64, now time.Time) (int64, error) {
	pickupStart := now.Add(off.PickupStartOffset)
	pickupEnd := now.Add(off.PickupEndOffset)
	cancelDeadline := pickupStart.Add(-1 * time.Hour)
	var publishedAt *time.Time
	if off.Status != "draft" {
		t := now.Add(-2 * time.Hour)
		publishedAt = &t
	}

	var offerID int64
	err := tx.QueryRow(ctx, `
		INSERT INTO offers (
			public_id, store_id, offer_type, title, description, category_id,
			original_price, discount_price, quantity_total, quantity_available, max_per_user,
			pickup_start, pickup_end, cancellation_deadline,
			allergens, allergens_declared, status, published_at
		) VALUES (
			$1::uuid, $2, $3::offer_type, $4, $5, $6,
			$7, $8, $9, $10, $11,
			$12, $13, $14,
			$15::allergen_type[], $16, $17::offer_status, $18
		)
		ON CONFLICT (public_id) DO UPDATE SET
			pickup_start = EXCLUDED.pickup_start,
			pickup_end = EXCLUDED.pickup_end,
			cancellation_deadline = EXCLUDED.cancellation_deadline
		RETURNING id`,
		off.PublicID, storeID, off.OfferType, off.Title, off.Description, off.CategoryID,
		off.OriginalPrice, off.DiscountPrice, off.QuantityTotal, off.QuantityAvailable, off.MaxPerUser,
		pickupStart, pickupEnd, cancelDeadline,
		off.Allergens, off.AllergensDeclared, off.Status, publishedAt,
	).Scan(&offerID)
	if err != nil {
		return 0, err
	}

	for _, allergen := range off.Allergens {
		if _, err := tx.Exec(ctx, `
			INSERT INTO offer_allergens (offer_id, allergen_code)
			VALUES ($1, $2)
			ON CONFLICT DO NOTHING`,
			offerID, allergen,
		); err != nil {
			return 0, err
		}
	}

	return offerID, nil
}

func ensureOrder(
	ctx context.Context,
	tx pgx.Tx,
	ord seedOrder,
	customerID, storeID, offerID int64,
	offerTitle string,
	unitOriginalPrice, unitPrice int64,
	adminID int64,
	now time.Time,
) error {
	var existingID int64
	err := tx.QueryRow(ctx, `SELECT id FROM orders WHERE public_id = $1::uuid`, ord.PublicID).Scan(&existingID)
	if err == nil {
		return nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}

	itemsTotal := unitPrice * int64(ord.Quantity)
	const commissionRate = 15.0
	commissionAmount := (itemsTotal * 15) / 100
	vendorPayout := itemsTotal - commissionAmount
	createdAt := now.Add(ord.CreatedOffset)
	reservedUntil := now.Add(ord.ReservedOffset)

	var paidAt, readyAt, completedAt, cancelledAt *time.Time
	var confirmedBy *int64
	var cancelledBy, cancelReason *string

	switch ord.Status {
	case enum.OrderStatusPaid:
		t := createdAt.Add(2 * time.Minute)
		paidAt = &t
	case enum.OrderStatusReady:
		t1 := createdAt.Add(2 * time.Minute)
		t2 := createdAt.Add(5 * time.Minute)
		paidAt, readyAt = &t1, &t2
	case enum.OrderStatusCompleted:
		t1 := createdAt.Add(2 * time.Minute)
		t2 := createdAt.Add(5 * time.Minute)
		t3 := createdAt.Add(20 * time.Minute)
		paidAt, readyAt, completedAt = &t1, &t2, &t3
		confirmedBy = &adminID
	case enum.OrderStatusNoShow:
		t1 := createdAt.Add(2 * time.Minute)
		t2 := createdAt.Add(5 * time.Minute)
		paidAt, readyAt = &t1, &t2
	case enum.OrderStatusCancelled:
		t := createdAt.Add(4 * time.Minute)
		cancelledAt = &t
		if ord.WithPayment {
			tp := createdAt.Add(1 * time.Minute)
			paidAt = &tp
		}
		if ord.CancelledBy != "" {
			cb := ord.CancelledBy
			cancelledBy = &cb
		}
		if ord.CancelReason != "" {
			cr := ord.CancelReason
			cancelReason = &cr
		}
	}

	var orderID int64
	err = tx.QueryRow(ctx, `
		INSERT INTO orders (
			public_id, user_id, store_id, status, idempotency_key,
			items_total, service_fee, total_amount, commission_rate, commission_amount, vendor_payout,
			pickup_code, qr_token, reserved_until, confirmed_by,
			paid_at, ready_at, completed_at, cancelled_at, cancelled_by, cancel_reason,
			created_at
		) VALUES (
			$1::uuid, $2, $3, $4::order_status, $5,
			$6, 0, $6, $7, $8, $9,
			$10, $11, $12, $13,
			$14, $15, $16, $17, $18::actor_type, $19,
			$20
		)
		RETURNING id`,
		ord.PublicID, customerID, storeID, string(ord.Status), ord.IdempotencyKey,
		itemsTotal, commissionRate, commissionAmount, vendorPayout,
		ord.PickupCode, ord.QRToken, reservedUntil, confirmedBy,
		paidAt, readyAt, completedAt, cancelledAt, cancelledBy, cancelReason,
		createdAt,
	).Scan(&orderID)
	if err != nil {
		return err
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO order_items (
			order_id, store_id, offer_id, title_snapshot, quantity, unit_original_price, unit_price
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (order_id, offer_id) DO NOTHING`,
		orderID, storeID, offerID, offerTitle, ord.Quantity, unitOriginalPrice, unitPrice,
	); err != nil {
		return err
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO order_status_history (order_id, from_status, to_status, actor_user_id, reason, created_at)
		VALUES ($1, NULL, $2::order_status, $3, $4, $5)`,
		orderID, string(ord.Status), customerID, cancelReason, createdAt,
	); err != nil {
		return err
	}

	if ord.WithPayment && itemsTotal > 0 {
		authority := "seed-auth-" + ord.IdempotencyKey
		providerRef := "seed-ref-" + ord.PickupCode
		if _, err := tx.Exec(ctx, `
			INSERT INTO payments (order_id, provider, amount, status, authority, provider_ref, paid_at)
			VALUES ($1, 'internal_sandbox', $2, 'succeeded', $3, $4, $5)
			ON CONFLICT (provider, authority) DO NOTHING`,
			orderID, itemsTotal, authority, providerRef, paidAt,
		); err != nil {
			return err
		}
	}

	return nil
}

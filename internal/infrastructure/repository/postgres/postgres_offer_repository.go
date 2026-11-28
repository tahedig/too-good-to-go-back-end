package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tahedig/too-good-to-go/backend/internal/domain/model"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/repository"
)

type offerRepository struct {
	pool *pgxpool.Pool
}

// NewOfferRepository returns a PostgreSQL-backed OfferRepository.
func NewOfferRepository(pool *pgxpool.Pool) repository.OfferRepository {
	return &offerRepository{pool: pool}
}

func (r *offerRepository) ListAvailableByStoreID(ctx context.Context, storeID int64) ([]*model.Offer, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, public_id::text, store_id, title, description, image_url,
		       original_price, discount_price, currency, quantity_available,
		       pickup_start, pickup_end,
		       (status = 'active' AND deleted_at IS NULL) AS is_active,
		       created_at, updated_at
		FROM offers
		WHERE store_id = $1
		  AND deleted_at IS NULL
		  AND status = 'active'
		  AND quantity_available > 0
		  AND pickup_end > now()
		ORDER BY pickup_end ASC, id ASC`, storeID)
	if err != nil {
		return nil, fmt.Errorf("list available offers by store: %w", err)
	}
	defer rows.Close()

	var offers []*model.Offer
	for rows.Next() {
		var o model.Offer
		if err := rows.Scan(
			&o.ID, &o.PublicID, &o.StoreID, &o.Title, &o.Description, &o.ImageURL,
			&o.OriginalPrice, &o.Price, &o.Currency, &o.QuantityAvailable,
			&o.PickupStart, &o.PickupEnd, &o.IsActive,
			&o.CreatedAt, &o.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan offer: %w", err)
		}
		offers = append(offers, &o)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate offers: %w", err)
	}
	return offers, nil
}

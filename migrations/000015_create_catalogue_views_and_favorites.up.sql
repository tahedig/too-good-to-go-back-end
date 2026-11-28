-- Customer saved/favorite offers (Figma Customer / Personal Area: محصولات ذخیره‌شده).
CREATE TABLE favorite_offers (
    user_id    BIGINT      NOT NULL REFERENCES users(id)  ON DELETE CASCADE,
    offer_id   BIGINT      NOT NULL REFERENCES offers(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, offer_id)
);
CREATE INDEX idx_favorite_offers_offer ON favorite_offers (offer_id);

-- Great-circle distance in whole meters (haversine). NULL when any coordinate is NULL.
CREATE OR REPLACE FUNCTION fn_distance_m(lat1 float8, lng1 float8, lat2 float8, lng2 float8) RETURNS bigint
LANGUAGE sql IMMUTABLE STRICT PARALLEL SAFE AS $$
    SELECT round(2 * 6371000 * asin(sqrt(
        power(sin(radians(lat2 - lat1) / 2), 2) +
        cos(radians(lat1)) * cos(radians(lat2)) * power(sin(radians(lng2 - lng1) / 2), 2))))::bigint
$$;

-- Single source of truth for "can a customer see/buy this offer right now?".
-- Built on top of the unified PostGIS stores/offers/categories schema so home,
-- catalogue, map, favorites, and product details all share one eligibility definition.
CREATE VIEW eligible_offers AS
SELECT o.id, o.public_id, o.store_id, COALESCE(o.category_id, s.category_id) AS category_id,
       c.name_fa AS category_name, c.slug AS category_slug, c.kind AS category_kind,
       o.title, o.description, o.image_url,
       o.original_price, o.discount_price AS price, o.currency, o.quantity_available,
       o.pickup_start, o.pickup_end, o.cancellation_deadline, o.created_at,
       o.discount_percent::int AS discount_percent,
       s.public_id AS store_public_id, s.name AS store_name, s.logo_url AS store_logo_url,
       s.district  AS store_district,
       ST_Y(s.location::geometry) AS store_lat,
       ST_X(s.location::geometry) AS store_lng
FROM offers o
JOIN stores s     ON s.id = o.store_id
JOIN categories c ON c.id = COALESCE(o.category_id, s.category_id)
WHERE o.deleted_at IS NULL
  AND o.status = 'active'
  AND o.quantity_available > 0
  AND o.pickup_end > now()
  AND s.deleted_at IS NULL
  AND s.status = 'approved'
  AND s.accepts_orders
  AND c.is_active;

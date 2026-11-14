-- Great-circle distance in whole meters (haversine). NULL when any coordinate is NULL.
CREATE FUNCTION fn_distance_m(lat1 float8, lng1 float8, lat2 float8, lng2 float8) RETURNS bigint
LANGUAGE sql IMMUTABLE STRICT PARALLEL SAFE AS $$
    SELECT round(2 * 6371000 * asin(sqrt(
        power(sin(radians(lat2 - lat1) / 2), 2) +
        cos(radians(lat1)) * cos(radians(lat2)) * power(sin(radians(lng2 - lng1) / 2), 2))))::bigint
$$;

-- Single source of truth for "can a customer see/buy this offer right now?".
-- Catalogue, store offer lists and home sections all read from this view, so availability and
-- pricing rules can never drift between pages. Vendor edits show up immediately (no caching layer).
CREATE VIEW eligible_offers AS
SELECT o.id, o.public_id, o.store_id, o.category_id,
       c.name_fa AS category_name, c.slug AS category_slug, c.kind AS category_kind,
       o.title, o.description, o.image_url,
       o.original_price, o.price, o.currency, o.quantity_available,
       o.pickup_start, o.pickup_end, o.created_at,
       CASE WHEN o.original_price > 0
            THEN round(100.0 * (o.original_price - o.price) / o.original_price)::int ELSE 0 END AS discount_percent,
       s.public_id AS store_public_id, s.name AS store_name, s.logo_url AS store_logo_url,
       s.district  AS store_district, s.latitude AS store_lat, s.longitude AS store_lng
FROM offers o
JOIN stores s     ON s.id = o.store_id
JOIN categories c ON c.id = o.category_id
WHERE o.deleted_at IS NULL AND o.is_active AND o.quantity_available > 0 AND o.pickup_end > now()
  AND s.deleted_at IS NULL AND s.is_active AND s.accepts_orders AND s.vendor_status IN ('approved', 'active')
  AND c.is_active;

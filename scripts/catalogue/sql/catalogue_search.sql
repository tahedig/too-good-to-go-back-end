-- Catalogue / store-offers / favorites-list query. pgx uses $n placeholders; here it is a PREPARE for testing.
-- $1 q (already escaped for LIKE and wrapped as %text%, or NULL)   $2 category ids smallint[]   $3 category kind
-- $4 price_min  $5 price_max  $6 pickup_from  $7 pickup_to  $8 excluded allergen codes text[]  $9 store public id
-- $10 lat  $11 lng  $12 sort: nearest|price_asc|price_desc|newest  $13 cursor sort_key  $14 cursor id
-- $15 user id (is_favorite)  $16 limit (service passes page size + 1 to detect "has more")  $17 radius km
PREPARE catalogue_search (text, smallint[], category_kind, bigint, bigint, timestamptz, timestamptz, text[], uuid,
                          float8, float8, text, bigint, bigint, bigint, int, float8) AS
WITH base AS (
    SELECT e.*,
           fn_distance_m($10, $11, e.store_lat, e.store_lng) AS distance_m
    FROM eligible_offers e
    WHERE ($1  IS NULL OR e.title ILIKE $1 ESCAPE '\' OR e.store_name ILIKE $1 ESCAPE '\')
      AND ($2  IS NULL OR e.category_id = ANY($2))
      AND ($3  IS NULL OR e.category_kind = $3)
      AND ($4  IS NULL OR e.price >= $4)
      AND ($5  IS NULL OR e.price <= $5)
      AND ($6  IS NULL OR e.pickup_end   > $6)      -- delivery window overlaps the requested range
      AND ($7  IS NULL OR e.pickup_start < $7)
      AND ($8  IS NULL OR NOT EXISTS (SELECT 1 FROM offer_allergens oa
                                      WHERE oa.offer_id = e.id AND oa.allergen_code = ANY($8)))
      AND ($9  IS NULL OR e.store_public_id = $9)
), keyed AS (
    SELECT b.*, CASE $12 WHEN 'nearest'    THEN b.distance_m
                         WHEN 'price_asc'  THEN b.price
                         WHEN 'price_desc' THEN -b.price
                         ELSE -b.id END AS sort_key           -- newest = highest id first
    FROM base b
)
SELECT k.public_id, k.title, k.image_url, k.price, k.original_price, k.discount_percent, k.currency,
       k.quantity_available, k.pickup_start, k.pickup_end, k.category_slug, k.category_name,
       k.store_public_id, k.store_name, k.store_logo_url, k.distance_m, k.sort_key, k.id,
       EXISTS (SELECT 1 FROM favorite_offers f WHERE f.user_id = $15 AND f.offer_id = k.id) AS is_favorite
FROM keyed k
WHERE ($13 IS NULL OR (k.sort_key, k.id) > ($13, $14))      -- keyset pagination: stable, no OFFSET
  AND ($17 IS NULL OR k.distance_m <= $17 * 1000)
ORDER BY k.sort_key, k.id                                   -- id as final tie-breaker => stable order
LIMIT $16;

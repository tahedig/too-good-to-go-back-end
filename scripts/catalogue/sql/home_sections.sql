-- Home page. Each of the 3 sections is its OWN query: the app calls them independently, 10 items each.
-- The same query powers that section's "مشاهده همه" (view all): the client keeps calling it with the
-- cursor of the last row, 10 more each time. All sections read eligible_offers, same rules as the catalogue.
--
-- Common parameters:  $1 limit (service passes 10+1 to detect "has more")   $2 lat  $3 lng
--                     $4 radius km (popular_nearby only)   $5 $6 $7 cursor (k1, k2, id) or NULLs
--                     $8 user id (for is_favorite)
-- Common output:      card columns..., k1, k2, id   -> the next cursor is (k1, k2, id) of the last row

-- Category shortcuts (separate, tiny call)
PREPARE home_categories AS
SELECT id, slug, name_fa, kind FROM categories WHERE is_active ORDER BY sort_order, id;

-- 1) فرصت‌های خوشمزه امروز: pickup window starts today (Asia/Tehran); best discount first
PREPARE home_today (int, float8, float8, float8, bigint, bigint, bigint, bigint) AS
SELECT e.public_id, e.title, e.image_url, e.price, e.original_price, e.discount_percent, e.currency,
       e.quantity_available, e.pickup_start, e.pickup_end, e.store_public_id, e.store_name,
       fn_distance_m($2, $3, e.store_lat, e.store_lng) AS distance_m,
       (-e.discount_percent)::bigint AS k1, 0::bigint AS k2, e.id,
       EXISTS (SELECT 1 FROM favorite_offers f WHERE f.user_id = $8 AND f.offer_id = e.id) AS is_favorite
FROM eligible_offers e
WHERE e.pickup_start < ((date_trunc('day', now() AT TIME ZONE 'Asia/Tehran') + interval '1 day') AT TIME ZONE 'Asia/Tehran')
  AND ($5 IS NULL OR ((-e.discount_percent)::bigint, 0::bigint, e.id) > ($5, $6, $7))
ORDER BY k1, k2, e.id
LIMIT $1;

-- 2) محبوب‌های اطراف شما: inside the radius, most-favorited first, then nearest. No lat/lng => empty.
PREPARE home_popular_nearby (int, float8, float8, float8, bigint, bigint, bigint, bigint) AS
SELECT * FROM (
    SELECT e.public_id, e.title, e.image_url, e.price, e.original_price, e.discount_percent, e.currency,
           e.quantity_available, e.pickup_start, e.pickup_end, e.store_public_id, e.store_name,
           fn_distance_m($2, $3, e.store_lat, e.store_lng) AS distance_m,
           (-(SELECT count(*) FROM favorite_offers f WHERE f.offer_id = e.id))::bigint AS k1,
           fn_distance_m($2, $3, e.store_lat, e.store_lng) AS k2, e.id,
           EXISTS (SELECT 1 FROM favorite_offers f WHERE f.user_id = $8 AND f.offer_id = e.id) AS is_favorite
    FROM eligible_offers e) x
WHERE x.distance_m <= $4 * 1000
  AND ($5 IS NULL OR (x.k1, x.k2, x.id) > ($5, $6, $7))
ORDER BY x.k1, x.k2, x.id
LIMIT $1;

-- 3) آخرین فرصت‌های نجات غذا: at most 3 left or window closes within 2 hours; closing soonest first
PREPARE home_last_chance (int, float8, float8, float8, bigint, bigint, bigint, bigint) AS
SELECT e.public_id, e.title, e.image_url, e.price, e.original_price, e.discount_percent, e.currency,
       e.quantity_available, e.pickup_start, e.pickup_end, e.store_public_id, e.store_name,
       fn_distance_m($2, $3, e.store_lat, e.store_lng) AS distance_m,
       extract(epoch FROM e.pickup_end)::bigint AS k1, 0::bigint AS k2, e.id,
       EXISTS (SELECT 1 FROM favorite_offers f WHERE f.user_id = $8 AND f.offer_id = e.id) AS is_favorite
FROM eligible_offers e
WHERE (e.quantity_available <= 3 OR e.pickup_end <= now() + interval '2 hours')
  AND ($5 IS NULL OR (extract(epoch FROM e.pickup_end)::bigint, 0::bigint, e.id) > ($5, $6, $7))
ORDER BY k1, k2, e.id
LIMIT $1;

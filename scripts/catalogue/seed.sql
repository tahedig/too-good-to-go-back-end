-- DEV ONLY seed: the same data scripts/catalogue/test_catalogue_sql.sh uses, so Swagger results match the tests.
-- Refuses to run on a database that already has users. Test customer = users.id 2  (X-Dev-User-Id: 2).
-- Test location (store A, Valiasr): lat=35.7219  lng=51.4090   (store B is ~6 km west)
DO $$ BEGIN
    IF EXISTS (SELECT 1 FROM users) THEN
        RAISE EXCEPTION 'seed.sql is for an empty dev database (users table is not empty)';
    END IF;
END $$;

INSERT INTO users(phone, full_name) VALUES ('+989121110001','vendor'),('+989121110002','customer');
-- A: Valiasr (center), B: ~6 km west, C pending, D inactive, E deleted, F not accepting orders
INSERT INTO stores(vendor_id, vendor_status, category_id, name, slug, address_line, latitude, longitude, is_active, accepts_orders, deleted_at) VALUES
 (1,'approved',1,'نانوایی صبحگاهی','a','ولیعصر',35.7219,51.4090,true,true,NULL),
 (1,'active',   7,'پیتزا آراز',   'b','تهرانپارس',35.7219,51.3400,true,true,NULL),
 (1,'pending_review',1,'در انتظار','c','x',35.7,51.4,true,true,NULL),
 (1,'approved', 1,'غیرفعال','d','x',35.7,51.4,false,true,NULL),
 (1,'approved', 1,'حذف شده','e','x',35.7,51.4,true,true,now()),
 (1,'approved', 1,'سفارش نمی‌گیرد','f','x',35.7,51.4,true,false,NULL);
-- 12 eligible offers in A, 10 in B; prices tie on purpose (ties must not break paging)
INSERT INTO offers(store_id, category_id, title, original_price, price, quantity_available, pickup_start, pickup_end)
SELECT CASE WHEN i<=12 THEN 1 ELSE 2 END, CASE WHEN i<=12 THEN CASE WHEN i%2=0 THEN 1 ELSE 2 END ELSE 7 END,
       CASE WHEN i%4=0 THEN 'نان سنگک ' ELSE 'بسته سورپرایز ' END || i,
       100000, 40000 + (i%4)*10000, 1 + i%5, now() - interval '1 hour', now() + (i || ' hours')::interval
FROM generate_series(1,22) i;
-- ineligible variants (must never appear anywhere)
INSERT INTO offers(store_id, category_id, title, original_price, price, quantity_available, pickup_start, pickup_end, is_active, deleted_at) VALUES
 (1,1,'sold out',100,50,0, now()-interval '1h', now()+interval '2h', true,NULL),
 (1,1,'expired', 100,50,5, now()-interval '5h', now()-interval '1h', true,NULL),
 (1,1,'inactive',100,50,5, now()-interval '1h', now()+interval '2h', false,NULL),
 (1,1,'deleted', 100,50,5, now()-interval '1h', now()+interval '2h', true,now()),
 (3,1,'store pending',100,50,5, now()-interval '1h', now()+interval '2h', true,NULL),
 (4,1,'store inactive',100,50,5, now()-interval '1h', now()+interval '2h', true,NULL),
 (5,1,'store deleted',100,50,5, now()-interval '1h', now()+interval '2h', true,NULL),
 (6,1,'store closed orders',100,50,5, now()-interval '1h', now()+interval '2h', true,NULL);
INSERT INTO offer_allergens SELECT id,'gluten' FROM offers WHERE id%3=0;
INSERT INTO offer_allergens SELECT id,'dairy'  FROM offers WHERE id%5=0;
INSERT INTO offer_allergens VALUES (7,'egg'),(7,'sesame');
INSERT INTO favorite_offers(user_id, offer_id) VALUES (2,1),(2,2),(2,15);
-- the test script makes offer 1 the soonest-closing "last chance" offer with 3 left
UPDATE offers SET quantity_available = 3 WHERE id = 1;

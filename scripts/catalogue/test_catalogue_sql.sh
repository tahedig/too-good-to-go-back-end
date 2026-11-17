#!/usr/bin/env bash
# Tests migrations 000005-000009 + catalogue/home SQL on a throw-away local PostgreSQL (non-root, no Docker).
#   MAIN_MIGRATIONS=/path/to/repo/migrations bash test_catalogue_sql.sh
set -Eeuo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
MAIN="${MAIN_MIGRATIONS:-$HERE/../migrations}"
(( EUID != 0 )) || { echo "run as non-root"; exit 1; }
export PATH="$(dirname "$(ls /usr/lib/postgresql/*/bin/initdb | sort -V | tail -n1)"):$PATH"
T="$(mktemp -d)"; SOCK="$T/s"; mkdir "$SOCK"; PORT=55434
trap 'pg_ctl -D "$T/d" -m immediate stop >/dev/null 2>&1 || true; rm -rf "$T"' EXIT
initdb -D "$T/d" -U postgres --auth=trust -E UTF8 --locale=C >/dev/null
pg_ctl -D "$T/d" -o "-p $PORT -k $SOCK -c listen_addresses=''" -l "$T/log" -w start >/dev/null
export PGHOST="$SOCK" PGPORT=$PORT PGUSER=postgres
psql -X -qAt -c "CREATE DATABASE tgtg" -d postgres
Q() { psql -X -qAt -v ON_ERROR_STOP=1 -d tgtg "$@"; }
pass() { echo "PASS: $*"; }; fail() { echo "FAIL: $*"; exit 1; }
ok() { [[ "$1" == "$2" ]] && pass "$3" || fail "$3 (expected '$2' got '$1')"; }

up()   { for f in $(ls "$@" 2>/dev/null | sort); do Q -f "$f" >/dev/null; done; }
echo "== migrations up / down / up"
up "$MAIN"/*.up.sql "$HERE"/migrations/*.up.sql
ok "$(Q -c "SELECT count(*) FROM information_schema.tables WHERE table_schema='public'")" 13 "12 tables + 1 view after up"
for f in $(ls "$MAIN"/*.down.sql | sort -r); do Q -f "$f" >/dev/null; done
ok "$(Q -c "SELECT count(*) FROM information_schema.tables WHERE table_schema='public' AND table_name IN ('stores','offers','categories','allergens','favorite_offers','eligible_offers')")" 0 "down removes everything"
up "$MAIN"/*.up.sql
pass "up again"

echo "== seed"
Q <<'SQL'
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
-- ineligible variants (must never appear)
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
SQL
SQL_LOAD="\\i $HERE/sql/catalogue_search.sql"
srch() { # env: Q CAT KIND PMIN PMAX PFROM PTO ALG STORE LAT LNG SORT CK CI FAVUSER LIM RAD
  printf '%s\n' "$SQL_LOAD" "EXECUTE catalogue_search(${Q:-NULL},${CAT:-NULL},${KIND:-NULL},${PMIN:-NULL},${PMAX:-NULL},${PFROM:-NULL},${PTO:-NULL},${ALG:-NULL},${STORE:-NULL},${LAT:-NULL},${LNG:-NULL},'${SORT:-price_asc}',${CK:-NULL},${CI:-NULL},${FAVUSER:-NULL},${LIM:-1000},${RAD:-NULL});" \
   | psql -X -qAt -F'|' -v ON_ERROR_STOP=1 -d tgtg 2>&1 | { grep -v '^PREPARE$' || true; }
}
ids() { cut -d'|' -f2; }

echo "== eligibility"
ok "$(srch | wc -l)" 22 "only the 22 eligible offers are returned"
ok "$(srch | grep -cE 'sold out|expired|inactive|deleted|pending|closed orders')" 0 "no ineligible offer leaks"
ok "$(Q -c "SELECT count(*) FROM eligible_offers")" 22 "view agrees with catalogue"
Q -c "UPDATE offers SET quantity_available=0 WHERE id=1"
ok "$(srch | wc -l)" 21 "vendor edit (sold out) reflected immediately"
Q -c "UPDATE offers SET quantity_available=3 WHERE id=1"

echo "== pagination: pages of 10 == one full list, for every sort"
for S in price_asc price_desc nearest newest; do
  full="$(LAT=35.72 LNG=51.40 SORT=$S srch | cut -d'|' -f1 | tr '\n' ' ')"
  paged=""; pk=NULL; pid=NULL; n=0
  while :; do
    page="$(LAT=35.72 LNG=51.40 SORT=$S LIM=10 CK=$pk CI=$pid srch)"; [[ -z "$page" ]] && break
    paged+="$(echo "$page" | cut -d'|' -f1 | tr '\n' ' ')"; n=$((n+1))
    last="$(echo "$page" | tail -n1)"; pk="$(echo "$last" | cut -d'|' -f17)"; pid="$(echo "$last" | cut -d'|' -f18)"
    (( n < 10 )) || fail "pagination did not terminate"
  done
  [[ "$full" == "$paged" ]] && pass "sort=$S: $n pages, identical order, no duplicates/gaps" || fail "sort=$S paging differs"
done
ok "$(SORT=price_asc srch | cut -d'|' -f4 | head -n1)" 40000 "price_asc starts at the cheapest"
ok "$(SORT=price_desc srch | cut -d'|' -f4 | head -n1)" 70000 "price_desc starts at the most expensive"
ok "$(LAT=35.7219 LNG=51.4090 SORT=nearest srch | head -n1 | cut -d'|' -f16)" 0 "nearest: store at the user's location is first (0 m)"
d="$(LAT=35.7219 LNG=51.4090 SORT=nearest srch | tail -n1 | cut -d'|' -f16)"
(( d > 5500 && d < 6500 )) && pass "haversine: store B is ~6 km away ($d m)" || fail "distance $d"

echo "== filters"
ok "$(CAT="ARRAY[7]::smallint[]" srch | wc -l)" 10 "category filter (pizza)"
ok "$(KIND="'bakery'" srch | wc -l)" 12 "kind filter (bakery tab)"
ok "$(PMIN=50000 PMAX=60000 srch | cut -d'|' -f4 | sort -u | tr '\n' ,)" "50000,60000," "price range"
SID="'$(Q -c "SELECT public_id FROM stores WHERE slug='b'")'"
ok "$(STORE="$SID" srch | wc -l)" 10 "store filter (store B)"
ok "$(Q="'%سنگک%'" srch | wc -l)" 5 "Persian search by title"
ok "$(Q="'%پیتزا%'" srch | wc -l)" 10 "search matches store name"
ok "$(Q="'%\\%%'" srch | wc -l)" 0 "literal % in query is escaped, matches nothing"
ok "$(Q="'%'" srch | wc -l)" 22 "(unescaped wildcard would match all: Go layer must escape)"
exp="$(Q -c "SELECT count(*) FROM eligible_offers e WHERE NOT EXISTS (SELECT 1 FROM offer_allergens a WHERE a.offer_id=e.id AND a.allergen_code='gluten')")"
ok "$(ALG="ARRAY['gluten']" srch | wc -l)" "$exp" "exclude gluten"
exp="$(Q -c "SELECT count(*) FROM eligible_offers e WHERE NOT EXISTS (SELECT 1 FROM offer_allergens a WHERE a.offer_id=e.id AND a.allergen_code IN ('gluten','dairy','egg'))")"
ok "$(ALG="ARRAY['gluten','dairy','egg']" srch | wc -l)" "$exp" "exclude several allergens at once"
ok "$(PFROM="now()+interval '18 hours'" srch | wc -l)" 4 "delivery-window filter (windows ending after +18h: offers 19-22)"
ok "$(KIND="'bakery'" PMIN=50000 ALG="ARRAY['gluten']" Q="'%بسته%'" SORT=price_desc LIM=3 srch | wc -l)" 3 "filters + sort + limit together"
ok "$(RAD=3 LAT=35.7219 LNG=51.4090 SORT=nearest srch | wc -l)" 12 "radius 3 km keeps only store A"
ok "$(FAVUSER=2 srch | awk -F'|' '$19=="t"' | wc -l)" 3 "is_favorite flag per user"
ok "$(KIND="'grocery'" srch | wc -l)" 0 "empty result is just zero rows"

echo "== home sections: 3 independent calls, 10 each, each with its own view-all paging"
HOME="\\i $HERE/sql/home_sections.sql"
hq() { printf '%s\n' "$HOME" "$1" | psql -X -qAt -F'|' -v ON_ERROR_STOP=1 -d tgtg 2>&1 | { grep -v '^PREPARE$' || true; }; }
ok "$(hq 'EXECUTE home_categories;' | wc -l)" 21 "21 category shortcuts"
# args: limit, lat, lng, radius, c1, c2, cid, user
sec() { echo "EXECUTE $1(${LIMX:-10}, 35.7219, 51.4090, ${RADX:-8}, ${C1:-NULL}, ${C2:-NULL}, ${CID:-NULL}, 2);"; }
for S in home_today home_popular_nearby home_last_chance; do
  ok "$(hq "$(sec $S)" | wc -l)" 10 "$S: first call returns exactly 10"
  full="$(LIMX=1000 hq "$(LIMX=1000 sec $S)" | cut -d'|' -f1 | tr '\n' ' ')"
  paged=""; c1=NULL; c2=NULL; cid=NULL; n=0
  while :; do
    page="$(hq "$(C1=$c1 C2=$c2 CID=$cid sec $S)")"; [[ -z "$page" ]] && break
    paged+="$(echo "$page" | cut -d'|' -f1 | tr '\n' ' ')"; n=$((n+1))
    last="$(echo "$page" | tail -n1)"; c1="$(echo "$last" | cut -d'|' -f14)"; c2="$(echo "$last" | cut -d'|' -f15)"; cid="$(echo "$last" | cut -d'|' -f16)"
    (( n < 10 )) || fail "$S paging did not terminate"
  done
  [[ -n "$full" && "$full" == "$paged" ]] && pass "$S view-all: $n pages of 10, identical to full list, no duplicates/gaps" || fail "$S paging differs"
done
ok "$(hq "$(sec home_popular_nearby)" | head -n1 | cut -d'|' -f14)" -1 "popular: most-favorited first"
ok "$(hq "EXECUTE home_popular_nearby(10, 30.0, 50.0, 8, NULL, NULL, NULL, 2);" | wc -l)" 0 "popular nearby far away: empty, no error"
ok "$(hq "EXECUTE home_popular_nearby(10, NULL, NULL, 8, NULL, NULL, NULL, 2);" | wc -l)" 0 "popular nearby without location: empty, no error"
ok "$(hq "$(sec home_today)" | grep -cE 'sold out|expired|pending|inactive|deleted')" 0 "home uses the same eligibility as catalogue"
ok "$(hq "$(sec home_last_chance)" | head -n1 | cut -d'|' -f8)" 3 "last chance: soonest-closing offer first (qty 3)"
Q -c "UPDATE offers SET is_active=false" ; ok "$(hq "$(sec home_today)" | wc -l)" 0 "all section empty after vendor deactivates offers (immediately)"

echo "== constraints"
Q -c "INSERT INTO offers(store_id,category_id,title,original_price,price,quantity_available,pickup_start,pickup_end) VALUES (1,1,'x',100,200,1,now(),now()+interval '1h')" 2>/dev/null && fail "price>original accepted" || pass "price must be <= original price"
Q -c "INSERT INTO offers(store_id,category_id,title,original_price,price,quantity_available,pickup_start,pickup_end) VALUES (1,1,'x',100,50,1,now(),now()-interval '1h')" 2>/dev/null && fail "bad window accepted" || pass "delivery window must be valid"
Q -c "INSERT INTO favorite_offers VALUES (2,1)" 2>/dev/null && fail "duplicate favorite" || pass "duplicate favorite rejected (PK)"
echo "ALL CATALOGUE SQL CHECKS PASSED"

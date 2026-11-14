#!/usr/bin/env bash
# Same checks as test_migrations.sh but WITHOUT Docker: uses your local PostgreSQL.
# Creates a throw-away database, runs up -> down -> up, then drops it.
# Requires: psql, migrate CLI, the PostGIS extension installed in that PostgreSQL,
# and a superuser (CREATE EXTENSION postgis needs one).
#   ADMIN_URL=postgres://app:app@localhost:5432/postgres?sslmode=disable bash scripts/test_migrations_local.sh
set -euo pipefail

ADMIN_URL="${ADMIN_URL:-postgres://app:app@localhost:5432/postgres?sslmode=disable}"
TEST_DB="tgtg_migrate_test"
BASE="${ADMIN_URL%/*}"                          # postgres://app:app@localhost:5432
TEST_URL="${BASE}/${TEST_DB}?sslmode=disable"
MIGRATE="migrate -path migrations -database ${TEST_URL}"

admin_q() { psql "$ADMIN_URL" -v ON_ERROR_STOP=1 -tAc "$1"; }
psql_q()  { psql "$TEST_URL"  -v ON_ERROR_STOP=1 -tAc "$1"; }
# Base tables only: PostGIS adds the spatial_ref_sys table and some views.
count_tables() { psql_q "SELECT count(*) FROM information_schema.tables WHERE table_schema='public' AND table_type='BASE TABLE' AND table_name NOT IN ('schema_migrations', 'spatial_ref_sys')"; }
MIGRATIONS=$(ls migrations/*.up.sql | wc -l | tr -d ' ')
TABLES=19

cleanup() { admin_q "DROP DATABASE IF EXISTS ${TEST_DB} WITH (FORCE)" >/dev/null 2>&1 || true; }
trap cleanup EXIT
cleanup
admin_q "CREATE DATABASE ${TEST_DB} WITH ENCODING 'UTF8' TEMPLATE template0 LC_COLLATE 'C' LC_CTYPE 'C'" >/dev/null

echo "==> UP"
$MIGRATE up
[ "$(count_tables)" = "$TABLES" ] || { echo "FAIL: expected $TABLES tables after up"; exit 1; }
[ "$(psql_q 'SELECT count(*) FROM roles')" = "5" ] || { echo "FAIL: roles not seeded"; exit 1; }

echo "==> DOWN one step at a time"
for _ in $(seq "$MIGRATIONS"); do $MIGRATE down 1; done
[ "$(count_tables)" = "0" ] || { echo "FAIL: tables remain after down"; exit 1; }
[ "$(psql_q "SELECT count(*) FROM pg_type WHERE typname='user_status'")" = "0" ] || { echo "FAIL: enum remains"; exit 1; }
[ "$(psql_q "SELECT count(*) FROM pg_proc WHERE proname='fn_set_updated_at'")" = "0" ] || { echo "FAIL: function remains"; exit 1; }
[ "$(psql_q "SELECT count(*) FROM pg_type t JOIN pg_namespace n ON n.oid = t.typnamespace WHERE n.nspname = 'public' AND t.typtype = 'e'")" = "0" ] || { echo "FAIL: enum types remain"; exit 1; }
[ "$(psql_q "SELECT count(*) FROM pg_extension WHERE extname IN ('postgis', 'pg_trgm')")" = "0" ] || { echo "FAIL: extensions remain"; exit 1; }

echo "==> UP again (idempotent round trip)"
$MIGRATE up
[ "$(count_tables)" = "$TABLES" ] || { echo "FAIL: expected $TABLES tables after second up"; exit 1; }

echo "==> constraint smoke test"
if psql_q "INSERT INTO users(phone) VALUES ('abc')" 2>/dev/null; then echo "FAIL: bad phone accepted"; exit 1; fi
psql_q "INSERT INTO users(phone) VALUES ('09121234567')" >/dev/null
if psql_q "INSERT INTO users(phone) VALUES ('09121234567')" 2>/dev/null; then echo "FAIL: duplicate phone accepted"; exit 1; fi

echo "ALL MIGRATION CHECKS PASSED"

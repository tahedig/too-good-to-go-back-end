#!/usr/bin/env bash
# Verifies: up -> tables exist -> down 1 step by step -> everything gone -> up again.
# Requires: docker, golang-migrate CLI (migrate)
set -euo pipefail

NAME=tgtg-migrate-test
PORT=55432
DB_URL="postgres://postgres:test@localhost:${PORT}/tgtg?sslmode=disable"
MIGRATE="migrate -path migrations -database ${DB_URL}"

cleanup() { docker rm -f "$NAME" >/dev/null 2>&1 || true; }
trap cleanup EXIT
cleanup

docker run -d --name "$NAME" -e POSTGRES_PASSWORD=test -e POSTGRES_DB=tgtg \
    -p ${PORT}:5432 postgres:16 >/dev/null
until docker exec "$NAME" pg_isready -U postgres -d tgtg >/dev/null 2>&1; do sleep 1; done
sleep 2

psql_q() { docker exec "$NAME" psql -U postgres -d tgtg -tAc "$1"; }
count_tables() { psql_q "SELECT count(*) FROM information_schema.tables WHERE table_schema='public' AND table_name <> 'schema_migrations'"; }

echo "==> UP"
$MIGRATE up
[ "$(count_tables)" = "4" ] || { echo "FAIL: expected 4 tables after up"; exit 1; }
[ "$(psql_q 'SELECT count(*) FROM roles')" = "5" ] || { echo "FAIL: roles not seeded"; exit 1; }

echo "==> DOWN one step at a time"
for i in 1 2 3 4; do $MIGRATE down 1; done
[ "$(count_tables)" = "0" ] || { echo "FAIL: tables remain after down"; exit 1; }
[ "$(psql_q "SELECT count(*) FROM pg_type WHERE typname='user_status'")" = "0" ] || { echo "FAIL: enum remains"; exit 1; }
[ "$(psql_q "SELECT count(*) FROM pg_proc WHERE proname='fn_set_updated_at'")" = "0" ] || { echo "FAIL: function remains"; exit 1; }

echo "==> UP again (idempotent round trip)"
$MIGRATE up
[ "$(count_tables)" = "4" ] || { echo "FAIL: expected 4 tables after second up"; exit 1; }

echo "==> constraint smoke test"
if psql_q "INSERT INTO users(phone) VALUES ('abc')" 2>/dev/null; then echo "FAIL: bad phone accepted"; exit 1; fi
psql_q "INSERT INTO users(phone) VALUES ('09121234567')" >/dev/null
if psql_q "INSERT INTO users(phone) VALUES ('09121234567')" 2>/dev/null; then echo "FAIL: duplicate phone accepted"; exit 1; fi

echo "ALL MIGRATION CHECKS PASSED"

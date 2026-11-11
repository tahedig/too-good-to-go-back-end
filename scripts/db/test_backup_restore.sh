#!/usr/bin/env bash
# End-to-end test of backup.sh / restore.sh on a throw-away local PostgreSQL cluster (no Docker needed).
# Must NOT run as root (postgres refuses). Requires PostgreSQL server binaries (initdb, pg_ctl).
#   bash scripts/db/test_backup_restore.sh
set -Eeuo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "$HERE/../.." && pwd)"
(( EUID != 0 )) || { echo "run as a non-root user"; exit 1; }
PGBIN="$(dirname "$(ls /usr/lib/postgresql/*/bin/initdb 2>/dev/null | sort -V | tail -n1 || command -v initdb)")"
export PATH="$PGBIN:$PATH"

T="$(mktemp -d)"; PORT=55433; SOCK="$T/sock"; mkdir -p "$SOCK"
trap 'pg_ctl -D "$T/data" -m immediate stop >/dev/null 2>&1 || true; rm -rf "$T"' EXIT
initdb -D "$T/data" -U postgres --auth=trust -E UTF8 --locale=C >/dev/null
pg_ctl -D "$T/data" -o "-p $PORT -k $SOCK -c listen_addresses=''" -l "$T/pg.log" -w start >/dev/null
export PGHOST="$SOCK" PGPORT=$PORT
su_psql() { psql -X -qAt -v ON_ERROR_STOP=1 -U postgres "$@"; }

# roles: read-only backup role, restore role that can create databases but owns no active data
su_psql -d postgres -c "CREATE ROLE tgtg_backup LOGIN; CREATE ROLE tgtg_restore LOGIN CREATEDB;"
su_psql -d postgres -c "CREATE DATABASE tgtg_staging OWNER postgres"
for f in "$ROOT"/migrations/*.up.sql; do su_psql -d tgtg_staging -f "$f" >/dev/null; done
su_psql -d tgtg_staging -c "GRANT pg_read_all_data TO tgtg_backup"

# representative data, including Persian text and special characters
su_psql -d tgtg_staging <<'SQL'
INSERT INTO users(phone, full_name, status, no_show_count, penalty_until) VALUES
 ('+989121234567','مبینا احمدی','active',0,NULL),
 ('+989351112233','شرکت "نان تازه"','active',2, now()),
 ('+989001234567','O''Brien','suspended',1,NULL),
 ('+989127654321',NULL,'deleted',0,NULL);
INSERT INTO user_roles(user_id, role_id, granted_by) SELECT id, 1, NULL FROM users;
INSERT INTO user_roles(user_id, role_id, granted_by) VALUES (1,5,NULL),(2,2,1);
INSERT INTO otp_codes(phone, code_hash, expires_at) SELECT phone, md5(phone), now()+interval '5 min' FROM users WHERE phone IS NOT NULL;
SQL

CFG="$T/db-backup.env"
cat > "$CFG" <<CFGEOF
PGHOST=$SOCK
PGPORT=$PORT
PGDATABASE=tgtg_staging
PGUSER=tgtg_backup
RESTORE_PGUSER=tgtg_restore
BACKUP_DIR=$T/backups
RETENTION_DAYS=14
RETENTION_MIN_KEEP=2
PROTECTED_DBS="tgtg_staging tgtg_production"
CFGEOF
chmod 600 "$CFG"; export DB_BACKUP_ENV="$CFG"
pass() { echo "PASS: $*"; }; fail() { echo "FAIL: $*"; exit 1; }

echo "== 1. backup"
bash "$HERE/backup.sh"
B="$(ls "$T"/backups/tgtg_*.dump | head -n1)"
[[ "$(stat -c %a "$T/backups")" == 700 && "$(stat -c %a "$B")" == 600 ]] && pass "storage permissions 0700/0600" || fail "permissions"
grep -q $'^T\tusers\t4\t' "$B.manifest" && pass "manifest has users=4" || fail "manifest"

echo "== 2. restore into separate DB and verify"
bash "$HERE/restore.sh" --keep --target tgtg_restore_test
[[ "$(su_psql -d tgtg_restore_test -c "SELECT count(*) FROM users WHERE full_name LIKE '%نان تازه%'")" == 1 ]] && pass "Persian data intact" || fail "unicode"
[[ "$(su_psql -d tgtg_staging -c "SELECT count(*) FROM users")" == 4 ]] && pass "active DB untouched" || fail "active DB changed"
su_psql -d tgtg_restore_test -c "INSERT INTO users(phone) VALUES ('abc')" 2>/dev/null && fail "constraint missing after restore" || pass "constraints enforced after restore"

echo "== 3. active DB is never overwritten"
bash "$HERE/restore.sh" --target tgtg_staging 2>/dev/null && fail "restored over active DB" || pass "refused target tgtg_staging"
bash "$HERE/restore.sh" --target tgtg_production 2>/dev/null && fail "restored over protected DB" || pass "refused non-restore name"
bash "$HERE/restore.sh" --target tgtg_restore_test 2>/dev/null && fail "overwrote existing restore DB silently" || pass "refused existing DB without --drop-existing"

echo "== 4. corruption is detected"
cp "$B" "$B.bak"; printf 'x' | dd of="$B" bs=1 seek=100 conv=notrunc 2>/dev/null
bash "$HERE/restore.sh" --backup "$B" --target tgtg_restore_bad 2>/dev/null && fail "corrupt backup accepted" || pass "corrupt backup rejected"
mv "$B.bak" "$B"

echo "== 5. data drift is detected by the manifest"
su_psql -d tgtg_staging -c "UPDATE users SET full_name='changed' WHERE id=1" >/dev/null
bash "$HERE/backup.sh" >/dev/null 2>&1; sleep 1
sed -i 's/^\(T\tusers\t4\t\).*/\1deadbeef/' "$(ls "$T"/backups/tgtg_*.dump | tail -n1).manifest"
bash "$HERE/restore.sh" --drop-existing --target tgtg_restore_test 2>/dev/null && fail "manifest mismatch not caught" || pass "checksum mismatch caught"

echo "== 6. retention"
touch -d '30 days ago' "$T"/backups/tgtg_tgtg_staging_20200101T000000Z.dump
for e in .sha256 .manifest; do touch "$T/backups/tgtg_tgtg_staging_20200101T000000Z.dump$e"; done
bash "$HERE/backup.sh" >/dev/null 2>&1
[[ ! -e "$T/backups/tgtg_tgtg_staging_20200101T000000Z.dump" ]] && pass "old backup pruned" || fail "retention"
[[ "$(ls "$T"/backups/tgtg_*.dump | wc -l)" -ge 2 ]] && pass "minimum kept" || fail "min keep"

echo "== 7. loose config permissions rejected"
chmod 644 "$CFG"; bash "$HERE/backup.sh" 2>/dev/null && fail "loose config accepted" || pass "world-readable config rejected"
echo "ALL BACKUP/RESTORE CHECKS PASSED"

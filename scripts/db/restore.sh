#!/usr/bin/env bash
# Restore a staging backup into a SEPARATE database and verify it against the backup manifest.
#   restore.sh [--backup FILE] [--target DBNAME] [--drop-existing] [--keep]
# Safety: never touches the active database. The target must start with "tgtg_restore_", must differ
# from PGDATABASE and every name in PROTECTED_DBS. --drop-existing only drops an old *restore* DB.
set -Eeuo pipefail
SCRIPT_NAME=restore
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib.sh
source "$HERE/lib.sh"
require_cmd pg_restore psql createdb dropdb sha256sum
load_config
prepare_backup_dir

BACKUP=""; TARGET=""; DROP_EXISTING=0; KEEP=0
while (( $# )); do
  case "$1" in
    --backup) BACKUP="${2:?}"; shift 2;;
    --target) TARGET="${2:?}"; shift 2;;
    --drop-existing) DROP_EXISTING=1; shift;;
    --keep) KEEP=1; shift;;
    *) die "unknown argument: $1";;
  esac
done
[[ -n "$BACKUP" ]] || BACKUP="$(latest_backup)"
[[ -n "$BACKUP" && -f "$BACKUP" ]] || die "no backup found in $BACKUP_DIR"
TARGET="${TARGET:-tgtg_restore_$(date -u +%Y%m%d_%H%M%S)}"

# --- guard rails: the active database can never be the target ---------------------------------------
[[ "$TARGET" =~ ^tgtg_restore_[a-z0-9_]{1,40}$ ]] || die "target must match ^tgtg_restore_[a-z0-9_]+\$ (got '$TARGET')"
[[ "$TARGET" != "$PGDATABASE" ]] || die "refusing: target equals the active database"
for p in $PROTECTED_DBS; do [[ "$TARGET" != "$p" ]] || die "refusing: $TARGET is a protected database"; done

# --- integrity of the file itself ---------------------------------------------------------------------
( cd "$(dirname "$BACKUP")" && sha256sum --check --status "$(basename "$BACKUP").sha256" ) || die "checksum mismatch for $BACKUP"
MANIFEST="$BACKUP.manifest"; [[ -f "$MANIFEST" ]] || die "manifest missing: $MANIFEST"
log "backup $(basename "$BACKUP") passed sha256 check"

# --- create the target (restore role must have CREATEDB) ---------------------------------------------
EXISTS="$(psql_dst -d postgres -c "SELECT 1 FROM pg_database WHERE datname='$TARGET'")"
if [[ -n "$EXISTS" ]]; then
  (( DROP_EXISTING )) || die "database $TARGET already exists (use --drop-existing to replace an old restore copy)"
  psql_dst -d postgres -c "DROP DATABASE \"$TARGET\" WITH (FORCE)"
fi
psql_dst -d postgres -c "CREATE DATABASE \"$TARGET\" TEMPLATE template0 ENCODING 'UTF8'"
cleanup() { if (( KEEP == 0 )) && [[ "${OK:-0}" == 1 ]]; then PGHOST="$RESTORE_PGHOST" PGPORT="$RESTORE_PGPORT" PGUSER="$RESTORE_PGUSER" dropdb --if-exists "$TARGET" || true; log "dropped $TARGET"; fi; }
trap cleanup EXIT

export PGHOST="$RESTORE_PGHOST" PGPORT="$RESTORE_PGPORT" PGUSER="$RESTORE_PGUSER"
log "restoring into $TARGET@$PGHOST (active DB untouched)"
if [[ "$BACKUP" == *.gpg ]]; then
  require_cmd gpg
  gpg --batch --decrypt "$BACKUP" | pg_restore --no-owner --no-privileges --exit-on-error --dbname="$TARGET"
else
  pg_restore --no-owner --no-privileges --exit-on-error --dbname="$TARGET" "$BACKUP"
fi

# --- verify against the manifest (same checks the backup computed from its snapshot) -------------------
export PGOPTIONS="-c timezone=UTC"
FAIL=0
while IFS=$'\t' read -r kind a b c; do
  case "$kind" in
    T)
      if [[ "$c" == "-" ]]; then q="SELECT count(*) || E'\t-' FROM public.\"$a\""
      else q="SELECT count(*) || E'\t' || md5(coalesce(string_agg(x::text, E'\n' ORDER BY x::text),'')) FROM public.\"$a\" x"; fi
      got="$(psql_dst -d "$TARGET" -c "$q")"
      if [[ "$got" == "$b"$'\t'"$c" ]]; then log "OK    table $a rows=$b${c:+ checksum=${c:0:8}}"; else log "FAIL  table $a expected '$b $c' got '${got/$'\t'/ }'"; FAIL=1; fi;;
    M)
      case "$a" in
        indexes)     q="SELECT count(*) FROM pg_indexes WHERE schemaname='public'";;
        triggers)    q="SELECT count(*) FROM pg_trigger t JOIN pg_class c ON c.oid=t.tgrelid JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='public' AND NOT t.tgisinternal";;
        constraints) q="SELECT count(*) FROM pg_constraint k JOIN pg_namespace n ON n.oid=k.connamespace WHERE n.nspname='public'";;
        functions)   q="SELECT count(*) FROM pg_proc p JOIN pg_namespace n ON n.oid=p.pronamespace WHERE n.nspname='public' AND NOT EXISTS (SELECT 1 FROM pg_depend d WHERE d.objid=p.oid AND d.deptype='e')";;
        enum_types)  q="SELECT count(*) FROM pg_type WHERE typtype='e' AND typnamespace='public'::regnamespace";;
        views)       q="SELECT count(*) FROM pg_views WHERE schemaname='public'";;
        *) continue;;
      esac
      got="$(psql_dst -d "$TARGET" -c "$q")"
      if [[ "$got" == "$b" ]]; then log "OK    $a=$b"; else log "FAIL  $a expected $b got $got"; FAIL=1; fi;;
  esac
done < "$MANIFEST"
(( FAIL == 0 )) || die "verification FAILED; $TARGET kept for inspection"
OK=1
log "RESTORE VERIFIED: $(basename "$BACKUP") -> $TARGET$( (( KEEP )) && echo ' (kept)' || echo ' (will be dropped)')"

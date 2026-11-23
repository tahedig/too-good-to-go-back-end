#!/usr/bin/env bash
# Staging PostgreSQL backup: consistent pg_dump (custom format) + manifest + checksum + retention.
#   * credentials: libpq password file only (PGPASSFILE), dedicated read-only role
#   * storage    : 0700 directory, 0600 files, optional GPG encryption at rest
#   * consistent : dump, row counts and checksums are all taken from ONE exported snapshot
#   * retention  : delete backups older than RETENTION_DAYS but always keep RETENTION_MIN_KEEP newest
set -Eeuo pipefail
SCRIPT_NAME=backup
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib.sh
source "$HERE/lib.sh"
require_cmd pg_dump pg_restore psql sha256sum flock find
load_config
prepare_backup_dir

exec 9>"$BACKUP_DIR/.backup.lock"
flock -n 9 || die "another backup is already running"

TS="$(date -u +%Y%m%dT%H%M%SZ)"
BASE="tgtg_${PGDATABASE}_${TS}"
PARTIAL="$BACKUP_DIR/.${BASE}.dump.partial"
MANIFEST_TMP="$BACKUP_DIR/.${BASE}.manifest.partial"
GPG_PARTIAL="$BACKUP_DIR/.${BASE}.dump.gpg.partial"
SHA_TMP="$BACKUP_DIR/.${BASE}.sha256.partial"
cleanup() { rm -f "$PARTIAL" "$MANIFEST_TMP" "$GPG_PARTIAL" "$SHA_TMP"; }
trap cleanup EXIT

log "starting backup of ${PGDATABASE}@${PGHOST}:${PGPORT} as ${PGUSER}"

# --- one long-lived read-only REPEATABLE READ session whose snapshot pg_dump will reuse -------------
coproc SNAP { psql -X -qAt -v ON_ERROR_STOP=1 -d "$PGDATABASE"; }
sql() { printf '%s\n' "$1" >&"${SNAP[1]}"; }
# Both readers run in the PARENT shell (never inside <(...) or $(...)): a failure must call die in the
# main process, otherwise a dead session would silently yield a partial result.
read_until() { # append session output up to (not including) the sentinel line to stdout (caller redirects)
  local line
  while IFS= read -r -t 600 line <&"${SNAP[0]}"; do
    [[ "$line" == "$1" ]] && return 0
    printf '%s\n' "$line"
  done
  die "database session closed or timed out before '$1' (check credentials / connectivity)"
}
read_block() { # read_block ARRAY_NAME : fill the array with the session output up to __END__
  local -n _out="$1"; local line
  _out=()
  while IFS= read -r -t 600 line <&"${SNAP[0]}"; do
    [[ "$line" == "__END__" ]] && return 0
    _out+=("$line")
  done
  die "database session closed or timed out while reading the table list (check credentials / connectivity)"
}
sql "BEGIN ISOLATION LEVEL REPEATABLE READ READ ONLY;"
sql "SELECT '__SNAP__' || pg_export_snapshot();"
SNAPSHOT=""
while IFS= read -r -t 60 line <&"${SNAP[0]}"; do
  if [[ "$line" == __SNAP__* ]]; then SNAPSHOT="${line#__SNAP__}"; break; fi
done
[[ -n "$SNAPSHOT" ]] || die "could not export a snapshot (check credentials / connectivity)"
[[ "${BACKUP_TEST_KILL_SESSION:-}" == 1 ]] && { kill "$SNAP_PID" 2>/dev/null || true; sleep 0.3; }   # test hook only

# --- manifest from the same snapshot: per-table row counts + checksums, and schema object counts ----
sql "SELECT c.relname FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace
      WHERE n.nspname='public' AND c.relkind IN ('r','p') AND NOT c.relispartition
        AND NOT EXISTS (SELECT 1 FROM pg_depend d WHERE d.objid=c.oid AND d.deptype='e')
      ORDER BY 1;"
sql "SELECT '__END__';"
read_block TABLES
[[ ${#TABLES[@]} -gt 0 ]] || die "no tables found in public schema; refusing to produce an empty backup"

: > "$MANIFEST_TMP"
for t in "${TABLES[@]}"; do
  if [[ " $CHECKSUM_TABLES " == *" $t "* ]]; then
    sql "SELECT 'T' || E'\t' || '$t' || E'\t' || count(*) || E'\t' || md5(coalesce(string_agg(x::text, E'\n' ORDER BY x::text),'')) FROM public.\"$t\" x;"
  else
    sql "SELECT 'T' || E'\t' || '$t' || E'\t' || count(*) || E'\t' || '-' FROM public.\"$t\";"
  fi
  sql "SELECT '__END__';"
  read_until __END__ >> "$MANIFEST_TMP"
done
sql "SELECT 'M' || E'\t' || m || E'\t' || v FROM (VALUES
  ('indexes',     (SELECT count(*) FROM pg_indexes WHERE schemaname='public')),
  ('triggers',    (SELECT count(*) FROM pg_trigger t JOIN pg_class c ON c.oid=t.tgrelid JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='public' AND NOT t.tgisinternal)),
  ('constraints', (SELECT count(*) FROM pg_constraint k JOIN pg_namespace n ON n.oid=k.connamespace WHERE n.nspname='public')),
  ('functions',   (SELECT count(*) FROM pg_proc p JOIN pg_namespace n ON n.oid=p.pronamespace WHERE n.nspname='public' AND NOT EXISTS (SELECT 1 FROM pg_depend d WHERE d.objid=p.oid AND d.deptype='e'))),
  ('enum_types',  (SELECT count(*) FROM pg_type WHERE typtype='e' AND typnamespace='public'::regnamespace)),
  ('views',       (SELECT count(*) FROM pg_views WHERE schemaname='public'))
) s(m,v);"
sql "SELECT '__END__';"
read_until __END__ >> "$MANIFEST_TMP"
printf 'S\tsource_db\t%s\nS\ttaken_at_utc\t%s\n' "$PGDATABASE" "$TS" >> "$MANIFEST_TMP"

# --- the dump itself, pinned to that snapshot --------------------------------------------------------
log "dumping ${#TABLES[@]} tables (snapshot $SNAPSHOT)"
pg_dump --format=custom --compress=6 --snapshot="$SNAPSHOT" --file="$PARTIAL" "$PGDATABASE"
sql "COMMIT;"
sql '\q'
wait "$SNAP_PID" 2>/dev/null || true

# --- validate the archive is readable, optionally encrypt, checksum, atomically publish --------------
pg_restore --list "$PARTIAL" >/dev/null || die "pg_restore cannot read the new archive; keeping nothing"
FINAL="$BACKUP_DIR/${BASE}.dump"
if [[ -n "$BACKUP_GPG_RECIPIENT" ]]; then
  require_cmd gpg
  gpg --batch --yes --trust-model always --encrypt --recipient "$BACKUP_GPG_RECIPIENT" \
      --output "$GPG_PARTIAL" "$PARTIAL"
  rm -f "$PARTIAL"
  PARTIAL="$GPG_PARTIAL"
  FINAL="${FINAL}.gpg"
fi
# Commit protocol. A backup EXISTS only when its dump has its final name, and discovery (restore, retention)
# looks only at *.dump / *.dump.gpg. So: finish and fsync everything else first, rename the sidecars into place
# (invisible to discovery), and rename the dump LAST, as the single atomic commit point. A crash at any step
# leaves either no new backup or a complete one, never a dump without its checksum and manifest.
FINAL_NAME="$(basename "$FINAL")"
printf '%s  %s\n' "$(sha256sum < "$PARTIAL" | cut -d' ' -f1)" "$FINAL_NAME" > "$SHA_TMP"
chmod 600 "$PARTIAL" "$SHA_TMP" "$MANIFEST_TMP"
sync -- "$PARTIAL" "$SHA_TMP" "$MANIFEST_TMP" 2>/dev/null || sync
mv -f "$SHA_TMP"      "${FINAL}.sha256"
mv -f "$MANIFEST_TMP" "${FINAL}.manifest"
[[ "${BACKUP_TEST_CRASH_BEFORE_COMMIT:-}" == 1 ]] && exit 99                    # test hook only
mv -f "$PARTIAL" "$FINAL"                                                       # <-- commit point
sync -- "$BACKUP_DIR" 2>/dev/null || true
date -u +%Y-%m-%dT%H:%M:%SZ > "$BACKUP_DIR/LAST_BACKUP_SUCCESS"
log "backup OK: $(basename "$FINAL") ($(du -h "$FINAL" | cut -f1))"

# --- retention: drop backups older than RETENTION_DAYS, but never go below RETENTION_MIN_KEEP -------
# Only COMPLETE backups (dump + .sha256 + .manifest) are counted, so a damaged one can never push a good one out.
# Sidecars whose dump does not exist (crash before the commit point) are removed; we hold the backup lock.
for side in "$BACKUP_DIR"/tgtg_*.dump.sha256 "$BACKUP_DIR"/tgtg_*.dump.manifest "$BACKUP_DIR"/tgtg_*.dump.gpg.sha256 "$BACKUP_DIR"/tgtg_*.dump.gpg.manifest; do
  [[ -e "$side" ]] || continue
  base="${side%.sha256}"; base="${base%.manifest}"
  [[ -f "$base" ]] || { rm -f -- "$side"; log "cleanup: removed orphan $(basename "$side") (no dump)"; }
done
mapfile -t ALL < <(list_complete_backups)
for d in "$BACKUP_DIR"/tgtg_*.dump "$BACKUP_DIR"/tgtg_*.dump.gpg; do
  [[ -f "$d" ]] || continue
  if [[ ! -f "$d.sha256" || ! -f "$d.manifest" ]]; then
    log "WARNING: incomplete backup ignored (missing checksum/manifest): $(basename "$d")"
  fi
done
total=${#ALL[@]}
deleted=0
for f in "${ALL[@]}"; do   # oldest first
  if (( total - deleted <= RETENTION_MIN_KEEP )); then break; fi
  if [[ -n "$(find "$f" -mtime +"$RETENTION_DAYS" -print)" ]]; then
    rm -f -- "$f.sha256" "$f.manifest" "$f"        # dump last, so a crash here leaves only invisible sidecars
    deleted=$((deleted + 1))
    log "retention: removed $(basename "$f")"
  fi
done
find "$BACKUP_DIR" -maxdepth 1 -name '.*.partial' -mtime +1 -delete   # leftovers from crashed runs
log "retention: kept $((total - deleted)) backup(s) (policy: ${RETENTION_DAYS}d, min ${RETENTION_MIN_KEEP})"

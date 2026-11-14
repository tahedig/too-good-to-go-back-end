#!/usr/bin/env bash
# Shared helpers for the TGTG database backup / restore scripts. Source this file; do not run it.
# shellcheck shell=bash

umask 077

log() { printf '%s [%s] %s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "${SCRIPT_NAME:-db}" "$*" >&2; }
die() { log "ERROR: $*"; exit 1; }

require_cmd() { for c in "$@"; do command -v "$c" >/dev/null 2>&1 || die "required command not found: $c"; done; }

# True if a path is accessible by group/others (we want 0600 files / 0700 dirs).
loose_perms() { local m; m=$(stat -c '%a' "$1"); [[ "${m: -2}" != "00" ]]; }

# load_config: reads KEY=VALUE settings from $DB_BACKUP_ENV (default /etc/tgtg/db-backup.env).
# Secrets are NEVER read from this file or from the command line: passwords live only in a
# libpq password file (PGPASSFILE, mode 0600), which libpq itself refuses to use if too open.
load_config() {
  local cfg="${DB_BACKUP_ENV:-/etc/tgtg/db-backup.env}"
  if [[ -f "$cfg" ]]; then
    if loose_perms "$cfg"; then die "$cfg must not be readable by group/others (chmod 600 $cfg)"; fi
    set -a
    # shellcheck disable=SC1090
    source "$cfg"
    set +a
  fi
  : "${PGHOST:?PGHOST is required}" "${PGDATABASE:?PGDATABASE (the ACTIVE staging DB) is required}"
  : "${BACKUP_DIR:?BACKUP_DIR is required}" "${PGUSER:?PGUSER (backup role) is required}"
  export PGHOST PGDATABASE PGUSER
  export PGPORT="${PGPORT:-5432}"
  export PGCONNECT_TIMEOUT="${PGCONNECT_TIMEOUT:-10}"
  # UTC keeps ::text output stable, so checksums computed at backup time match those at restore time.
  export PGOPTIONS="${PGOPTIONS:-} -c timezone=UTC -c lock_timeout=30s"
  if [[ -n "${PGPASSFILE:-}" ]]; then export PGPASSFILE; fi
  unset PGPASSWORD   # never accept a password through the environment
  RETENTION_DAYS="${RETENTION_DAYS:-14}"
  RETENTION_MIN_KEEP="${RETENTION_MIN_KEEP:-3}"
  CHECKSUM_TABLES="${CHECKSUM_TABLES:-roles users user_roles otp_codes}"
  BACKUP_GPG_RECIPIENT="${BACKUP_GPG_RECIPIENT:-}"
  RESTORE_PGHOST="${RESTORE_PGHOST:-$PGHOST}"
  RESTORE_PGPORT="${RESTORE_PGPORT:-$PGPORT}"
  RESTORE_PGUSER="${RESTORE_PGUSER:-}"
  PROTECTED_DBS="${PROTECTED_DBS:-}"
}

# Prepare the restricted backup directory (0700, owner only) and refuse to use a loose one.
prepare_backup_dir() {
  mkdir -p "$BACKUP_DIR"
  chmod 700 "$BACKUP_DIR"
  [[ -O "$BACKUP_DIR" ]] || die "$BACKUP_DIR must be owned by the user running this script ($(id -un))"
  if loose_perms "$BACKUP_DIR"; then die "$BACKUP_DIR permissions are too open"; fi
  return 0
}

# COMPLETE backups only (dump or dump.gpg that also has its .sha256 and .manifest), oldest first.
# A dump without both sidecars is incomplete (e.g. an old crashed run) and is never restored or counted.
list_complete_backups() {
  local f
  while IFS= read -r f; do
    if [[ -f "$f.sha256" && -f "$f.manifest" ]]; then printf '%s\n' "$f"; fi
  done < <(find "$BACKUP_DIR" -maxdepth 1 -type f \( -name 'tgtg_*.dump' -o -name 'tgtg_*.dump.gpg' \) | sort)
  return 0   # never let the last (possibly skipped) item turn into a failure under set -e
}
latest_backup() { list_complete_backups | tail -n1; }

# psql against the source (backup role) and the restore target (restore role).
psql_src() { psql -X -qAt -v ON_ERROR_STOP=1 "$@"; }
psql_dst() {
  PGHOST="$RESTORE_PGHOST" PGPORT="$RESTORE_PGPORT" PGUSER="${RESTORE_PGUSER:?RESTORE_PGUSER is required for restore}" \
    psql -X -qAt -v ON_ERROR_STOP=1 "$@"
}

(( BASH_VERSINFO[0] > 4 || (BASH_VERSINFO[0] == 4 && BASH_VERSINFO[1] >= 3) )) || { echo "bash >= 4.3 required (namerefs)" >&2; exit 1; }

# Staging database backup & restore

Scripts live in `scripts/db/`. Everything is plain bash + the PostgreSQL client tools (`pg_dump`, `pg_restore`, `psql`), PostgreSQL 14+.

| Requirement | How it is met |
|---|---|
| Secure credentials | Dedicated **read-only** role `tgtg_backup`. Password only in a libpq password file (`PGPASSFILE`, mode 0600). `PGPASSWORD` is unset, passwords are never on the command line or in the config file, config must be mode 0600 or the scripts refuse to run. TLS via `PGSSLMODE=require`. |
| Restricted storage | `BACKUP_DIR` is created 0700 and must be owned by the running user; files are 0600 (`umask 077`). Optional GPG encryption at rest (`BACKUP_GPG_RECIPIENT`; only the public key is needed on the host). Systemd unit adds sandboxing. |
| Retention | Delete backups older than `RETENTION_DAYS` (14), but never go below `RETENTION_MIN_KEEP` (3), so a failing job cannot empty the folder. Partial files are written as `.partial` and published atomically. |
| Schedule | Daily 03:00 (`tgtg-db-backup.timer`) and a weekly restore drill (`tgtg-db-restore-verify.timer`). A cron alternative is included. |
| Recovery never overwrites the active DB | `restore.sh` only restores into a **new** database named `tgtg_restore_*`; it refuses the active DB, anything in `PROTECTED_DBS`, any other name, and an existing database unless `--drop-existing` (which still only accepts `tgtg_restore_*` names). There is no flag to restore over the active DB. |
| Verified recovery | Each backup has a `.manifest` taken from the **same snapshot** as the dump (row counts for all tables, md5 content checksums for `CHECKSUM_TABLES`, counts of indexes/triggers/constraints/functions/enums). `restore.sh` checks sha256 of the file first, restores, then compares every manifest line. Any mismatch exits non-zero and keeps the restored DB for inspection. |

## One-time setup (on the staging DB host or a jump host)

```sql
-- as a superuser on the staging server
CREATE ROLE tgtg_backup  LOGIN PASSWORD '...';  GRANT pg_read_all_data TO tgtg_backup;  -- PG14+
CREATE ROLE tgtg_restore LOGIN PASSWORD '...' CREATEDB;                                  -- on the restore server
```
```bash
sudo useradd --system --home /var/backups/tgtg tgtg-backup
sudo install -d -m 0700 -o tgtg-backup /var/backups/tgtg /etc/tgtg
# /etc/tgtg/.pgpass  (mode 0600):  host:port:db:user:password  one line per role
sudo install -m 0600 -o tgtg-backup deploy/db-backup/db-backup.env.example /etc/tgtg/db-backup.env   # then edit
sudo cp deploy/db-backup/*.service deploy/db-backup/*.timer /etc/systemd/system/
sudo systemctl enable --now tgtg-db-backup.timer tgtg-db-restore-verify.timer
```
Adjust `ExecStart` paths in the units to where the scripts are deployed. For off-host copies, sync `BACKUP_DIR` to a private bucket/server with write-only credentials (not included here).

## Crash safety (commit protocol)
A backup exists only when its dump has its final name. `backup.sh` therefore writes the checksum and manifest
first (hidden `.partial` files, fsync, then renamed into place) and renames the **dump last** as the single atomic
commit point. Restore and retention only look at `*.dump` / `*.dump.gpg` that also have both sidecars
(`list_complete_backups`), so a crash can leave orphan sidecars (removed by the next run) but never a visible dump
without its checksum and manifest. A database session that dies mid-run aborts the backup (all session reads happen
in the main shell, not in `<(...)`), so a partial table list can never become a manifest. Requires bash >= 4.3.

## Manual use

```bash
DB_BACKUP_ENV=/etc/tgtg/db-backup.env scripts/db/backup.sh                       # backup now
DB_BACKUP_ENV=/etc/tgtg/db-backup.env scripts/db/restore.sh --keep               # latest -> tgtg_restore_<ts>, verified, kept
DB_BACKUP_ENV=... scripts/db/restore.sh --backup /var/backups/tgtg/<file>.dump --target tgtg_restore_incident1 --keep
```
Without `--keep`, a successfully verified drill database is dropped automatically.

### If you really need to recover production-style (active DB)
Restore to `tgtg_restore_*` first, inspect it, then switch the app's `DB_URL` to it, or rename databases manually during a maintenance window (`ALTER DATABASE ... RENAME`). This is deliberately a human decision and not part of the scripts.

## Test (done criteria evidence)
`make db-backup-test` (`scripts/db/test_backup_restore.sh`) starts a throw-away PostgreSQL, applies `migrations/*.up.sql`, loads representative data (users incl. Persian names and quotes, roles, user_roles, otp_codes), then asserts: backup perms 0700/0600; restore verified; Persian data intact; constraints enforced; active DB untouched; restore onto active/protected/non-restore names refused; corrupt file rejected; manifest drift caught; retention prunes old files but keeps the minimum; loose config rejected. Last run: **all checks passed** on PostgreSQL 16.

## Notes / limits
- `CHECKSUM_TABLES` defaults to the 4 current tables; add new tables as migrations land (row counts are checked for every table automatically).
- Redis (OTP/rate-limit data) is not covered; it is ephemeral by design.
- Monitor `LAST_BACKUP_SUCCESS` in `BACKUP_DIR` or the systemd unit result and alert if older than 26h.
- Restore drills read the backup as `tgtg-backup`; if the restore server differs, set `RESTORE_PGHOST` and add its entry to `.pgpass`.

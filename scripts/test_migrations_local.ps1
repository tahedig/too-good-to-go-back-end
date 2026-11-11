# Windows / PowerShell version of the migration round-trip test (no Docker, no make, no bash).
# Requires: psql and migrate on PATH, a DB user allowed to CREATE DATABASE.
# Run from the repo root:
#   powershell -ExecutionPolicy Bypass -File scripts\test_migrations_local.ps1
#   powershell -ExecutionPolicy Bypass -File scripts\test_migrations_local.ps1 -AdminUrl "postgres://USER:PASS@localhost:5432/postgres?sslmode=disable"
param(
    [string]$AdminUrl = "postgres://app:app@localhost:5432/postgres?sslmode=disable"
)
$ErrorActionPreference = "Continue"

$TestDb  = "tgtg_migrate_test"
$Base    = $AdminUrl.Substring(0, $AdminUrl.LastIndexOf("/"))
$TestUrl = "$Base/${TestDb}?sslmode=disable"

function Fail($msg) { Write-Host "FAIL: $msg" -ForegroundColor Red; exit 1 }

function AdminQ([string]$sql) {
    $r = & psql $AdminUrl -v ON_ERROR_STOP=1 -tAc $sql
    if ($LASTEXITCODE -ne 0) { Fail "psql (admin) failed: $sql" }
    return ("$r").Trim()
}
function Q([string]$sql) {
    $r = & psql $TestUrl -v ON_ERROR_STOP=1 -tAc $sql
    if ($LASTEXITCODE -ne 0) { Fail "psql failed: $sql" }
    return ("$r").Trim()
}
function QWorks([string]$sql) {            # true if the statement succeeds
    & psql $TestUrl -v ON_ERROR_STOP=1 -tAc $sql 2>$null | Out-Null
    return ($LASTEXITCODE -eq 0)
}
function RunMigrate {
    & migrate -path migrations -database $TestUrl @args
    if ($LASTEXITCODE -ne 0) { Fail "migrate $args" }
}
function CountTables {
    Q "SELECT count(*) FROM information_schema.tables WHERE table_schema='public' AND table_name <> 'schema_migrations'"
}
function Cleanup {
    & psql $AdminUrl -tAc "DROP DATABASE IF EXISTS $TestDb WITH (FORCE)" 2>$null | Out-Null
}

try {
    Cleanup
    AdminQ "CREATE DATABASE $TestDb WITH ENCODING 'UTF8' TEMPLATE template0 LC_COLLATE 'C' LC_CTYPE 'C'" | Out-Null

    Write-Host "==> UP"
    RunMigrate up
    if ((CountTables) -ne "4") { Fail "expected 4 tables after up" }
    if ((Q "SELECT count(*) FROM roles") -ne "5") { Fail "roles not seeded" }

    Write-Host "==> DOWN one step at a time"
    1..4 | ForEach-Object { RunMigrate down 1 }
    if ((CountTables) -ne "0") { Fail "tables remain after down" }
    if ((Q "SELECT count(*) FROM pg_type WHERE typname='user_status'") -ne "0") { Fail "enum remains" }
    if ((Q "SELECT count(*) FROM pg_proc WHERE proname='fn_set_updated_at'") -ne "0") { Fail "function remains" }

    Write-Host "==> UP again (idempotent round trip)"
    RunMigrate up
    if ((CountTables) -ne "4") { Fail "expected 4 tables after second up" }

    Write-Host "==> constraint smoke test"
    if (QWorks "INSERT INTO users(phone) VALUES ('abc')") { Fail "bad phone accepted" }
    if (-not (QWorks "INSERT INTO users(phone) VALUES ('09121234567')")) { Fail "valid phone rejected" }
    if (QWorks "INSERT INTO users(phone) VALUES ('09121234567')") { Fail "duplicate phone accepted" }

    Write-Host "ALL MIGRATION CHECKS PASSED" -ForegroundColor Green
}
finally {
    Cleanup
}
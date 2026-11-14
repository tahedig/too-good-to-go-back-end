#!/usr/bin/env python3
"""Applies the 5 small edits to EXISTING files that hook the catalogue feature in.
Idempotent (safe to re-run), keeps each file's line endings (CRLF/LF), and stops without touching
anything if an anchor is not found. Run from the repo root:  python3 scripts/catalogue/apply_wiring.py
Afterwards:  gofmt -w internal && go mod tidy && go generate ./internal/injector
"""
import pathlib, re, sys

ROOT = pathlib.Path(__file__).resolve().parents[2]
edits = []  # (path, description, already_applied_marker, regex, replacement)

def edit(path, desc, marker, pattern, repl, flags=0):
    edits.append((ROOT / path, desc, marker, re.compile(pattern, flags), repl))

# ---- router.go
edit("internal/presentation/router/router.go", "Handlers struct fields", "Catalogue ",
     r"(\tPublicStore\s+handlerinterface\.PublicStoreHandler)",
     r"\1\n\tCatalogue    handlerinterface.CatalogueHandler\n\tUserAuth     middlewareinterface.UserAuth")
edit("internal/presentation/router/router.go", "register catalogue routes", "registerCatalogue(",
     r"(\n\treturn r, nil\n)", r"\n\tregisterCatalogue(r, h)\n\1")

# ---- wire.go
edit("internal/injector/wire.go", "wire providers", "provideDatabase",
     r"(\t\tprovideAuthService,\n)",
     r"\1\t\tprovideDatabase,\n\t\tprovideCatalogueService,\n\t\tprovideCatalogueHandler,\n\t\tprovideUserAuth,\n")

# ---- injector.go
edit("internal/injector/injector.go", "middlewareinterface import", "middlewareinterface",
     r'(\thandlerinterface "[^"]+/presentation/handler/interface"\n)',
     r'\1\tmiddlewareinterface "github.com/tahedig/too-good-to-go/backend/internal/presentation/middleware/interface"\n')
edit("internal/injector/injector.go", "provideHandlers", "Catalogue: catalogue",
     r"func provideHandlers\(\n\thealth handlerinterface\.HealthHandler,\n\tauth handlerinterface\.AuthHandler,\n\) router\.Handlers \{\n\treturn router\.Handlers\{Health: health, Auth: auth\}\n\}",
     "func provideHandlers(\n\thealth handlerinterface.HealthHandler,\n\tauth handlerinterface.AuthHandler,\n"
     "\tcatalogue handlerinterface.CatalogueHandler,\n\tuserAuth middlewareinterface.UserAuth,\n) router.Handlers {\n"
     "\treturn router.Handlers{Health: health, Auth: auth, Catalogue: catalogue, UserAuth: userAuth}\n}")

# ---- env.go
edit("internal/bootstrap/env.go", "DatabaseURL field", "DatabaseURL string",
     r"(\tRedisKeyPrefix string\n)",
     r"\1\n\t// DatabaseURL (postgres://user:pass@host:5432/db?sslmode=disable) enables the catalogue endpoints\n\t// (home, offers, favorites). Empty = those endpoints are not registered.\n\tDatabaseURL string\n")
edit("internal/bootstrap/env.go", "DATABASE_URL parsing", 'p.string("DATABASE_URL"',
     r'(\t\tRedisKeyPrefix:\s+p\.string\("REDIS_KEY_PREFIX", "tgtg:"\),\n)',
     r'\1\t\tDatabaseURL:         p.string("DATABASE_URL", ""),\n')

def main():
    plan = {}
    for path, desc, marker, rx, repl in edits:
        if not path.exists():
            sys.exit(f"missing file: {path.relative_to(ROOT)}")
        raw = plan.get(path, path.read_bytes().decode("utf-8"))
        crlf = "\r\n" in raw
        text = raw.replace("\r\n", "\n")
        if marker in text:
            print(f"skip   {path.relative_to(ROOT)}: {desc} (already applied)")
            plan[path] = raw
            continue
        new, n = rx.subn(repl, text, count=1)
        if n != 1:
            sys.exit(f"ANCHOR NOT FOUND in {path.relative_to(ROOT)} for '{desc}'. Nothing was written; apply that edit by hand (see INTEGRATION.md).")
        print(f"edit   {path.relative_to(ROOT)}: {desc}")
        plan[path] = new.replace("\n", "\r\n") if crlf else new
    for path, content in plan.items():
        path.write_bytes(content.encode("utf-8"))
    # .env.example
    env = ROOT / ".env.example"
    if env.exists():
        t = env.read_bytes().decode("utf-8")
        if "DATABASE_URL" not in t:
            nl = "\r\n" if "\r\n" in t else "\n"
            t = t.rstrip() + nl + nl + "# PostgreSQL for the catalogue endpoints (home, offers, favorites). Empty = disabled." + nl + \
                "DATABASE_URL=" + nl
            env.write_bytes(t.encode("utf-8"))
            print("edit   .env.example: DATABASE_URL")
    print("done")

main()

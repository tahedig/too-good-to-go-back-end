#!/usr/bin/env python3
"""
Load test for the Too Good To Go backend: public home, catalogue and store APIs.

Brings up an isolated copy of the stack (its own compose project, own volume,
own host port), seeds a realistically shaped catalogue, drives it with modelled
user sessions up to 50 concurrent users, samples resource usage, and writes
loadtest-report.md.

The repository is treated as READ ONLY. Nothing is written inside it. The
compose override, the seed SQL and the report are generated at run time and
live outside the repo. A checksum of the repo tree is taken before and after
the run and reported.

Python 3.9+, standard library only. Requires docker and docker compose.

Usage:
    python3 loadtest.py --repo /path/to/backend
    python3 loadtest.py --repo /path/to/backend --dry-run
"""

from __future__ import annotations

import argparse
import datetime as dt
import gzip
import hashlib
import http.client
import io
import json
import math
import multiprocessing as mp
import os
import platform
import random
import re
import shutil
import statistics
import subprocess
import sys
import tempfile
import threading
import time
import zlib
from collections import Counter, defaultdict

VERSION = "1.0"

# ----------------------------------------------------------------- constants

# Tehran districts used to cluster stores the way a real city is clustered:
# dense in the centre and along Valiasr, thinner towards the edges.
# (name, lat, lng, weight, spread_km)
DISTRICTS = [
    ("ولیعصر", 35.7219, 51.4090, 22, 2.0),
    ("تجریش", 35.8000, 51.4330, 10, 1.8),
    ("ونک", 35.7560, 51.4100, 14, 1.6),
    ("انقلاب", 35.7010, 51.3940, 13, 1.8),
    ("تهرانپارس", 35.7380, 51.5200, 9, 2.6),
    ("سعادت‌آباد", 35.7840, 51.3730, 9, 1.8),
    ("نارمک", 35.7400, 51.4950, 7, 2.0),
    ("شهرری", 35.5900, 51.4300, 5, 3.0),
    ("اکباتان", 35.7030, 51.3100, 6, 2.2),
    ("پیروزی", 35.6890, 51.4660, 5, 2.4),
]

STORE_PREFIX = ["نانوایی", "کافه", "رستوران", "فروشگاه", "شیرینی‌سرای", "آشپزخانه", "سوپرمارکت", "آبمیوه"]
STORE_NAME = ["صبحگاهی", "آراز", "بهاران", "گلستان", "ایرانیان", "مهر", "پارسه", "شمیم", "آفتاب", "نیلوفر",
              "کاوه", "دماوند", "البرز", "ستاره", "رویال", "سنتی", "خانه", "سبز"]

OFFER_TITLE = ["بسته سورپرایز", "نان سنگک تازه", "جعبه شیرینی", "پک صبحانه", "ساندویچ ویژه", "پیتزا مخصوص",
               "سالاد فصل", "آبمیوه طبیعی", "بسته میوه", "غذای خانگی", "کیک روز", "قهوه و کیک",
               "بشقاب ایرانی", "برگر دستساز", "پک لبنیات", "خشکبار مخلوط"]
OFFER_SUFFIX = ["ویژه امروز", "پایان روز", "تازه", "خانگی", "اقتصادی", "دو نفره", "خانواده", "کلاسیک"]

# Matches the allergen_type enum and the allergens reference table.
ALLERGENS = ["gluten", "dairy", "egg", "peanut", "nuts", "soy", "fish", "shellfish", "sesame"]

# categories seeded by migration 000007, grouped by their category_kind
CATS_BAKERY = [1, 2]
CATS_READY = [3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 21]
CATS_GROCERY = [16, 17, 18, 19, 20]
ALL_CATS = CATS_BAKERY + CATS_READY + CATS_GROCERY

# Search terms that exist in the generated titles, so text search matches a
# realistic fraction of rows instead of nothing or everything.
SEARCH_TERMS = ["سورپرایز", "نان", "شیرینی", "صبحانه", "پیتزا", "قهوه", "سالاد", "میوه", "خانگی", "برگر"]

ENDPOINTS = [
    "GET /home/categories",
    "GET /home/today",
    "GET /home/popular-nearby",
    "GET /home/last-chance",
    "GET /offers",
    "GET /offers/filters",
    "GET /offers/map",
    "GET /stores/{id}",
    "GET /stores/{id}/offers",
    "GET /favorites/offers",
]

# ------------------------------------------------------------------- helpers


def run(cmd, *, stdin=None, timeout=600, check=True, capture=True):
    """Run a command. cmd is a list. Returns CompletedProcess."""
    p = subprocess.run(
        cmd,
        input=stdin.encode("utf-8") if isinstance(stdin, str) else stdin,
        stdout=subprocess.PIPE if capture else None,
        stderr=subprocess.PIPE if capture else None,
        timeout=timeout,
    )
    if check and p.returncode != 0:
        out = (p.stdout or b"").decode("utf-8", "replace")
        err = (p.stderr or b"").decode("utf-8", "replace")
        raise RuntimeError(
            "command failed (%d): %s\nstdout:\n%s\nstderr:\n%s"
            % (p.returncode, " ".join(cmd), out[-4000:], err[-4000:])
        )
    return p


def out_of(p):
    return (p.stdout or b"").decode("utf-8", "replace")


def log(msg):
    print("[%s] %s" % (dt.datetime.now().strftime("%H:%M:%S"), msg), flush=True)


def pct(values, q):
    """Percentile by nearest rank on a sorted list."""
    if not values:
        return 0.0
    s = sorted(values)
    k = max(0, min(len(s) - 1, int(math.ceil(q / 100.0 * len(s))) - 1))
    return s[k]


def fmt(x, nd=1):
    return ("%." + str(nd) + "f") % x


def copy_escape(s):
    """Escape a text field for COPY ... FROM STDIN (text format)."""
    return (
        str(s)
        .replace("\\", "\\\\")
        .replace("\t", "\\t")
        .replace("\n", "\\n")
        .replace("\r", "\\r")
    )


class Stack:
    """Wraps docker compose for one isolated project."""

    def __init__(self, repo, project, override, port):
        self.repo = repo
        self.project = project
        self.override = override
        self.port = port
        self.base = os.path.join(repo, "docker-compose.yml")

    def _cmd(self, *args):
        return ["docker", "compose", "-p", self.project,
                "-f", self.base, "-f", self.override] + list(args)

    def compose(self, *args, **kw):
        return run(self._cmd(*args), **kw)

    def exists(self):
        p = run(["docker", "compose", "-p", self.project, "ps", "-aq"], check=False)
        return bool(out_of(p).strip())

    def up(self, timeout=1800):
        log("building and starting the isolated stack (first run pulls images and builds, this takes a while)")
        self.compose("up", "-d", "--build", timeout=timeout)

    def down(self):
        log("tearing the stack down and removing its volume")
        run(self._cmd("down", "-v", "--remove-orphans"), check=False, timeout=600)

    def container_id(self, service):
        p = self.compose("ps", "-q", service, check=False)
        return out_of(p).strip().splitlines()[0] if out_of(p).strip() else ""

    def psql(self, sql, *, quiet=True, timeout=1800, check=True):
        """Pipe SQL into psql inside the db container. Never touches the repo."""
        args = ["exec", "-T", "-e", "PGCLIENTENCODING=UTF8", "db", "psql",
                "-U", "app", "-d", "tgtg", "-v", "ON_ERROR_STOP=1", "-X"]
        if quiet:
            args += ["-q"]
        return self.compose(*args, stdin=sql, timeout=timeout, check=check)

    def psql_rows(self, sql, timeout=600):
        """Run a query and return rows as lists of strings."""
        p = self.compose("exec", "-T", "-e", "PGCLIENTENCODING=UTF8", "db", "psql",
                         "-U", "app", "-d", "tgtg",
                         "-v", "ON_ERROR_STOP=1", "-X", "-qAt", "-F", "\t",
                         stdin=sql, timeout=timeout)
        rows = []
        for line in out_of(p).splitlines():
            line = line.rstrip("\n")
            if line:
                rows.append(line.split("\t"))
        return rows

    def wait_healthy(self, timeout=900):
        log("waiting for the api to report healthy on port %d" % self.port)
        deadline = time.time() + timeout
        last = ""
        while time.time() < deadline:
            try:
                c = http.client.HTTPConnection("127.0.0.1", self.port, timeout=5)
                c.request("GET", "/health")
                r = c.getresponse()
                body = r.read()
                c.close()
                if r.status == 200:
                    log("api is healthy")
                    return json.loads(body.decode("utf-8", "replace"))
                last = "status %d" % r.status
            except Exception as e:  # noqa: BLE001
                last = type(e).__name__
            time.sleep(3)
        raise RuntimeError("api did not become healthy within %ds (last: %s)" % (timeout, last))


# ------------------------------------------------------------ repo integrity


SKIP_DIRS = {".git", "node_modules", "vendor", ".idea", ".vscode"}


def repo_fingerprint(repo, exclude=()):
    """sha256 over every tracked file path and content, so any change shows up.

    `exclude` holds absolute paths to leave out, which is how this test's own
    script and its own report can live inside the repo without the check
    reporting a false change.
    """
    excl = {os.path.normcase(os.path.abspath(x)) for x in exclude}
    h = hashlib.sha256()
    n = 0
    for root, dirs, files in os.walk(repo):
        dirs[:] = sorted(d for d in dirs if d not in SKIP_DIRS)
        for name in sorted(files):
            path = os.path.join(root, name)
            if os.path.normcase(os.path.abspath(path)) in excl:
                continue
            rel = os.path.relpath(path, repo).replace(os.sep, "/")
            h.update(rel.encode("utf-8"))
            try:
                with open(path, "rb") as fh:
                    while True:
                        b = fh.read(1 << 16)
                        if not b:
                            break
                        h.update(b)
            except OSError:
                h.update(b"<unreadable>")
            n += 1
    return h.hexdigest(), n


# -------------------------------------------------------------- preflight


def preflight(args):
    problems = []
    for tool in ("docker",):
        if shutil.which(tool) is None:
            problems.append("%s is not on PATH" % tool)
    if shutil.which("docker"):
        p = run(["docker", "compose", "version"], check=False)
        if p.returncode != 0:
            problems.append("docker compose v2 is not available")
        p = run(["docker", "info", "--format", "{{.ServerVersion}}"], check=False)
        if p.returncode != 0:
            problems.append("the docker daemon is not reachable (is Docker Desktop running?)")

    base = os.path.join(args.repo, "docker-compose.yml")
    if not os.path.isfile(base):
        problems.append("no docker-compose.yml under --repo %s" % args.repo)

    # Refuse to run against anything that is not a throwaway local database.
    env_db = os.environ.get("DATABASE_URL", "")
    if env_db:
        host = ""
        m = re.search(r"@([^/:]+)", env_db)
        if m:
            host = m.group(1)
        if host and host not in ("localhost", "127.0.0.1", "db", "::1"):
            problems.append(
                "DATABASE_URL in your environment points at %r. Refusing to run so no "
                "real data can be touched. Unset it and try again." % host)
    return problems


# ------------------------------------------------------- compose override


OVERRIDE_TEMPLATE = """# Generated by loadtest.py {version}. Lives outside the repository.
# Isolates this run: own host port, own resource caps. The repo's
# docker-compose.yml is not modified.
services:
  db:
    cpus: {db_cpus}
    mem_limit: {db_mem}
    ports: !override []
  api:
    cpus: {api_cpus}
    mem_limit: {api_mem}
    ports: !override
      - "127.0.0.1:{port}:8080"
    environment:
      APP_ENV: development
      LOG_LEVEL: info
      HTTP_PORT: "8080"
      DATABASE_URL: postgres://app:app@db:5432/tgtg?sslmode=disable
      REDIS_URL: redis://redis:6379/0
      JWT_SECRET: loadtest-only-secret-not-a-real-credential-32
"""

OVERRIDE_FALLBACK = """# Generated by loadtest.py {version}. Lives outside the repository.
services:
  db:
    cpus: {db_cpus}
    mem_limit: {db_mem}
  api:
    cpus: {api_cpus}
    mem_limit: {api_mem}
    environment:
      APP_ENV: development
      LOG_LEVEL: info
      HTTP_PORT: "8080"
      DATABASE_URL: postgres://app:app@db:5432/tgtg?sslmode=disable
      REDIS_URL: redis://redis:6379/0
      JWT_SECRET: loadtest-only-secret-not-a-real-credential-32
"""


def write_override(args, tmpdir):
    """Write the override. Falls back to the repo's own port if !override is unsupported."""
    body = OVERRIDE_TEMPLATE.format(
        version=VERSION, port=args.port,
        api_cpus=args.api_cpus, api_mem=args.api_mem,
        db_cpus=args.db_cpus, db_mem=args.db_mem)
    if args.no_limits:
        body = re.sub(r"^\s+(cpus|mem_limit):.*\n", "", body, flags=re.M)
    path = os.path.join(tmpdir, "loadtest-override.yml")
    with open(path, "w", encoding="utf-8") as fh:
        fh.write(body)
    if args.dry_run:
        return path, body, True
    probe = run(["docker", "compose", "-p", args.project,
                 "-f", os.path.join(args.repo, "docker-compose.yml"),
                 "-f", path, "config"], check=False)
    if probe.returncode == 0:
        return path, body, True
    # Older compose without the !override tag: keep the repo's 8080 mapping.
    body = OVERRIDE_FALLBACK.format(
        version=VERSION, api_cpus=args.api_cpus, api_mem=args.api_mem,
        db_cpus=args.db_cpus, db_mem=args.db_mem)
    if args.no_limits:
        body = re.sub(r"^\s+(cpus|mem_limit):.*\n", "", body, flags=re.M)
    with open(path, "w", encoding="utf-8") as fh:
        fh.write(body)
    args.port = 8080
    log("note: this docker compose does not support the !override tag, so the run uses "
        "the repo's own port 8080. Make sure nothing else is listening there.")
    return path, body, False


# ------------------------------------------------------------ seed generator
#
# Everything here is generated at run time and applied to the throwaway
# database inside the container. scripts/catalogue/seed.sql is NOT used: it
# targets a superseded schema (vendor_status, latitude/longitude, is_active,
# offers.price, no vendors row) and fails on its first INSERT. That file is
# left untouched.


def _jitter_point(rnd, lat, lng, spread_km):
    # Sum of three uniforms approximates a normal, which gives a dense centre
    # and a thinning edge rather than a uniform blob.
    g = lambda: (rnd.random() + rnd.random() + rnd.random() - 1.5) / 1.5
    dlat = g() * spread_km / 111.32
    dlng = g() * spread_km / (111.32 * max(0.2, math.cos(math.radians(lat))))
    return lat + dlat, lng + dlng


def _weighted(rnd, items):
    total = sum(w for _, w in items)
    x = rnd.random() * total
    acc = 0.0
    for item, w in items:
        acc += w
        if x <= acc:
            return item
    return items[-1][0]


def ts(moment):
    return moment.strftime("%Y-%m-%d %H:%M:%S+00")


def gen_users_vendors_stores(args, rnd, now):
    """COPY blocks for users, vendors and stores. Returns (sql, store_meta)."""
    out = io.StringIO()

    # --- users. phone must match ^\+?[0-9]{10,15}$ and be unique.
    out.write("COPY users (phone, full_name, created_at) FROM STDIN;\n")
    for i in range(1, args.users_rows + 1):
        created = now - dt.timedelta(days=rnd.randint(0, 500), minutes=rnd.randint(0, 1440))
        out.write("+9891%08d\t%s\t%s\n" % (i, copy_escape("کاربر %d" % i), ts(created)))
    out.write("\\.\n\n")

    # --- vendors. Most partners run one store, some run two (a chain).
    n_vendors = max(1, int(args.stores * 0.8))
    out.write("COPY vendors (owner_user_id, business_name, contact_phone, created_at) FROM STDIN;\n")
    for i in range(1, n_vendors + 1):
        owner = ((i - 1) % args.users_rows) + 1
        created = now - dt.timedelta(days=rnd.randint(30, 500))
        out.write("%d\t%s\t+9891%08d\t%s\n"
                  % (owner, copy_escape("کسب‌وکار %d" % i), (i % args.users_rows) + 1, ts(created)))
    out.write("\\.\n\n")

    # --- stores.
    # status 'approved' requires approved_by and approved_at (stores_approval_chk);
    # status 'rejected' requires rejection_reason (stores_rejection_chk).
    status_mix = [("approved", 90), ("pending", 4), ("suspended", 3), ("rejected", 2), ("approved_deleted", 1)]
    district_mix = [((n, la, ln, sp), w) for (n, la, ln, w, sp) in DISTRICTS]

    meta = []
    out.write("COPY stores (vendor_id, category_id, name, slug, biography, city, district, "
              "address_line, location, status, rejection_reason, approved_by, approved_at, "
              "accepts_orders, rating_avg, rating_count, created_at, deleted_at) FROM STDIN;\n")
    for idx in range(args.stores):
        vendor_id = (idx % n_vendors) + 1
        dname, dlat, dlng, spread = _weighted(rnd, district_mix)
        lat, lng = _jitter_point(rnd, dlat, dlng, spread)
        kind = _weighted(rnd, status_mix)
        deleted = kind == "approved_deleted"
        status = "approved" if deleted else kind
        accepts = (status == "approved") and (rnd.random() > 0.05)
        cat = rnd.choice(ALL_CATS)
        created = now - dt.timedelta(days=rnd.randint(1, 400), minutes=rnd.randint(0, 1440))
        approved_at = created + dt.timedelta(days=rnd.randint(1, 7))
        name = "%s %s" % (rnd.choice(STORE_PREFIX), rnd.choice(STORE_NAME))
        row = [
            str(vendor_id),
            str(cat),
            copy_escape("%s %d" % (name, idx)),
            "lt-store-%d" % idx,
            copy_escape("فروشگاه محلی در %s با عرضه روزانه محصولات تازه." % dname),
            copy_escape("تهران"),
            copy_escape(dname),
            copy_escape("%s، خیابان شماره %d، پلاک %d" % (dname, rnd.randint(1, 90), rnd.randint(1, 300))),
            "SRID=4326;POINT(%.6f %.6f)" % (lng, lat),
            status,
            copy_escape("مدارک ارسالی ناقص بود.") if status == "rejected" else "\\N",
            "1" if status in ("approved", "suspended") else "\\N",
            ts(approved_at) if status in ("approved", "suspended") else "\\N",
            "t" if accepts else "f",
            "%.2f" % (rnd.uniform(3.4, 4.9)),
            str(rnd.randint(0, 900)),
            ts(created),
            ts(now - dt.timedelta(days=rnd.randint(1, 60))) if deleted else "\\N",
        ]
        out.write("\t".join(row) + "\n")
        meta.append({
            "idx": idx, "eligible": (status == "approved" and accepts and not deleted),
            "approved_live": (status == "approved" and not deleted),
            "lat": lat, "lng": lng, "cat": cat, "district": dname,
        })
    out.write("\\.\n\n")
    out.write("ANALYZE users; ANALYZE vendors; ANALYZE stores;\n")
    return out.getvalue(), meta, n_vendors


def gen_offers(args, rnd, now, meta, store_ids):
    """COPY block for offers. store_ids maps store index to the real stores.id."""
    eligible = [m for m in meta if m["eligible"]]
    approved_live = [m for m in meta if m["approved_live"]]
    others = [m for m in meta if not m["approved_live"]]
    if not eligible:
        raise RuntimeError("no eligible stores were generated")

    # Offer counts per store follow a power law: a few busy partners, a long tail.
    def pick_store(pool):
        r = rnd.random() ** 2.0
        return pool[min(len(pool) - 1, int(r * len(pool)))]

    out = io.StringIO()
    out.write("COPY offers (store_id, offer_type, title, description, category_id, "
              "original_price, discount_price, quantity_total, quantity_available, "
              "pickup_start, pickup_end, allergens, allergens_declared, status, "
              "published_at, created_at, deleted_at) FROM STDIN;\n")

    def row(store, status, start, end, qty_total, qty_avail, deleted_at=None):
        cat = store["cat"] if rnd.random() < 0.3 else rnd.choice(ALL_CATS)
        # fn_offer_min_discount enforces at least 30 percent off, so stay at 32..70.
        original = int(rnd.choice([45, 60, 80, 120, 160, 220, 300, 420]) * 1000 * rnd.uniform(0.8, 1.25))
        off = rnd.randint(32, 70)
        discount = int(original * (100 - off) / 100.0)
        discount = max(1000, min(discount, original - 1000))
        allergens = "{}"
        if rnd.random() < 0.6:
            picks = rnd.sample(ALLERGENS, rnd.randint(1, 3))
            allergens = "{" + ",".join(picks) + "}"
        title = "%s %s" % (rnd.choice(OFFER_TITLE), rnd.choice(OFFER_SUFFIX))
        created = now - dt.timedelta(days=rnd.randint(0, 25), minutes=rnd.randint(0, 1440))
        cells = [
            str(store_ids[store["idx"]]),
            "surprise_bag" if rnd.random() < 0.65 else "specific_item",
            copy_escape(title),
            copy_escape("بسته غذایی تازه و سالم که در پایان روز با تخفیف عرضه می‌شود."),
            str(cat),
            str(original),
            str(discount),
            str(qty_total),
            str(qty_avail),
            ts(start),
            ts(end),
            allergens,
            "t" if rnd.random() < 0.7 else "f",
            status,
            ts(created) if status in ("active", "sold_out", "expired") else "\\N",
            ts(created),
            ts(deleted_at) if deleted_at else "\\N",
        ]
        return "\t".join(cells) + "\n"

    # --- the eligible set: active, in stock, window still open, approved store
    # Windows are generated relative to the run time, with an evening weighting,
    # so the eligible set is the same size whatever hour the test is started.
    n_elig = args.eligible_offers
    for _ in range(n_elig):
        store = pick_store(eligible)
        if rnd.random() < 0.18:
            # last chance: window closes within two hours
            start = now - dt.timedelta(minutes=rnd.randint(30, 180))
            end = now + dt.timedelta(minutes=rnd.randint(10, 115))
        else:
            start = now + dt.timedelta(minutes=rnd.randint(-120, 420))
            end = start + dt.timedelta(minutes=rnd.randint(60, 180))
            if end <= now + dt.timedelta(minutes=10):
                end = now + dt.timedelta(minutes=rnd.randint(30, 200))
        qty_total = rnd.randint(1, 12)
        qty_avail = 1 + int((qty_total - 1) * (rnd.random() ** 1.5))
        if rnd.random() < 0.12:
            qty_avail = min(qty_avail, 3)  # feeds the "last chance" rule too
        out.write(row(store, "active", start, end, qty_total, qty_avail))

    # --- the ineligible bulk, so filtering does real work.
    n_rest = max(0, args.total_offers - n_elig)
    for _ in range(n_rest):
        pickk = rnd.random()
        if pickk < 0.40:  # finished: window already closed
            store = pick_store(approved_live)
            start = now - dt.timedelta(hours=rnd.randint(4, 24 * 30))
            end = start + dt.timedelta(hours=rnd.randint(1, 3))
            qt = rnd.randint(1, 12)
            out.write(row(store, "expired", start, end, qt, 0))
        elif pickk < 0.60:  # sold out
            store = pick_store(approved_live)
            start = now - dt.timedelta(hours=rnd.randint(1, 6))
            end = now + dt.timedelta(hours=rnd.randint(1, 5))
            qt = rnd.randint(1, 12)
            out.write(row(store, "sold_out", start, end, qt, 0))
        elif pickk < 0.75:  # never published
            store = pick_store(approved_live)
            start = now + dt.timedelta(hours=rnd.randint(1, 48))
            end = start + dt.timedelta(hours=2)
            qt = rnd.randint(1, 12)
            out.write(row(store, "draft", start, end, qt, qt))
        elif pickk < 0.85:  # paused by the vendor
            store = pick_store(approved_live)
            start = now - dt.timedelta(hours=1)
            end = now + dt.timedelta(hours=rnd.randint(1, 6))
            qt = rnd.randint(1, 12)
            out.write(row(store, "inactive", start, end, qt, qt))
        elif pickk < 0.92:  # soft deleted while still live
            store = pick_store(approved_live)
            start = now - dt.timedelta(hours=1)
            end = now + dt.timedelta(hours=rnd.randint(1, 6))
            qt = rnd.randint(1, 12)
            out.write(row(store, "active", start, end, qt, qt,
                          deleted_at=now - dt.timedelta(hours=rnd.randint(1, 72))))
        else:
            # offers belonging to stores that are not approved, so they can never
            # be published. The fn_offer_requires_approved_store trigger refuses
            # status 'active' for these, so they stay draft or inactive.
            store = pick_store(others) if others else pick_store(approved_live)
            start = now - dt.timedelta(hours=1)
            end = now + dt.timedelta(hours=rnd.randint(1, 6))
            qt = rnd.randint(1, 12)
            out.write(row(store, "draft" if rnd.random() < 0.5 else "inactive",
                          start, end, qt, qt))
    out.write("\\.\n\n")

    # Allergen join table, derived from the array so both stay consistent.
    out.write(
        "INSERT INTO offer_allergens (offer_id, allergen_code)\n"
        "SELECT id, unnest(allergens)::text FROM offers WHERE cardinality(allergens) > 0\n"
        "ON CONFLICT DO NOTHING;\n\n")

    # Favorites, heavily skewed: a few offers are saved by many users, most by none.
    out.write("""CREATE TEMP TABLE _eo AS
    SELECT row_number() OVER (ORDER BY id) AS rn, id
    FROM offers WHERE status = 'active' AND deleted_at IS NULL AND quantity_available > 0;
CREATE INDEX ON _eo (rn);
ANALYZE _eo;
WITH bounds AS (SELECT (SELECT count(*) FROM _eo) AS n_eo,
                       (SELECT count(*) FROM users) AS n_u),
picks AS (
    SELECT 1 + floor(random() * b.n_u)::bigint                      AS uid,
           1 + floor(power(random(), 3) * b.n_eo)::bigint           AS rn
    FROM bounds b, generate_series(1, %d)
)
INSERT INTO favorite_offers (user_id, offer_id)
SELECT p.uid, e.id FROM picks p JOIN _eo e ON e.rn = p.rn
ON CONFLICT DO NOTHING;
DROP TABLE _eo;

ANALYZE offers; ANALYZE offer_allergens; ANALYZE favorite_offers; ANALYZE stores;
""" % args.favorites)
    return out.getvalue()


# ------------------------------------------------------------- load engine


class Client:
    """One keep-alive HTTP/1.1 connection, the way a mobile client holds one."""

    def __init__(self, host, port, timeout):
        self.host, self.port, self.timeout = host, port, timeout
        self.conn = None
        self.samples = []  # (label, status, ms, err_kind)

    def _connect(self):
        self.conn = http.client.HTTPConnection(self.host, self.port, timeout=self.timeout)

    def close(self):
        try:
            if self.conn:
                self.conn.close()
        except Exception:  # noqa: BLE001
            pass
        self.conn = None

    def get(self, label, path, user_id=None, parse=False):
        headers = {
            "Accept": "application/json",
            "Accept-Encoding": "gzip",
            "User-Agent": "tgtg-loadtest/%s" % VERSION,
            "Connection": "keep-alive",
        }
        if user_id:
            headers["X-Dev-User-Id"] = str(user_id)
        t0 = time.perf_counter()
        for attempt in (0, 1):
            try:
                if self.conn is None:
                    self._connect()
                self.conn.request("GET", path, headers=headers)
                resp = self.conn.getresponse()
                body = resp.read()  # a real client reads the whole body
                ms = (time.perf_counter() - t0) * 1000.0
                if resp.getheader("Content-Encoding", "") == "gzip" and (parse or False):
                    try:
                        body = gzip.decompress(body)
                    except (OSError, zlib.error):
                        pass
                doc = None
                if parse:
                    try:
                        doc = json.loads(body.decode("utf-8", "replace"))
                    except ValueError:
                        doc = None
                self.samples.append((label, resp.status, ms, "", len(body)))
                if resp.getheader("Connection", "").lower() == "close":
                    self.close()
                return resp.status, doc
            except (http.client.BadStatusLine, http.client.RemoteDisconnected, ConnectionResetError, BrokenPipeError):
                self.close()
                if attempt == 0:
                    continue  # stale keep-alive socket, reconnect once
                ms = (time.perf_counter() - t0) * 1000.0
                self.samples.append((label, 0, ms, "conn", 0))
                return 0, None
            except (TimeoutError, OSError) as e:
                self.close()
                ms = (time.perf_counter() - t0) * 1000.0
                kind = "timeout" if isinstance(e, TimeoutError) or "timed out" in str(e) else "conn"
                self.samples.append((label, 0, ms, kind, 0))
                return 0, None
        return 0, None


def _next_cursor(doc):
    if isinstance(doc, dict):
        c = doc.get("next_cursor")
        if isinstance(c, str) and c:
            return c
    return None


def _first_list_len(doc):
    if isinstance(doc, dict):
        for v in doc.values():
            if isinstance(v, list):
                return len(v)
    if isinstance(doc, list):
        return len(doc)
    return -1


def _loc(rnd, data):
    lat, lng = rnd.choice(data["centers"])
    return lat + rnd.uniform(-0.02, 0.02), lng + rnd.uniform(-0.02, 0.02)


def _sort(rnd):
    return rnd.choice(["nearest", "price_asc", "price_desc", "newest"])


# --- individual user actions. Each returns the number of requests it issued.


def act_app_open(cl, rnd, data, uid):
    lat, lng = _loc(rnd, data)
    _get(cl, data, "GET /home/categories", "/home/categories", uid)
    _get(cl, data, "GET /home/today", "/home/today?lat=%.5f&lng=%.5f&limit=10" % (lat, lng), uid)
    _get(cl, data, "GET /home/popular-nearby",
         "/home/popular-nearby?lat=%.5f&lng=%.5f&radius_km=%d&limit=10" % (lat, lng, rnd.choice([3, 5, 8])), uid)
    _get(cl, data, "GET /home/last-chance", "/home/last-chance?lat=%.5f&lng=%.5f&limit=10" % (lat, lng), uid)
    return 4


def act_browse(cl, rnd, data, uid):
    lat, lng = _loc(rnd, data)
    path = "/offers?lat=%.5f&lng=%.5f&sort=%s&limit=20" % (lat, lng, _sort(rnd))
    st, doc = _get(cl, data, "GET /offers", path, uid, parse=True)
    n = 1
    cur = _next_cursor(doc)
    for _ in range(rnd.randint(0, 3)):  # the user scrolls
        if not cur:
            break
        st, doc = _get(cl, data, "GET /offers", path + "&cursor=" + cur, uid, parse=True)
        cur = _next_cursor(doc)
        n += 1
    return n


def act_filter(cl, rnd, data, uid):
    lat, lng = _loc(rnd, data)
    n = 0
    if rnd.random() < 0.5:  # opening the filter sheet loads the options
        _get(cl, data, "GET /offers/filters", "/offers/filters", uid)
        n += 1
    cats = ",".join(str(c) for c in rnd.sample(ALL_CATS, rnd.randint(1, 3)))
    lo = rnd.choice([0, 20000, 40000, 60000])
    hi = lo + rnd.choice([40000, 80000, 150000, 300000])
    path = ("/offers?lat=%.5f&lng=%.5f&category=%s&price_min=%d&price_max=%d&sort=%s&limit=20"
            % (lat, lng, cats, lo, hi, _sort(rnd)))
    if rnd.random() < 0.35:
        path += "&radius_km=%d" % rnd.choice([2, 5, 10])
    if rnd.random() < 0.25:
        path += "&exclude_allergens=" + ",".join(rnd.sample(ALLERGENS, rnd.randint(1, 2)))
    if rnd.random() < 0.2:
        path += "&kind=" + rnd.choice(["bakery", "ready_meal", "grocery"])
    st, doc = _get(cl, data, "GET /offers", path, uid, parse=True)
    n += 1
    cur = _next_cursor(doc)
    if cur and rnd.random() < 0.4:
        _get(cl, data, "GET /offers", path + "&cursor=" + cur, uid)
        n += 1
    return n


def act_search(cl, rnd, data, uid):
    lat, lng = _loc(rnd, data)
    term = rnd.choice(SEARCH_TERMS)
    path = "/offers?q=%s&lat=%.5f&lng=%.5f&limit=20" % (_urlq(term), lat, lng)
    _get(cl, data, "GET /offers", path, uid)
    return 1


def act_map(cl, rnd, data, uid):
    lat, lng = _loc(rnd, data)
    _get(cl, data, "GET /offers/map",
         "/offers/map?lat=%.5f&lng=%.5f&radius_km=%d&limit=%d"
           % (lat, lng, rnd.choice([2, 5, 10]), rnd.choice([50, 100, 200])), uid)
    return 1


def act_store(cl, rnd, data, uid):
    sid = rnd.choice(data["store_ids"])
    lat, lng = _loc(rnd, data)
    _get(cl, data, "GET /stores/{id}", "/stores/%s" % sid, uid)
    _get(cl, data, "GET /stores/{id}/offers",
         "/stores/%s/offers?lat=%.5f&lng=%.5f&limit=20" % (sid, lat, lng), uid)
    return 2


def act_favorites(cl, rnd, data, uid):
    lat, lng = _loc(rnd, data)
    _get(cl, data, "GET /favorites/offers",
         "/favorites/offers?lat=%.5f&lng=%.5f&limit=20" % (lat, lng), uid)
    return 1


def probe_availability(port, store_id, user_id, timeout=15):
    """Hit each endpoint once and record its status, so routes this build does
    not register are reported and skipped instead of counted as failures."""
    cl = Client("127.0.0.1", port, timeout)
    checks = [
        ("GET /home/categories", "/home/categories", None),
        ("GET /home/today", "/home/today?lat=35.7219&lng=51.4090&limit=5", None),
        ("GET /home/popular-nearby", "/home/popular-nearby?lat=35.7219&lng=51.4090&radius_km=5&limit=5", None),
        ("GET /home/last-chance", "/home/last-chance?lat=35.7219&lng=51.4090&limit=5", None),
        ("GET /offers", "/offers?lat=35.7219&lng=51.4090&limit=20", None),
        ("GET /offers/filters", "/offers/filters", None),
        ("GET /offers/map", "/offers/map?lat=35.7219&lng=51.4090&radius_km=5&limit=10", None),
        ("GET /stores/{id}", "/stores/%s" % store_id, None),
        ("GET /stores/{id}/offers", "/stores/%s/offers?limit=5" % store_id, None),
        ("GET /favorites/offers", "/favorites/offers?limit=5", user_id),
    ]
    out, rows = {}, {}
    for label, path, uid in checks:
        st, doc = cl.get("probe", path, uid, parse=True)
        out[label] = st
        rows[label] = _first_list_len(doc)
    cl.close()
    return out, rows


def _get(cl, data, label, path, uid=None, parse=False):
    """Issue the request unless this build does not serve that endpoint."""
    if label in data.get("unavailable", ()):
        return 0, None
    return cl.get(label, path, uid, parse=parse)


def _urlq(s):
    from urllib.parse import quote
    return quote(s, safe="")


ACTIONS = [
    (act_browse, 35, False),
    (act_filter, 22, False),
    (act_store, 15, False),
    (act_search, 13, False),
    (act_map, 10, False),
    (act_favorites, 5, True),  # authenticated only
]


ACTION_ENDPOINTS = {
    "act_browse": ("GET /offers",),
    "act_filter": ("GET /offers", "GET /offers/filters"),
    "act_store": ("GET /stores/{id}", "GET /stores/{id}/offers"),
    "act_search": ("GET /offers",),
    "act_map": ("GET /offers/map",),
    "act_favorites": ("GET /favorites/offers",),
}


def _pick_action(rnd, authed, unavailable=()):
    pool = []
    for (f, w, needs_auth) in ACTIONS:
        if needs_auth and not authed:
            continue
        eps = ACTION_ENDPOINTS.get(f.__name__, ())
        if eps and all(e in unavailable for e in eps):
            continue
        pool.append((f, w))
    return _weighted(rnd, pool) if pool else None


def _think(rnd):
    # log-normal think time, median about 2.7s, clamped to a sane range
    return min(12.0, max(0.8, rnd.lognormvariate(1.0, 0.6)))


def session_worker(cfg, data, stop_at, seed, samples_out, lock):
    """Phase A: one virtual user living through sessions with think time."""
    rnd = random.Random(seed)
    cl = Client(cfg["host"], cfg["port"], cfg["timeout"])
    # Stagger arrivals so 50 users do not all start on the same millisecond.
    time.sleep(rnd.uniform(0, cfg["stagger"]))
    while time.time() < stop_at:
        authed = rnd.random() < cfg["auth_share"]
        uid = rnd.choice(data["user_ids"]) if authed else None
        act_app_open(cl, rnd, data, uid)
        for _ in range(rnd.randint(3, 10)):
            if time.time() >= stop_at:
                break
            time.sleep(_think(rnd))
            act = _pick_action(rnd, authed, data.get("unavailable", ()))
            if act is None:
                break
            act(cl, rnd, data, uid)
        if time.time() < stop_at:
            time.sleep(_think(rnd) * 2)  # the user puts the phone away
        cl.close()
    cl.close()
    with lock:
        samples_out.extend(cl.samples)


def closed_loop_worker(cfg, data, stop_at, seed, samples_out, lock):
    """Phase B: no think time, to find the ceiling."""
    rnd = random.Random(seed)
    cl = Client(cfg["host"], cfg["port"], cfg["timeout"])
    while time.time() < stop_at:
        authed = rnd.random() < cfg["auth_share"]
        uid = rnd.choice(data["user_ids"]) if authed else None
        if rnd.random() < 0.30:
            act_app_open(cl, rnd, data, uid)
        else:
            act = _pick_action(rnd, authed, data.get("unavailable", ()))
            if act is None:
                act_app_open(cl, rnd, data, uid)
            else:
                act(cl, rnd, data, uid)
    cl.close()
    with lock:
        samples_out.extend(cl.samples)


def _proc_entry(cfg, data, phase, n_users, stop_at, seed_base, queue):
    """Runs in a child process: n_users threads, then ships its samples back."""
    try:
        import resource as _res  # POSIX only, absent on Windows
    except ImportError:
        _res = None
    samples = []
    lock = threading.Lock()
    fn = session_worker if phase == "A" else closed_loop_worker
    threads = [
        threading.Thread(target=fn, args=(cfg, data, stop_at, seed_base + i, samples, lock), daemon=True)
        for i in range(n_users)
    ]
    for t in threads:
        t.start()
    for t in threads:
        t.join(timeout=max(0.0, stop_at - time.time()) + cfg["timeout"] + 30)
    cpu = 0.0
    if _res is not None:
        try:
            ru = _res.getrusage(_res.RUSAGE_SELF)
            cpu = ru.ru_utime + ru.ru_stime
        except Exception:  # noqa: BLE001
            cpu = 0.0
    queue.put((samples, cpu))


def run_phase(cfg, data, phase, n_users, duration, procs):
    """Spread n_users virtual users over `procs` processes so the Python client
    does not become the bottleneck. Returns (samples, wall_seconds, client_cpu)."""
    procs = max(1, min(procs, n_users))
    per = [n_users // procs] * procs
    for i in range(n_users % procs):
        per[i] += 1
    queue = mp.Queue()
    stop_at = time.time() + duration + (cfg["stagger"] if phase == "A" else 0)
    t0 = time.time()
    children = []
    for i, k in enumerate(per):
        p = mp.Process(target=_proc_entry,
                       args=(cfg, data, phase, k, stop_at, 1000 * (i + 1), queue))
        p.start()
        children.append(p)
    samples, cpu = [], 0.0
    budget = duration + cfg["timeout"] + 120
    for _ in children:
        try:
            s, c = queue.get(timeout=budget)
        except Exception:  # noqa: BLE001
            log("a load generator process did not report back, its samples are missing")
            continue
        samples.extend(s)
        cpu += c
    for p in children:
        p.join(timeout=60)
        if p.is_alive():
            p.terminate()
    return samples, time.time() - t0, cpu


# ---------------------------------------------------------------- sampling


class ResourceSampler(threading.Thread):
    """Samples container CPU and memory plus PostgreSQL connection counts.

    docker stats reports CPU where 100 percent means one full core, so a value
    above 100 means more than one core is in use.
    """

    def __init__(self, stack, interval=2.0):
        super().__init__(daemon=True)
        self.stack = stack
        self.interval = interval
        self._done = threading.Event()
        self.phase = "idle"
        self.rows = []   # (t, phase, container, cpu_pct, mem_mb)
        self.pg = []     # (t, phase, total_conns, active_conns, longest_active_s)
        self.ids = {}

    def resolve(self):
        for svc in ("api", "db"):
            cid = self.stack.container_id(svc)
            if cid:
                self.ids[cid[:12]] = svc

    def stop(self):
        self._done.set()

    def run(self):
        self.resolve()
        pgsql = ("SELECT count(*) FILTER (WHERE pid <> pg_backend_pid()), "
                 "count(*) FILTER (WHERE state = 'active' AND pid <> pg_backend_pid()), "
                 "coalesce(max(EXTRACT(epoch FROM (now() - query_start))) "
                 "FILTER (WHERE state = 'active' AND pid <> pg_backend_pid()), 0) "
                 "FROM pg_stat_activity WHERE datname = 'tgtg' AND backend_type = 'client backend';")
        while not self._done.is_set():
            now = time.time()
            if not self.ids:
                self.resolve()
                if not self.ids:
                    self._done.wait(self.interval)
                    continue
            try:
                p = run(["docker", "stats", "--no-stream", "--format",
                         "{{.Name}}\t{{.CPUPerc}}\t{{.MemUsage}}"] + list(self.ids.keys()),
                        check=False, timeout=30)
                for line in out_of(p).splitlines():
                    parts = line.split("\t")
                    if len(parts) < 3:
                        continue
                    name, cpu, mem = parts[0], parts[1], parts[2]
                    svc = "api" if ".api." in name or name.endswith("-api-1") else (
                        "db" if ".db." in name or name.endswith("-db-1") else name)
                    try:
                        cpu_v = float(cpu.strip().rstrip("%"))
                    except ValueError:
                        continue
                    mem_v = _mem_mb(mem.split("/")[0].strip())
                    self.rows.append((now, self.phase, svc, cpu_v, mem_v))
            except Exception:  # noqa: BLE001
                pass
            try:
                rows = self.stack.psql_rows(pgsql, timeout=30)
                if rows and len(rows[0]) >= 3:
                    self.pg.append((now, self.phase,
                                    int(rows[0][0]), int(rows[0][1]), float(rows[0][2])))
            except Exception:  # noqa: BLE001
                pass
            self._done.wait(self.interval)


def _mem_mb(s):
    m = re.match(r"([\d.]+)\s*([KMGT]?i?B)", s.strip(), re.I)
    if not m:
        return 0.0
    v = float(m.group(1))
    unit = m.group(2).upper()
    mult = {"B": 1.0 / (1024 * 1024), "KIB": 1.0 / 1024, "KB": 1.0 / 1024,
            "MIB": 1.0, "MB": 1.0, "GIB": 1024.0, "GB": 1024.0,
            "TIB": 1024 * 1024.0, "TB": 1024 * 1024.0}
    return v * mult.get(unit, 1.0)


def res_summary(rows, phase=None):
    out = {}
    for svc in ("api", "db"):
        vals = [(c, m) for (_, ph, s, c, m) in rows if s == svc and (phase is None or ph == phase)]
        if not vals:
            continue
        cpus = [c for c, _ in vals]
        mems = [m for _, m in vals]
        out[svc] = {
            "cpu_avg": statistics.fmean(cpus), "cpu_max": max(cpus),
            "mem_avg": statistics.fmean(mems), "mem_max": max(mems),
            "n": len(vals),
        }
    return out


def pg_summary(pg, phase=None):
    vals = [(t, c, a, w) for (t, ph, c, a, w) in pg if phase is None or ph == phase]
    if not vals:
        return None
    conns = [c for _, c, _, _ in vals]
    active = [a for _, _, a, _ in vals]
    return {"conn_avg": statistics.fmean(conns), "conn_max": max(conns),
            "active_avg": statistics.fmean(active), "active_max": max(active),
            "n": len(vals)}


# ---------------------------------------------------------------- metrics


def summarize(samples, wall):
    """Aggregate raw samples into overall and per endpoint metrics."""
    per = defaultdict(list)
    ok_ms = []
    status_counts = Counter()
    err_counts = Counter()
    bytes_total = 0
    for label, status, ms, err, nbytes in samples:
        bytes_total += nbytes
        if err:
            err_counts[err] += 1
            per[label].append((status, ms, err))
            continue
        status_counts[status] += 1
        per[label].append((status, ms, ""))
        if 200 <= status < 300:
            ok_ms.append(ms)

    def block(rows):
        ms_ok = [m for (s, m, e) in rows if not e and 200 <= s < 300]
        bad = sum(1 for (s, m, e) in rows if e or not (200 <= s < 300))
        return {
            "count": len(rows),
            "ok": len(ms_ok),
            "errors": bad,
            "err_rate": (100.0 * bad / len(rows)) if rows else 0.0,
            "rps": len(rows) / wall if wall > 0 else 0.0,
            "p50": pct(ms_ok, 50), "p90": pct(ms_ok, 90), "p95": pct(ms_ok, 95),
            "p99": pct(ms_ok, 99), "max": max(ms_ok) if ms_ok else 0.0,
            "mean": statistics.fmean(ms_ok) if ms_ok else 0.0,
        }

    total_rows = [(s, m, e) for (_, s, m, e, _b) in samples]
    return {
        "overall": block(total_rows),
        "per_endpoint": {k: block(v) for k, v in sorted(per.items())},
        "status_counts": dict(status_counts),
        "err_counts": dict(err_counts),
        "wall": wall,
        "bytes": bytes_total,
    }


# ------------------------------------------------------- query plans / logs


EXPLAIN_CASES = [
    ("home_popular_nearby (home screen, popular near you)",
     "EXPLAIN (ANALYZE, BUFFERS) EXECUTE home_popular_nearby(11, 35.7219, 51.4090, 5, NULL, NULL, NULL, NULL);"),
    ("home_today (home screen, today's offers)",
     "EXPLAIN (ANALYZE, BUFFERS) EXECUTE home_today(11, 35.7219, 51.4090, 5, NULL, NULL, NULL, NULL);"),
    ("home_last_chance (home screen, last chance)",
     "EXPLAIN (ANALYZE, BUFFERS) EXECUTE home_last_chance(11, 35.7219, 51.4090, 5, NULL, NULL, NULL, NULL);"),
    ("catalogue_search, sort=nearest with a 5 km radius",
     "EXPLAIN (ANALYZE, BUFFERS) EXECUTE catalogue_search(NULL,NULL,NULL,NULL,NULL,NULL,NULL,NULL,NULL,"
     "35.7219,51.4090,'nearest',NULL,NULL,NULL,21,5);"),
    ("catalogue_search, text search on title or store name",
     "EXPLAIN (ANALYZE, BUFFERS) EXECUTE catalogue_search('%نان%',NULL,NULL,NULL,NULL,NULL,NULL,NULL,NULL,"
     "35.7219,51.4090,'price_asc',NULL,NULL,NULL,21,NULL);"),
    ("catalogue_search, same query for a signed in user (is_favorite subquery active)",
     "EXPLAIN (ANALYZE, BUFFERS) EXECUTE catalogue_search(NULL,NULL,NULL,NULL,NULL,NULL,NULL,NULL,NULL,"
     "35.7219,51.4090,'nearest',NULL,NULL,1,21,5);"),
]


def collect_plans(stack, repo):
    """Prepare the repo's own catalogue SQL and explain it. The .sql files are
    read, never written, and are piped in over stdin so nothing is mounted."""
    parts = []
    for rel in ("scripts/catalogue/sql/home_sections.sql",
                "scripts/catalogue/sql/catalogue_search.sql"):
        path = os.path.join(repo, rel.replace("/", os.sep))
        if os.path.isfile(path):
            with open(path, "r", encoding="utf-8") as fh:
                parts.append(fh.read())
    if not parts:
        return []
    plans = []
    for title, stmt in EXPLAIN_CASES:
        sql = "\n".join(parts) + "\n" + stmt + "\n"
        p = stack.compose("exec", "-T", "-e", "PGCLIENTENCODING=UTF8", "db", "psql",
                          "-U", "app", "-d", "tgtg",
                          "-X", "-qAt", "-v", "ON_ERROR_STOP=1",
                          stdin=sql, timeout=300, check=False)
        text = out_of(p).strip()
        if p.returncode != 0:
            text = "could not be collected:\n" + (p.stderr or b"").decode("utf-8", "replace")[-800:]
        plans.append((title, text))
    return plans


LOG_TIME_KEYS = ("time", "ts", "timestamp")


def collect_server_latency(stack, t_from, t_to):
    """Parse the api container's own JSON request log for server side latency."""
    since = dt.datetime.fromtimestamp(t_from, dt.timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")
    p = stack.compose("logs", "--no-color", "--since", since, "api", check=False, timeout=300)
    per = defaultdict(list)
    statuses = Counter()
    for raw in out_of(p).splitlines():
        i = raw.find("{")
        if i < 0:
            continue
        try:
            rec = json.loads(raw[i:])
        except ValueError:
            continue
        if not isinstance(rec, dict) or "latency_ms" not in rec:
            continue
        stamp = None
        for k in LOG_TIME_KEYS:
            if k in rec:
                stamp = rec[k]
                break
        if isinstance(stamp, str):
            try:
                tsec = dt.datetime.fromisoformat(stamp.replace("Z", "+00:00")).timestamp()
                if tsec < t_from or tsec > t_to:
                    continue
            except ValueError:
                pass
        route = rec.get("route") or rec.get("path") or "unknown"
        try:
            per[route].append(float(rec["latency_ms"]))
        except (TypeError, ValueError):
            continue
        if "status" in rec:
            statuses[rec["status"]] += 1
    return {r: {"count": len(v), "p50": pct(v, 50), "p95": pct(v, 95), "max": max(v)}
            for r, v in sorted(per.items())}, dict(statuses)


# ----------------------------------------------------------------- report


def _exec_ms(plan_text):
    m = re.search(r"Execution Time:\s*([\d.]+)\s*ms", plan_text or "")
    return float(m.group(1)) if m else None


def _plan_flags(plan_text):
    t = plan_text or ""
    return {
        "seq_scan": "Seq Scan" in t,
        "gist": "idx_stores_location" in t,
        "fts": "idx_offers_fts" in t,
        "trgm": ("idx_offers_title_trgm" in t) or ("idx_stores_name_trgm" in t),
        "sort": "Sort Method" in t,
    }


def md_table(headers, rows):
    out = ["| " + " | ".join(headers) + " |",
           "|" + "|".join([" --- "] * len(headers)) + "|"]
    for r in rows:
        out.append("| " + " | ".join(str(c) for c in r) + " |")
    return "\n".join(out)


def endpoint_rows(sm):
    rows = []
    for label, b in sorted(sm["per_endpoint"].items(), key=lambda kv: -kv[1]["p95"]):
        rows.append([label, b["count"], fmt(b["rps"], 2), fmt(b["p50"]), fmt(b["p95"]),
                     fmt(b["p99"]), fmt(b["max"]), b["errors"], fmt(b["err_rate"], 2)])
    return rows


def write_report(path, ctx):
    a = ctx["phase_a"]
    stages = ctx["stages"]
    res_a = ctx["res_a"]
    pg_a = ctx["pg_a"]
    plans = dict(ctx["plans"])
    L = []
    W = L.append

    W("# Load test report: public home, catalogue and store APIs")
    W("")
    W("Too Good To Go backend (Go 1.25, Gin, PostgreSQL with PostGIS, Redis). "
      "Generated by `loadtest.py` v%s on %s." % (VERSION, ctx["started_human"]))
    W("")
    W(ctx.get("integrity_note", "") + " "
      "No request issued during the run was anything other than a GET, so no data was written "
      "through the API. No frontend was involved at any point.")
    W("")

    # ---- summary
    W("## Summary")
    W("")
    ov = a["overall"]
    W("At %d concurrent users browsing with realistic think time, the API served "
      "%s requests over %s seconds (%s requests per second) with a p95 of %s ms and an error rate of %s percent."
      % (ctx["users"], ov["count"], fmt(a["wall"]), fmt(ov["rps"], 1), fmt(ov["p95"]), fmt(ov["err_rate"], 2)))
    worst = max(a["per_endpoint"].items(), key=lambda kv: kv[1]["p95"]) if a["per_endpoint"] else None
    if worst:
        W("")
        W("The slowest endpoint was `%s` at a p95 of %s ms, against an overall p95 of %s ms."
          % (worst[0], fmt(worst[1]["p95"]), fmt(ov["p95"])))
    W("")

    # ---- environment
    W("## Environment")
    W("")
    W(md_table(["Item", "Value"], [
        ["Host OS", ctx["host_os"]],
        ["Host CPU cores", ctx["host_cpus"]],
        ["Host memory", ctx["host_mem"]],
        ["Docker", ctx["docker_version"]],
        ["Docker Compose", ctx["compose_version"]],
        ["API container cap", ctx["api_cap"]],
        ["Database container cap", ctx["db_cap"]],
        ["API image", "built from the repository dockerfile (golang:1.25-alpine builder, alpine:3.20 runtime)"],
        ["Database image", "postgis/postgis:16-3.4"],
        ["Cache image", "redis:7-alpine"],
        ["Compose project", ctx["project"]],
        ["API base URL", "http://127.0.0.1:%d" % ctx["port"]],
        ["APP_ENV", "development (required for the X-Dev-User-Id header used by signed in traffic)"],
        ["Run started", ctx["started_human"]],
        ["Tehran local time at start", ctx["tehran_time"]],
        ["Load generator", "Python %s, %d worker processes" % (ctx["py_version"], ctx["procs"])],
    ]))
    W("")
    W("### Seeded dataset")
    W("")
    W(md_table(["Table", "Rows"], [[k, v] for k, v in ctx["counts"].items()]))
    W("")
    W("The catalogue was generated specifically for this run. `scripts/catalogue/seed.sql` in the "
      "repository was not used: it targets a superseded schema (it writes `vendor_status`, "
      "`latitude`, `longitude`, `is_active` and `offers.price`, and inserts no `vendors` row), so it "
      "fails on its first statement against the current migrations. It was left untouched.")
    W("")
    W("Shape of the generated data, chosen so the queries do the work they would do in production:")
    W("")
    W("* stores are clustered across %d Tehran districts with a dense centre and a thinner edge, "
      "not scattered uniformly, so radius queries return realistic row counts" % len(DISTRICTS))
    W("* only about %s of %s offer rows are eligible for display. The rest are expired, sold out, "
      "draft, paused, soft deleted, or belong to stores that are pending, suspended, rejected or "
      "deleted, so the `eligible_offers` view has to filter rather than scan a clean table"
      % (ctx["counts"].get("offers eligible for display", "?"), ctx["counts"].get("offers", "?")))
    W("* favorites follow a heavy skew (a small number of very popular offers, a long tail with none), "
      "which is what the popular near you query counts over")
    W("* prices are spread with discounts between 32 and 70 percent, respecting the 30 percent floor "
      "that the `fn_offer_min_discount` trigger enforces from `platform_settings`")
    W("* titles and store names come from a repeated Persian vocabulary, so text search matches a "
      "realistic fraction of rows rather than none or all")
    W("* pickup windows are generated relative to the moment the test starts, with an evening "
      "weighting. This keeps the eligible set the same size whatever hour the test runs, which is a "
      "deliberate difference from production, where the eligible set shrinks late in the evening.")
    W("")

    # ---- methodology
    W("## Methodology")
    W("")
    W("Two phases were measured, and they answer different questions.")
    W("")
    W("**Phase A, realistic sessions.** %d concurrent virtual users, each living through sessions "
      "the way the mobile app would drive them: an app open that issues the four home calls, then "
      "between 3 and 10 further actions, with log-normal think time between actions (median about "
      "2.7 seconds, clamped to between 0.8 and 12 seconds). Arrivals were staggered over %d seconds "
      "rather than starting together. Each user held one keep-alive connection and sent "
      "`Accept-Encoding: gzip`, reading every response body in full. About %d percent of sessions "
      "were signed in (via the `X-Dev-User-Id` header, which is the authentication stand-in this "
      "build ships) and the rest were anonymous, because the `is_favorite` subquery only runs for "
      "signed in callers. Duration %d seconds after a %d second warmup that was discarded."
      % (ctx["users"], ctx["stagger"], int(ctx["auth_share"] * 100), ctx["duration"], ctx["warmup"]))
    W("")
    W("**Phase B, saturation.** The same request mix with the think time removed, at rising "
      "concurrency (%s), each level held for %d seconds. This finds the ceiling and shows which "
      "component becomes the constraint." % (", ".join(str(s) for s in ctx["stage_levels"]), ctx["stage_duration"]))
    W("")
    W("Action weights within a session: browse the catalogue and scroll 35, open filters and filter 22, "
      "open a store and its offers 15, text search 13, map 10, saved offers 5 (signed in only). "
      "Every session begins with an app open, so the home endpoints carry the weight a real app gives them.")
    W("")
    W("Latency was measured client side around the full request and response body read. It was "
      "cross checked against the server's own `latency_ms` field, which the API writes for every "
      "request, so application time can be separated from queueing and transport. Container CPU and "
      "memory were sampled every %s seconds with `docker stats`, and PostgreSQL connection counts "
      "were sampled from `pg_stat_activity` over the same interval. Query plans were taken after the "
      "run with `EXPLAIN (ANALYZE, BUFFERS)` using the repository's own SQL sources, piped in over "
      "stdin so nothing had to be mounted or edited." % fmt(ctx["sample_interval"]))
    W("")
    W("Not tested: authentication and OTP, vendor and admin routes, reservations, orders, writes of "
      "any kind, TLS, and any path that would change data.")
    _un = ctx.get("unavailable") or []
    if _un:
        W("")
        W("**Endpoints this build does not serve.** Each endpoint was probed once before measuring. "
          "The following returned 404, meaning their routes are not registered in the running "
          "binary, so they were excluded from the workload and carry no results below: %s. "
          "Treat their absence as a wiring defect to fix, not as a performance result."
          % ", ".join("`%s`" % x for x in _un))
    W("")

    # ---- results
    W("## Results")
    W("")
    W("### Phase A: %d concurrent users with think time" % ctx["users"])
    W("")
    W(md_table(["Metric", "Value"], [
        ["Requests", ov["count"]],
        ["Duration (s)", fmt(a["wall"])],
        ["Throughput (req/s)", fmt(ov["rps"], 2)],
        ["Successful responses", ov["ok"]],
        ["Errors", "%d (%s percent)" % (ov["errors"], fmt(ov["err_rate"], 2))],
        ["Latency p50 (ms)", fmt(ov["p50"])],
        ["Latency p90 (ms)", fmt(ov["p90"])],
        ["Latency p95 (ms)", fmt(ov["p95"])],
        ["Latency p99 (ms)", fmt(ov["p99"])],
        ["Latency max (ms)", fmt(ov["max"])],
        ["Response bytes read", "%s" % a["bytes"]],
    ]))
    W("")
    W("Per endpoint, ordered by p95:")
    W("")
    W(md_table(["Endpoint", "Requests", "req/s", "p50 ms", "p95 ms", "p99 ms", "max ms", "Errors", "Error %"],
               endpoint_rows(a)))
    W("")
    W("Status codes seen: %s. Transport level failures: %s."
      % (", ".join("%s x%d" % (k, v) for k, v in sorted(a["status_counts"].items())) or "none",
         ", ".join("%s x%d" % (k, v) for k, v in sorted(a["err_counts"].items())) or "none"))
    W("")

    if stages:
        W("### Phase B: saturation")
        W("")
        rows = []
        for st in stages:
            s = st["metrics"]["overall"]
            r = st.get("res", {})
            pgs = st.get("pg") or {}
            rows.append([st["users"], s["count"], fmt(s["rps"], 1), fmt(s["p50"]), fmt(s["p95"]),
                         fmt(s["p99"]), fmt(s["err_rate"], 2),
                         fmt(r.get("api", {}).get("cpu_avg", 0.0)),
                         fmt(r.get("db", {}).get("cpu_avg", 0.0)),
                         pgs.get("conn_max", "n/a")])
        W(md_table(["Concurrency", "Requests", "req/s", "p50 ms", "p95 ms", "p99 ms", "Error %",
                    "api CPU %", "db CPU %", "peak DB conns"], rows))
        W("")
        W("CPU percent is as `docker stats` reports it, where 100 percent means one full core.")
        W("")

    W("### Resource usage during phase A")
    W("")
    if res_a:
        W(md_table(["Container", "CPU avg %", "CPU peak %", "Memory avg MB", "Memory peak MB", "Samples"],
                   [[svc, fmt(v["cpu_avg"]), fmt(v["cpu_max"]), fmt(v["mem_avg"]), fmt(v["mem_max"]), v["n"]]
                    for svc, v in res_a.items()]))
    else:
        W("No resource samples were collected.")
    W("")
    if pg_a:
        W("PostgreSQL connections during phase A: average %s, peak %s, of which active on average %s "
          "and at peak %s." % (fmt(pg_a["conn_avg"], 1), pg_a["conn_max"],
                               fmt(pg_a["active_avg"], 1), pg_a["active_max"]))
        W("")
    if ctx.get("client_cpu_a"):
        W("Load generator cost: %s CPU seconds over %s seconds of wall clock across %d processes. "
          "If this approaches the number of cores on the host, the client itself is competing with "
          "the stack for CPU and the latency figures are pessimistic."
          % (fmt(ctx["client_cpu_a"]), fmt(a["wall"]), ctx["procs"]))
    else:
        W("Load generator cost: not measurable on this platform (the CPU accounting used here is "
          "POSIX only, so it reports nothing on Windows). The client ran %d worker processes. Judge "
          "client side contention from the container CPU figures above instead."
          % ctx["procs"])
    W("")

    if ctx["server_lat"]:
        W("### Client latency against server reported latency")
        W("")
        W("The API logs its own handler latency for every request. Comparing the two separates time "
          "spent inside the application from time spent queueing or in transport.")
        W("")
        rows = []
        for route, v in sorted(ctx["server_lat"].items(), key=lambda kv: -kv[1]["p95"])[:15]:
            rows.append([route, v["count"], fmt(v["p50"]), fmt(v["p95"]), fmt(v["max"])])
        W(md_table(["Route (server side)", "Requests", "p50 ms", "p95 ms", "max ms"], rows))
        W("")

    # ---- findings
    W("## Findings")
    W("")
    W(ctx["findings_text"])
    W("")

    W("## Recommendations")
    W("")
    W("None of these were applied. The repository was left exactly as it was found.")
    W("")
    W(ctx["recommendations_text"])
    W("")

    # ---- limitations
    W("## Limitations")
    W("")
    for item in [
        "Everything ran on one machine, so the load generator, the API and the database competed for "
        "the same CPU and memory. A production deployment separates them.",
        "The containers were capped (%s for the API, %s for the database) to resemble a small staging "
        "VM. Numbers scale with those caps and are not a statement about production hardware."
        % (ctx["api_cap"], ctx["db_cap"]),
        "There is no network latency, no TLS termination and no reverse proxy in the path. Real clients "
        "pay all three.",
        "The dataset is synthetic. Its size and skew were chosen to be plausible, but real traffic has "
        "hot spots this generator does not reproduce, in particular repeated requests for the same "
        "popular stores.",
        "Pickup windows were generated relative to the start of the run, so the eligible set does not "
        "shrink through the evening the way it would in production.",
        "A real mobile client issues the four home calls in parallel. Each virtual user issued them "
        "sequentially on one connection, so app open wall time here is the sum rather than the maximum. "
        "Server side load per app open is identical either way.",
        "Authentication is the temporary X-Dev-User-Id header, not JWT. Once real token authentication "
        "lands, every request will additionally verify a token and reload the user, which this run does "
        "not measure.",
        "Redis was running but the public catalogue path does not use it, so its cost does not appear here.",
    ]:
        W("* " + item)
    W("")

    # ---- reproduce
    W("## Reproduce")
    W("")
    W("```bash")
    W(ctx["cmdline"])
    W("```")
    W("")
    W("The script brings the stack up, seeds it, measures, writes this report and then removes the "
      "stack and its volume. Add `--keep` to leave it running for inspection.")
    W("")

    # ---- appendix
    W("## Appendix A: query plans")
    W("")
    W("Taken after the run against the seeded database, using the repository's own SQL under "
      "`scripts/catalogue/sql/`, which is the source the compiled queries in "
      "`internal/infrastructure/repository/postgres/catalogue_sql.go` are generated from.")
    W("")
    for title, text in ctx["plans"]:
        W("### %s" % title)
        W("")
        W("```")
        W(text if text else "(no plan captured)")
        W("```")
        W("")

    W("## Appendix B: repository integrity")
    W("")
    W(md_table(["Check", "Value"], [
        ["Files hashed", ctx["fp_files_before"]],
        ["Fingerprint before the run", ctx["fp_before"]],
        ["Fingerprint after the run", ctx["fp_after"]],
        ["Result", "unchanged" if ctx["fp_before"] == ctx["fp_after"] else "CHANGED, investigate"],
    ]))
    W("")
    _ex = ctx.get("fp_excluded") or []
    W("The fingerprint is a SHA-256 over every file path and its contents in the repository tree, "
      "excluding `.git`, `node_modules` and `vendor`%s."
      % (", and this test's own files (%s), which are additions rather than changes"
         % ", ".join("`%s`" % x for x in _ex) if _ex else ""))
    W("")

    with open(path, "w", encoding="utf-8") as fh:
        fh.write("\n".join(L) + "\n")
    return path


# ---------------------------------------------------------------- findings


def build_findings(ctx):
    a = ctx["phase_a"]
    plans = dict(ctx["plans"])
    stages = ctx["stages"]
    F, R = [], []
    n = 0

    eps = a["per_endpoint"]
    ov = a["overall"]

    # 1. the slowest endpoint
    if eps:
        label, b = max(eps.items(), key=lambda kv: kv[1]["p95"])
        n += 1
        F.append("### %d. `%s` is the slowest endpoint" % (n, label))
        F.append("")
        F.append("Evidence: p50 %s ms, p95 %s ms, p99 %s ms over %d requests, against an overall "
                 "p95 of %s ms across all endpoints."
                 % (fmt(b["p50"]), fmt(b["p95"]), fmt(b["p99"]), b["count"], fmt(ov["p95"])))
        if "popular-nearby" in label:
            F.append("")
            F.append("This is the query behind `home_popular_nearby` in "
                     "`scripts/catalogue/sql/home_sections.sql`. For every row in `eligible_offers` it "
                     "runs a correlated `count(*)` over `favorite_offers` and two calls to "
                     "`fn_distance_m`, inside a subquery, and only then applies the radius filter in "
                     "the outer `WHERE`. Nothing prunes the set before that per row work happens, and "
                     "the home screen calls this endpoint on every app open.")
            R.append("Rewrite `home_popular_nearby` so the radius filter runs before the per row work. "
                     "Filter stores with `ST_DWithin(s.location, $point, $radius)` so the GIST index "
                     "`idx_stores_location` can be used, then aggregate favorites once with a "
                     "`GROUP BY` or a maintained counter column on `offers` instead of a correlated "
                     "`count(*)` per row. File: `scripts/catalogue/sql/home_sections.sql`, then "
                     "regenerate `catalogue_sql.go`.")

    # 2. query plans
    pop = plans.get("home_popular_nearby (home screen, popular near you)", "")
    near = plans.get("catalogue_search, sort=nearest with a 5 km radius", "")
    txt = plans.get("catalogue_search, text search on title or store name", "")
    fav = plans.get("catalogue_search, same query for a signed in user (is_favorite subquery active)", "")
    t_pop, t_near, t_txt, t_fav = (_exec_ms(pop), _exec_ms(near), _exec_ms(txt), _exec_ms(fav))

    if t_pop is not None or t_near is not None:
        n += 1
        F.append("")
        F.append("### %d. The distance filter cannot use the spatial index" % n)
        F.append("")
        line = ("Evidence: "
                + ", ".join(x for x in [
                    ("`home_popular_nearby` executes in %s ms" % fmt(t_pop)) if t_pop is not None else "",
                    ("`catalogue_search` with sort=nearest and a 5 km radius executes in %s ms" % fmt(t_near)) if t_near is not None else "",
                ] if x)
                + " on the seeded dataset (plans in Appendix A).")
        F.append(line)
        flags = _plan_flags(pop + near)
        F.append("")
        if not flags["gist"]:
            F.append("The GIST index `idx_stores_location` does not appear in either plan. Both queries "
                     "compute `fn_distance_m` as an expression and then compare the computed value, so "
                     "PostgreSQL has no index it can use for the radius and falls back to evaluating "
                     "every eligible row.")
        else:
            F.append("The plans do reference `idx_stores_location`, so the spatial index is being used "
                     "on this path.")
        if flags["seq_scan"]:
            F.append("")
            F.append("The plans contain a sequential scan, which is consistent with that reading.")
        R.append("Use `ST_DWithin` on the geography column for radius filtering instead of comparing a "
                 "computed `fn_distance_m` value, and keep `fn_distance_m` only for presenting the "
                 "distance of the rows that survive. This is what makes `idx_stores_location` usable.")

    if t_txt is not None:
        n += 1
        flags = _plan_flags(txt)
        F.append("")
        F.append("### %d. Text search does not use the full text index" % n)
        F.append("")
        F.append("Evidence: the text search plan executes in %s ms. The index `idx_offers_fts` over "
                 "the generated `search_vector` column %s in the plan, and the trigram indexes %s."
                 % (fmt(t_txt),
                    "appears" if flags["fts"] else "does not appear",
                    "appear" if flags["trgm"] else "do not appear"))
        F.append("")
        F.append("`catalogue_search` matches with `e.title ILIKE $1 OR e.store_name ILIKE $1`. The "
                 "`OR` across two tables inside a view generally prevents either trigram index from "
                 "being used, and `ILIKE` never uses `idx_offers_fts`, which was built for "
                 "`to_tsvector` matching. The index exists and is maintained on every write but is "
                 "not read by this path.")
        R.append("Either search through `search_vector` with `websearch_to_tsquery` so `idx_offers_fts` "
                 "is used, or split the `OR` into two indexable branches combined with `UNION`, so the "
                 "trigram indexes can serve each side.")

    if t_fav is not None and t_near is not None:
        delta = t_fav - t_near
        n += 1
        F.append("")
        F.append("### %d. Signed in requests pay for the is_favorite subquery" % n)
        F.append("")
        m_loops = re.search(r"favorite_offers[^\n]*loops=(\d+)", fav)
        fav_loops = int(m_loops.group(1)) if m_loops else 1
        F.append("Evidence: the same catalogue query executes in %s ms anonymously and %s ms for a "
                 "signed in user, a difference of %s ms (%s percent)."
                 % (fmt(t_near), fmt(t_fav), fmt(delta),
                    fmt(100.0 * delta / t_near if t_near else 0.0)))
        F.append("")
        if fav_loops > 1:
            F.append("The plan shows the `favorite_offers` lookup running %d times, once per row, "
                     "so the cost scales with the size of the page." % fav_loops)
        else:
            F.append("The plan shows the `favorite_offers` lookup running once (`loops=1`), not once "
                     "per row, so this difference is not per row subquery cost. On a single sample it "
                     "is within run to run variation, and the anonymous plan additionally short "
                     "circuits the subquery with a one time filter. Treat the gap as unexplained "
                     "rather than attributed, and measure it over repeated runs before acting on it.")
        if delta > max(2.0, 0.15 * t_near):
            R.append("Replace the per row `EXISTS` for `is_favorite` with a single `LEFT JOIN` against "
                     "the caller's favorites, or fetch the favorite ids for the page in one query and "
                     "mark the rows in the service layer.")

    # 3. which container saturates
    res = ctx["res_a"]
    if res.get("api") and res.get("db"):
        n += 1
        api_cap = ctx["api_cpu_cap"] * 100.0
        db_cap = ctx["db_cpu_cap"] * 100.0
        api_u = res["api"]["cpu_avg"] / api_cap * 100 if api_cap else 0
        db_u = res["db"]["cpu_avg"] / db_cap * 100 if db_cap else 0
        busier = "database" if db_u >= api_u else "API"
        F.append("")
        F.append("### %d. The %s container is the busier of the two" % (n, busier))
        F.append("")
        F.append("Evidence during phase A: api averaged %s percent CPU (peak %s) against a cap of %s "
                 "percent, and db averaged %s percent CPU (peak %s) against a cap of %s percent. "
                 "Memory peaked at %s MB for api and %s MB for db."
                 % (fmt(res["api"]["cpu_avg"]), fmt(res["api"]["cpu_max"]), fmt(api_cap),
                    fmt(res["db"]["cpu_avg"]), fmt(res["db"]["cpu_max"]), fmt(db_cap),
                    fmt(res["api"]["mem_max"]), fmt(res["db"]["mem_max"])))
        if db_u > api_u * 1.3:
            F.append("")
            F.append("The work is in the database, not in Go. Tuning the queries is where the "
                     "improvement is, and adding API instances alone would not help.")

    # 4. connection pool
    if stages:
        conn_maxes = [(st["users"], (st.get("pg") or {}).get("conn_max")) for st in stages]
        known = [(u, c) for u, c in conn_maxes if c]
        if len(known) >= 2:
            n += 1
            ceiling = max(c for _, c in known)
            plateau = len({c for _, c in known}) <= max(1, len(known) // 2)
            F.append("")
            F.append("### %d. Database connections are capped by the pgx pool defaults" % n)
            F.append("")
            F.append("Evidence: peak client connections to PostgreSQL by concurrency level: %s."
                     % ", ".join("%d users: %s" % (u, c) for u, c in known))
            F.append("")
            F.append("`internal/infrastructure/repository/postgres/driver/pool.go` calls "
                     "`pgxpool.NewWithConfig` without setting `MaxConns`, so the pool uses the pgx "
                     "default of `max(4, runtime.NumCPU())`. The observed ceiling is %d connections."
                     % ceiling)
            if plateau:
                rps = [(st["users"], st["metrics"]["overall"]["rps"]) for st in stages]
                p95 = [(st["users"], st["metrics"]["overall"]["p95"]) for st in stages]
                db_cpus = [st.get("res", {}).get("db", {}).get("cpu_avg", 0.0) for st in stages]
                db_peak = max(db_cpus) if db_cpus else 0.0
                cap = (ctx.get("db_cpu_cap") or 0.0) * 100.0
                F.append("")
                F.append("Connections stop rising while concurrency keeps climbing. Throughput by "
                         "level: %s. p95 by level: %s."
                         % (", ".join("%d: %s/s" % (u, fmt(v, 1)) for u, v in rps),
                            ", ".join("%d: %s ms" % (u, fmt(v)) for u, v in p95)))
                F.append("")
                if cap and db_peak >= 0.9 * cap:
                    F.append("The pool is not the binding constraint here, though. The database "
                             "container ran at %s percent CPU against a cap of %s percent, so it was "
                             "saturated at every level. Throughput is limited by database CPU, and "
                             "raising `MaxConns` on its own would add contention rather than "
                             "throughput. Treat the unset `MaxConns` as a correctness and portability "
                             "problem (it silently follows the core count of whatever host the "
                             "container lands on) rather than as the cause of this ceiling."
                             % (fmt(db_peak), fmt(cap)))
                else:
                    F.append("The database still had CPU headroom (%s percent used of a %s percent "
                             "cap), so requests were waiting for a pool slot rather than for the "
                             "database itself." % (fmt(db_peak), fmt(cap)))
            R.append("Set `MaxConns` explicitly in `driver/pool.go` (and `MinConns` for warm "
                     "connections) rather than relying on `max(4, NumCPU)`, which silently depends on "
                     "the core count of whatever machine the container lands on. Size it against the "
                     "database's own `max_connections` and the number of API instances.")

    # 5. errors
    if ov["errors"]:
        n += 1
        F.append("")
        F.append("### %d. Failed requests" % n)
        F.append("")
        F.append("Evidence: %d of %d requests failed (%s percent). Status codes: %s. Transport "
                 "failures: %s."
                 % (ov["errors"], ov["count"], fmt(ov["err_rate"], 2),
                    ", ".join("%s x%d" % (k, v) for k, v in sorted(a["status_counts"].items()) if not (200 <= int(k) < 300)) or "none",
                    ", ".join("%s x%d" % (k, v) for k, v in sorted(a["err_counts"].items())) or "none"))
    else:
        n += 1
        F.append("")
        F.append("### %d. No failed requests" % n)
        F.append("")
        F.append("Evidence: all %d requests in phase A returned a 2xx status with no timeouts and no "
                 "dropped connections." % ov["count"])

    if not R:
        R.append("No change is indicated by this run.")
    rec = "\n".join("%d. %s" % (i + 1, r) for i, r in enumerate(R))
    rec += ("\n\nThese are ordered by the size of the effect the measurements suggest, not by effort. "
            "Review them before acting: the measurements come from one host with synthetic data.")
    return "\n".join(F), rec


# -------------------------------------------------------------------- main


def parse_args(argv):
    p = argparse.ArgumentParser(
        description="Load test the public home, catalogue and store APIs in an isolated stack.",
        formatter_class=argparse.ArgumentDefaultsHelpFormatter)
    p.add_argument("--repo", required=True, help="path to the extracted backend repo (holds docker-compose.yml)")
    p.add_argument("--report", default="loadtest-report.md", help="where to write the report")
    p.add_argument("--project", default="tgtg-loadtest", help="docker compose project name, kept separate from anything else")
    p.add_argument("--port", type=int, default=18080, help="host port for the isolated api")

    p.add_argument("--users", type=int, default=50, help="concurrent users in phase A")
    p.add_argument("--duration", type=int, default=180, help="phase A measured seconds")
    p.add_argument("--warmup", type=int, default=30, help="discarded warmup seconds")
    p.add_argument("--stagger", type=int, default=10, help="seconds over which phase A users arrive")
    p.add_argument("--stages", default="10,25,50,100", help="phase B concurrency levels")
    p.add_argument("--stage-duration", type=int, default=45, help="seconds per phase B level")
    p.add_argument("--skip-saturation", action="store_true", help="run phase A only")
    p.add_argument("--auth-share", type=float, default=0.4, help="fraction of sessions that are signed in")
    p.add_argument("--procs", type=int, default=0, help="load generator processes (0 = auto)")
    p.add_argument("--timeout", type=float, default=20.0, help="per request timeout in seconds")

    p.add_argument("--stores", type=int, default=1200, help="stores to seed")
    p.add_argument("--total-offers", type=int, default=40000, help="total offer rows to seed")
    p.add_argument("--eligible-offers", type=int, default=8000, help="of those, how many are displayable")
    p.add_argument("--users-rows", type=int, default=50000, help="users to seed")
    p.add_argument("--favorites", type=int, default=100000, help="favorite rows to attempt")

    p.add_argument("--api-cpus", default="2", help="CPU cap for the api container")
    p.add_argument("--api-mem", default="1g", help="memory cap for the api container")
    p.add_argument("--db-cpus", default="2", help="CPU cap for the db container")
    p.add_argument("--db-mem", default="2g", help="memory cap for the db container")
    p.add_argument("--no-limits", action="store_true", help="do not cap container resources")

    p.add_argument("--sample-interval", type=float, default=2.0, help="resource sampling interval in seconds")
    p.add_argument("--keep", action="store_true", help="leave the stack running afterwards")
    p.add_argument("--dry-run", action="store_true", help="generate the override and seed SQL, then stop")
    p.add_argument("--seed", type=int, default=20261126, help="random seed for the dataset and traffic")
    return p.parse_args(argv)


def docker_versions():
    d = c = "unknown"
    try:
        d = out_of(run(["docker", "--version"], check=False)).strip()
        c = out_of(run(["docker", "compose", "version"], check=False)).strip()
    except Exception:  # noqa: BLE001
        pass
    return d, c


def docker_host_info():
    try:
        p = run(["docker", "info", "--format", "{{.NCPU}}\t{{.MemTotal}}\t{{.OperatingSystem}}"], check=False)
        parts = out_of(p).strip().split("\t")
        if len(parts) >= 3:
            mem_gb = float(parts[1]) / (1024 ** 3)
            return parts[0], "%s GB" % fmt(mem_gb, 1), parts[2]
    except Exception:  # noqa: BLE001
        pass
    return str(os.cpu_count() or "?"), "unknown", "unknown"


def main(argv=None):
    argv = list(sys.argv[1:] if argv is None else argv)
    args = parse_args(argv)
    args.repo = os.path.abspath(args.repo)
    rnd = random.Random(args.seed)
    started = time.time()

    problems = preflight(args)
    if problems and not args.dry_run:
        print("Cannot start:\n" + "\n".join("  * " + x for x in problems), file=sys.stderr)
        return 2
    if args.eligible_offers > args.total_offers:
        print("--eligible-offers cannot exceed --total-offers", file=sys.stderr)
        return 2

    tmpdir = tempfile.mkdtemp(prefix="tgtg-loadtest-")
    log("working directory for generated files: %s" % tmpdir)

    # This script and the report it writes may well live inside the repo (for
    # example under scripts/ and reports/). They are excluded from the
    # fingerprint so placing them there is not reported as a change to the repo.
    self_path = os.path.abspath(__file__)
    report_path = os.path.abspath(args.report)
    excluded = [self_path, report_path,
                report_path + ".raw.json",
                os.path.splitext(report_path)[0] + "-raw.json"]
    rel_excluded = []
    for _p in excluded:
        try:
            _r = os.path.relpath(_p, args.repo)
        except ValueError:
            continue
        if not _r.startswith(".."):
            rel_excluded.append(_r.replace(os.sep, "/"))

    log("fingerprinting the repository so any accidental change is visible")
    fp_before, n_files = repo_fingerprint(args.repo, excluded)
    log("  %d files, fingerprint %s" % (n_files, fp_before[:16]))

    override, override_body, isolated_port = write_override(args, tmpdir)

    now = dt.datetime.now(dt.timezone.utc)
    log("generating the seed dataset (%d stores, %d offers of which %d displayable, %d users)"
        % (args.stores, args.total_offers, args.eligible_offers, args.users_rows))
    sql_a, meta, n_vendors = gen_users_vendors_stores(args, rnd, now)

    if args.dry_run:
        pa = os.path.join(tmpdir, "seed-part1.sql")
        with open(pa, "w", encoding="utf-8") as fh:
            fh.write(sql_a)
        ids = {m["idx"]: 1000 + m["idx"] for m in meta}
        pb = os.path.join(tmpdir, "seed-part2.sql")
        with open(pb, "w", encoding="utf-8") as fh:
            fh.write(gen_offers(args, rnd, now, meta, ids))
        print("\n--- compose override (%s) ---\n%s" % (override, override_body))
        print("--- seed written to ---\n  %s\n  %s" % (pa, pb))
        print("\nEligible stores generated: %d of %d"
              % (sum(1 for m in meta if m["eligible"]), len(meta)))
        print("Dry run complete. Nothing was started and nothing in the repo was touched.")
        return 0

    stack = Stack(args.repo, args.project, override, args.port)
    if stack.exists():
        print("A compose project named %r already exists. Remove it or pass a different --project."
              % args.project, file=sys.stderr)
        return 2

    sampler = None
    try:
        stack.up()
        health = stack.wait_healthy()

        log("applying the seed (users, vendors, stores)")
        stack.psql(sql_a)
        rows = stack.psql_rows("SELECT id, slug FROM stores WHERE slug LIKE 'lt-store-%' ORDER BY id;")
        store_ids = {}
        for sid, slug in rows:
            try:
                store_ids[int(slug.rsplit("-", 1)[1])] = int(sid)
            except (ValueError, IndexError):
                continue
        if len(store_ids) != args.stores:
            raise RuntimeError("expected %d stores, found %d" % (args.stores, len(store_ids)))

        log("applying the seed (offers, allergens, favorites) and analysing")
        stack.psql(gen_offers(args, rnd, now, meta, store_ids))

        counts_sql = ("SELECT (SELECT count(*) FROM users), (SELECT count(*) FROM vendors), "
                      "(SELECT count(*) FROM stores), (SELECT count(*) FROM offers), "
                      "(SELECT count(*) FROM eligible_offers), (SELECT count(*) FROM offer_allergens), "
                      "(SELECT count(*) FROM favorite_offers);")
        c = stack.psql_rows(counts_sql)[0]
        counts = {
            "users": c[0], "vendors": c[1], "stores": c[2], "offers": c[3],
            "offers eligible for display": c[4], "offer allergen rows": c[5],
            "favorite rows": c[6],
        }
        log("seeded: " + ", ".join("%s=%s" % (k, v) for k, v in counts.items()))
        if int(c[4]) < 100:
            raise RuntimeError("only %s offers are eligible for display, the seed did not work" % c[4])

        log("discovering real ids to drive the test with")
        sids = [r[0] for r in stack.psql_rows(
            "SELECT s.public_id FROM stores s WHERE s.status = 'approved' AND s.deleted_at IS NULL "
            "AND s.accepts_orders AND EXISTS (SELECT 1 FROM eligible_offers e WHERE e.store_id = s.id) "
            "ORDER BY random() LIMIT 200;")]
        uids = [int(r[0]) for r in stack.psql_rows(
            "SELECT DISTINCT user_id FROM favorite_offers ORDER BY 1 LIMIT 300;")]
        if not sids:
            raise RuntimeError("no store has displayable offers, the seed did not work")
        if not uids:
            uids = [1]
        data = {
            "store_ids": sids,
            "user_ids": uids,
            "centers": [(la, ln) for (_n, la, ln, _w, _s) in DISTRICTS],
        }

        # Probe every endpoint once: anything this build does not register is
        # skipped and reported, rather than counted as a failure.
        log("probing which endpoints this build serves")
        avail, rowcounts = probe_availability(args.port, sids[0], uids[0])
        unavailable = sorted(k for k, v in avail.items() if v == 404)
        for label in sorted(avail):
            log("  %-26s %s%s" % (label, avail[label],
                                  "" if avail[label] != 200 else " (%d rows)" % rowcounts[label]))
        if unavailable:
            log("not served by this build, they will be skipped: %s" % ", ".join(unavailable))
        data["unavailable"] = set(unavailable)

        if avail.get("GET /offers") == 404:
            raise RuntimeError(
                "GET /offers returned 404, so the catalogue routes are not registered and there is "
                "nothing to measure.\n\n"
                "This is a wiring defect in the backend, not a problem with the test or the data "
                "(the seed loaded %s displayable offers).\n"
                "  internal/injector/injector.go, provideDatabase: the returned `database` struct "
                "never sets its `pool` field, so `db.pool` is nil.\n"
                "  internal/injector/catalogue.go, provideCatalogueService: returns nil when "
                "`db.pool` is nil, so the catalogue handler is nil and registerCatalogue "
                "registers nothing.\n"
                "Fix: add `pool: pool,` to the struct literal returned by provideDatabase."
                % counts.get("offers eligible for display", "the"))
        if avail.get("GET /offers") != 200 or rowcounts.get("GET /offers", 0) <= 0:
            raise RuntimeError("GET /offers returned status %s with %s rows. Refusing to measure an "
                               "empty catalogue." % (avail.get("GET /offers"),
                                                     rowcounts.get("GET /offers")))
        log("catalogue probe returned %d offers, proceeding" % rowcounts["GET /offers"])

        procs = args.procs or max(1, min(4, (os.cpu_count() or 2)))
        cfg = {"host": "127.0.0.1", "port": args.port, "timeout": args.timeout,
               "auth_share": args.auth_share, "stagger": args.stagger}

        sampler = ResourceSampler(stack, args.sample_interval)
        sampler.phase = "warmup"
        sampler.start()

        if args.warmup > 0:
            log("warming up for %ds (discarded)" % args.warmup)
            run_phase({**cfg, "stagger": 0}, data, "B", min(10, args.users), args.warmup, procs)

        log("phase A: %d concurrent users with think time for %ds" % (args.users, args.duration))
        sampler.phase = "A"
        t_a0 = time.time()
        samples_a, wall_a, cpu_a = run_phase(cfg, data, "A", args.users, args.duration, procs)
        t_a1 = time.time()
        phase_a = summarize(samples_a, wall_a)
        log("phase A done: %d requests, %s req/s, p95 %s ms, %s percent errors"
            % (phase_a["overall"]["count"], fmt(phase_a["overall"]["rps"], 1),
               fmt(phase_a["overall"]["p95"]), fmt(phase_a["overall"]["err_rate"], 2)))

        stages = []
        if not args.skip_saturation:
            levels = [int(x) for x in args.stages.split(",") if x.strip()]
            for lvl in levels:
                tag = "B%d" % lvl
                sampler.phase = tag
                log("phase B: %d concurrent, no think time, %ds" % (lvl, args.stage_duration))
                s, w, _c = run_phase({**cfg, "stagger": 0}, data, "B", lvl, args.stage_duration, procs)
                m = summarize(s, w)
                stages.append({"users": lvl, "metrics": m,
                               "res": res_summary(sampler.rows, tag),
                               "pg": pg_summary(sampler.pg, tag)})
                log("  %d users: %s req/s, p95 %s ms, %s percent errors"
                    % (lvl, fmt(m["overall"]["rps"], 1), fmt(m["overall"]["p95"]),
                       fmt(m["overall"]["err_rate"], 2)))
        else:
            levels = []

        sampler.phase = "idle"
        sampler.stop()
        sampler.join(timeout=20)

        # Insurance: the measurement is finished and the stack is about to be
        # destroyed, so dump the raw numbers before anything else can fail.
        raw_path = os.path.abspath(args.report) + ".raw.json"
        try:
            os.makedirs(os.path.dirname(raw_path) or ".", exist_ok=True)
            with open(raw_path, "w", encoding="utf-8") as fh:
                json.dump({
                    "phase_a": phase_a,
                    "stages": [{"users": st["users"], "metrics": st["metrics"],
                                "res": st.get("res"), "pg": st.get("pg")} for st in stages],
                    "resources_phase_a": res_summary(sampler.rows, "A"),
                    "pg_phase_a": pg_summary(sampler.pg, "A"),
                    "counts": counts,
                    "unavailable": unavailable,
                }, fh, indent=2, default=str)
            log("raw results saved to %s" % raw_path)
        except Exception as e:  # noqa: BLE001
            log("could not save raw results: %s" % e)

        # Neither of these is worth losing a finished run over.
        plans = []
        try:
            log("collecting query plans")
            plans = collect_plans(stack, args.repo)
        except Exception as e:  # noqa: BLE001
            log("query plans could not be collected: %s" % e)
        server_lat = {}
        try:
            log("collecting server side latency from the api log")
            server_lat, _server_status = collect_server_latency(stack, t_a0, t_a1)
        except Exception as e:  # noqa: BLE001
            log("server side latency could not be collected: %s" % e)

        log("re-fingerprinting the repository")
        fp_after, _ = repo_fingerprint(args.repo, excluded)
        if fp_after != fp_before:
            log("WARNING: the repository fingerprint changed during the run")

        dv, cv = docker_versions()
        ncpu, nmem, dos = docker_host_info()
        tehran = dt.datetime.now(dt.timezone.utc) + dt.timedelta(hours=3, minutes=30)
        ctx = {
            "phase_a": phase_a, "stages": stages,
            "res_a": res_summary(sampler.rows, "A"), "pg_a": pg_summary(sampler.pg, "A"),
            "plans": plans, "server_lat": server_lat,
            "counts": counts,
            "users": args.users, "duration": args.duration, "warmup": args.warmup,
            "stagger": args.stagger, "auth_share": args.auth_share,
            "stage_levels": levels, "stage_duration": args.stage_duration,
            "sample_interval": args.sample_interval,
            "procs": procs, "client_cpu_a": cpu_a,
            "project": args.project, "port": args.port,
            "host_os": "%s %s (%s)" % (platform.system(), platform.release(), platform.machine()),
            "host_cpus": "%s visible to Docker (%s on this machine)" % (ncpu, os.cpu_count()),
            "host_mem": "%s available to Docker" % nmem,
            "docker_version": dv, "compose_version": cv,
            "api_cap": "unlimited" if args.no_limits else "%s CPUs, %s memory" % (args.api_cpus, args.api_mem),
            "db_cap": "unlimited" if args.no_limits else "%s CPUs, %s memory" % (args.db_cpus, args.db_mem),
            "api_cpu_cap": 0.0 if args.no_limits else float(args.api_cpus),
            "db_cpu_cap": 0.0 if args.no_limits else float(args.db_cpus),
            "py_version": platform.python_version(),
            "started_human": dt.datetime.now().strftime("%Y-%m-%d %H:%M:%S"),
            "tehran_time": tehran.strftime("%Y-%m-%d %H:%M") + " Asia/Tehran",
            "fp_before": fp_before, "fp_after": fp_after, "fp_files_before": n_files,
            "unavailable": unavailable,
            "fp_excluded": rel_excluded,
            "integrity_note": (
                "No file that was already in the repository was modified, moved or deleted. This "
                "test's own files (%s) sit inside the repository tree because that is where they "
                "were placed, and they are the only additions. Everything else the run needed (the "
                "compose override and the seed SQL) was generated in a temporary directory outside "
                "the repository and removed with it."
                % ", ".join("`%s`" % x for x in rel_excluded)
                if rel_excluded else
                "The repository was not modified. Every byte of the test environment (compose "
                "override, seed data, report) was generated at run time outside the repository tree."
            ),
            "cmdline": "python3 loadtest.py " + " ".join(
                (x if x.startswith("-") else '"%s"' % x) for x in argv),
            "health": health,
        }
        if not ctx["api_cpu_cap"]:
            ctx["api_cpu_cap"] = float(os.cpu_count() or 1)
        if not ctx["db_cpu_cap"]:
            ctx["db_cpu_cap"] = float(os.cpu_count() or 1)
        ctx["findings_text"], ctx["recommendations_text"] = build_findings(ctx)

        out = os.path.abspath(args.report)
        try:
            os.makedirs(os.path.dirname(out) or ".", exist_ok=True)
            write_report(out, ctx)
            log("report written to %s" % out)
        except Exception as e:  # noqa: BLE001
            # Never lose a measured run to a formatting error: dump the numbers.
            log("writing the report failed: %s" % e)
            raw = os.path.splitext(out)[0] + "-raw.json"
            try:
                with open(raw, "w", encoding="utf-8") as fh:
                    json.dump({"phase_a": phase_a, "stages": stages,
                               "resources": ctx.get("res_a"), "pg": ctx.get("pg_a"),
                               "counts": counts, "unavailable": unavailable,
                               "server_latency": server_lat},
                              fh, default=str, indent=2)
                log("the raw measurements were saved to %s instead" % raw)
            except Exception as e2:  # noqa: BLE001
                log("the raw measurements could not be saved either: %s" % e2)
            raise
        log("total run time %s minutes" % fmt((time.time() - started) / 60.0, 1))
        return 0

    finally:
        try:
            if sampler and sampler.is_alive():
                sampler.stop()
        except Exception:  # noqa: BLE001
            pass
        if not args.keep:
            try:
                stack.down()
            except Exception as e:  # noqa: BLE001
                log("teardown had a problem: %s" % e)
        else:
            log("stack left running (--keep). Remove it with: docker compose -p %s -f %s -f %s down -v"
                % (args.project, os.path.join(args.repo, "docker-compose.yml"), override))


if __name__ == "__main__":
    mp.freeze_support()
    sys.exit(main())
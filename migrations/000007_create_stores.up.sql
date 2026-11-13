-- PostGIS for store locations; pg_trgm for fuzzy name search.
CREATE EXTENSION IF NOT EXISTS postgis;
CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE TYPE store_status AS ENUM ('pending', 'approved', 'rejected', 'suspended');

CREATE TABLE categories (
    id         SMALLINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    parent_id  SMALLINT,
    slug       TEXT     NOT NULL,
    name_fa    TEXT     NOT NULL,
    icon       TEXT,
    sort_order SMALLINT NOT NULL DEFAULT 0,
    is_active  BOOLEAN  NOT NULL DEFAULT true,
    CONSTRAINT categories_slug_key    UNIQUE (slug),
    CONSTRAINT categories_parent_fkey FOREIGN KEY (parent_id) REFERENCES categories(id) ON DELETE SET NULL
);

INSERT INTO categories (slug, name_fa, sort_order) VALUES
    ('restaurant',  'رستوران',        1),
    ('fastfood',    'فست‌فود',        2),
    ('bakery',      'نانوایی',        3),
    ('pastry',      'قنادی',          4),
    ('cafe',        'کافه',           5),
    ('supermarket', 'سوپرمارکت',      6),
    ('fruit',       'میوه و تره‌بار', 7);

-- A vendor is the business/owner entity. Admin approval is per store.
CREATE TABLE vendors (
    id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    owner_user_id   BIGINT       NOT NULL,
    business_name   TEXT         NOT NULL,
    legal_name      TEXT,
    license_number  TEXT,
    contact_phone   TEXT,
    commission_rate NUMERIC(5,2),                          -- NULL => platform default
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT now(),
    CONSTRAINT vendors_owner_fkey     FOREIGN KEY (owner_user_id) REFERENCES users(id),
    CONSTRAINT vendors_commission_chk CHECK (commission_rate BETWEEN 0 AND 100)
);
CREATE INDEX idx_vendors_owner ON vendors(owner_user_id);

CREATE TRIGGER trg_vendors_updated
    BEFORE UPDATE ON vendors
    FOR EACH ROW EXECUTE FUNCTION fn_set_updated_at();

CREATE TABLE stores (
    id               BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    public_id        UUID         NOT NULL DEFAULT gen_random_uuid(),
    vendor_id        BIGINT       NOT NULL,
    category_id      SMALLINT,
    name             TEXT         NOT NULL,
    slug             TEXT         NOT NULL,                 -- SEO-friendly URL
    biography        TEXT         NOT NULL,
    phone            TEXT,
    city             TEXT         NOT NULL,
    district         TEXT,
    address_line     TEXT         NOT NULL,                 -- also the pickup address
    location         GEOGRAPHY(Point, 4326) NOT NULL,
    logo_url         TEXT,
    cover_image_url  TEXT,
    -- admin approval: a new store is 'pending' and invisible until 'approved'
    status           store_status NOT NULL DEFAULT 'pending',
    rejection_reason TEXT,
    approved_by      BIGINT,
    approved_at      TIMESTAMPTZ,
    accepts_orders   BOOLEAN      NOT NULL DEFAULT true,    -- the owner's own pause switch
    -- denormalised aggregates (maintained by triggers / jobs)
    rating_avg       NUMERIC(3,2) NOT NULL DEFAULT 0,
    rating_count     INT          NOT NULL DEFAULT 0,
    trust_score      NUMERIC(5,2) NOT NULL DEFAULT 100,
    meals_saved      INT          NOT NULL DEFAULT 0,
    created_at       TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ  NOT NULL DEFAULT now(),
    deleted_at       TIMESTAMPTZ,
    CONSTRAINT stores_public_id_key UNIQUE (public_id),
    CONSTRAINT stores_slug_key      UNIQUE (slug),
    CONSTRAINT stores_vendor_fkey   FOREIGN KEY (vendor_id)   REFERENCES vendors(id),
    CONSTRAINT stores_category_fkey FOREIGN KEY (category_id) REFERENCES categories(id),
    CONSTRAINT stores_approved_fkey FOREIGN KEY (approved_by) REFERENCES users(id),
    CONSTRAINT stores_trust_chk     CHECK (trust_score BETWEEN 0 AND 100),
    CONSTRAINT stores_biography_chk CHECK (length(btrim(biography)) > 0),
    CONSTRAINT stores_rejection_chk CHECK (status <> 'rejected' OR rejection_reason IS NOT NULL),
    CONSTRAINT stores_approval_chk  CHECK (status <> 'approved' OR (approved_by IS NOT NULL AND approved_at IS NOT NULL))
);
CREATE INDEX idx_stores_location  ON stores USING GIST (location);
CREATE INDEX idx_stores_vendor    ON stores(vendor_id);
CREATE INDEX idx_stores_category  ON stores(category_id) WHERE status = 'approved' AND deleted_at IS NULL;
CREATE INDEX idx_stores_pending   ON stores(created_at) WHERE status = 'pending';   -- admin review queue
CREATE INDEX idx_stores_name_trgm ON stores USING GIN (name gin_trgm_ops);

CREATE TRIGGER trg_stores_updated
    BEFORE UPDATE ON stores
    FOR EACH ROW EXECUTE FUNCTION fn_set_updated_at();

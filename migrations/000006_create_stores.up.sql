-- Mirrors internal/domain/model/store.go (PR #8). Coordinates are plain doubles (haversine in SQL).
CREATE TABLE stores (
    id                  BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    public_id           UUID             NOT NULL DEFAULT gen_random_uuid(),
    vendor_id           BIGINT           NOT NULL REFERENCES users(id),
    vendor_status       TEXT             NOT NULL DEFAULT 'pending_review',
    category_id         SMALLINT         REFERENCES categories(id),
    owner_name          TEXT             NOT NULL DEFAULT '',
    name                TEXT             NOT NULL,
    slug                TEXT             NOT NULL,
    biography           TEXT             NOT NULL DEFAULT '',
    pickup_instructions TEXT             NOT NULL DEFAULT '',
    phone               TEXT,
    city                TEXT             NOT NULL DEFAULT '',
    district            TEXT,
    address_line        TEXT             NOT NULL,
    latitude            DOUBLE PRECISION NOT NULL,
    longitude           DOUBLE PRECISION NOT NULL,
    logo_url            TEXT,
    cover_image_url     TEXT,
    is_active           BOOLEAN          NOT NULL DEFAULT TRUE,
    accepts_orders      BOOLEAN          NOT NULL DEFAULT TRUE,
    rating_avg          NUMERIC(3,2)     NOT NULL DEFAULT 0,
    rating_count        INT              NOT NULL DEFAULT 0,
    trust_score         NUMERIC(5,2)     NOT NULL DEFAULT 0,
    meals_saved         INT              NOT NULL DEFAULT 0,
    created_at          TIMESTAMPTZ      NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ      NOT NULL DEFAULT now(),
    deleted_at          TIMESTAMPTZ,
    CONSTRAINT stores_public_id_key UNIQUE (public_id),
    CONSTRAINT stores_slug_key      UNIQUE (slug),
    CONSTRAINT stores_name_chk      CHECK (btrim(name) <> ''),
    CONSTRAINT stores_lat_chk       CHECK (latitude  BETWEEN -90  AND 90),
    CONSTRAINT stores_lng_chk       CHECK (longitude BETWEEN -180 AND 180),
    CONSTRAINT stores_status_chk    CHECK (vendor_status IN ('pending_review','approved','active','rejected','suspended'))
);
CREATE INDEX idx_stores_vendor ON stores (vendor_id) WHERE deleted_at IS NULL;
CREATE TRIGGER trg_stores_updated BEFORE UPDATE ON stores FOR EACH ROW EXECUTE FUNCTION fn_set_updated_at();

CREATE TABLE store_images (
    id         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    store_id   BIGINT   NOT NULL REFERENCES stores(id) ON DELETE CASCADE,
    url        TEXT     NOT NULL,
    is_primary BOOLEAN  NOT NULL DEFAULT FALSE,
    sort_order SMALLINT NOT NULL DEFAULT 0
);
CREATE INDEX idx_store_images_store ON store_images (store_id, sort_order);

-- day_of_week: 0 = Saturday ... 6 = Friday (same as model.WorkingHour)
CREATE TABLE store_working_hours (
    store_id    BIGINT   NOT NULL REFERENCES stores(id) ON DELETE CASCADE,
    day_of_week SMALLINT NOT NULL CHECK (day_of_week BETWEEN 0 AND 6),
    open_time   TIME,
    close_time  TIME,
    is_closed   BOOLEAN  NOT NULL DEFAULT FALSE,
    PRIMARY KEY (store_id, day_of_week),
    CONSTRAINT swh_times_chk CHECK (is_closed OR (open_time IS NOT NULL AND close_time IS NOT NULL))
);

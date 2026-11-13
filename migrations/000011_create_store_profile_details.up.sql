-- Pickup instructions for store pickup guide.
ALTER TABLE stores
    ADD COLUMN pickup_instructions TEXT NOT NULL DEFAULT '';

-- Weekly working hours per store (0 = Saturday to 6 = Friday).
CREATE TABLE store_working_hours (
    id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    store_id    BIGINT       NOT NULL,
    day_of_week SMALLINT     NOT NULL,
    day_name_fa TEXT         NOT NULL,
    open_time   VARCHAR(5)   NOT NULL,
    close_time  VARCHAR(5)   NOT NULL,
    is_closed   BOOLEAN      NOT NULL DEFAULT false,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT now(),
    CONSTRAINT store_working_hours_store_fkey      FOREIGN KEY (store_id) REFERENCES stores(id) ON DELETE CASCADE,
    CONSTRAINT store_working_hours_day_chk         CHECK (day_of_week BETWEEN 0 AND 6),
    CONSTRAINT store_working_hours_store_day_key   UNIQUE (store_id, day_of_week)
);
CREATE INDEX idx_store_working_hours_store ON store_working_hours(store_id);

CREATE TRIGGER trg_store_working_hours_updated
    BEFORE UPDATE ON store_working_hours
    FOR EACH ROW EXECUTE FUNCTION fn_set_updated_at();

-- Store gallery images with ordering and primary cover indicator.
CREATE TABLE store_images (
    id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    public_id   UUID         NOT NULL DEFAULT gen_random_uuid(),
    store_id    BIGINT       NOT NULL,
    image_url   TEXT         NOT NULL,
    is_primary  BOOLEAN      NOT NULL DEFAULT false,
    sort_order  SMALLINT     NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT now(),
    CONSTRAINT store_images_public_id_key UNIQUE (public_id),
    CONSTRAINT store_images_store_fkey    FOREIGN KEY (store_id) REFERENCES stores(id) ON DELETE CASCADE,
    CONSTRAINT store_images_url_chk       CHECK (length(btrim(image_url)) > 0)
);
CREATE INDEX idx_store_images_store ON store_images(store_id, sort_order);

CREATE TRIGGER trg_store_images_updated
    BEFORE UPDATE ON store_images
    FOR EACH ROW EXECUTE FUNCTION fn_set_updated_at();

-- Offer currency (default: تومان).
ALTER TABLE offers
    ADD COLUMN currency VARCHAR(16) NOT NULL DEFAULT 'تومان';

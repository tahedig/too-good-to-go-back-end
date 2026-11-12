CREATE EXTENSION IF NOT EXISTS pg_trgm;

-- Mirrors internal/domain/model/offer.go (PR #8) plus category. Prices are integers (toman).
CREATE TABLE offers (
    id                 BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    public_id          UUID        NOT NULL DEFAULT gen_random_uuid(),
    store_id           BIGINT      NOT NULL REFERENCES stores(id) ON DELETE CASCADE,
    category_id        SMALLINT    NOT NULL REFERENCES categories(id),
    title              TEXT        NOT NULL,
    description        TEXT        NOT NULL DEFAULT '',
    image_url          TEXT,
    original_price     BIGINT      NOT NULL,
    price              BIGINT      NOT NULL,
    currency           TEXT        NOT NULL DEFAULT 'تومان',
    quantity_available INT         NOT NULL DEFAULT 0,
    pickup_start       TIMESTAMPTZ NOT NULL,
    pickup_end         TIMESTAMPTZ NOT NULL,
    is_active          BOOLEAN     NOT NULL DEFAULT TRUE,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at         TIMESTAMPTZ,
    CONSTRAINT offers_public_id_key UNIQUE (public_id),
    CONSTRAINT offers_title_chk     CHECK (btrim(title) <> ''),
    CONSTRAINT offers_price_chk     CHECK (price >= 0 AND original_price >= price),
    CONSTRAINT offers_qty_chk       CHECK (quantity_available >= 0),
    CONSTRAINT offers_window_chk    CHECK (pickup_end > pickup_start)
);
-- Every offer must have a delivery window (pickup_start/pickup_end NOT NULL above).
CREATE INDEX idx_offers_store    ON offers (store_id)    WHERE deleted_at IS NULL;
CREATE INDEX idx_offers_listing  ON offers (category_id, pickup_end)
    WHERE deleted_at IS NULL AND is_active AND quantity_available > 0;
CREATE INDEX idx_offers_title_trgm ON offers USING gin (title gin_trgm_ops);
CREATE INDEX idx_stores_name_trgm  ON stores USING gin (name  gin_trgm_ops);
CREATE TRIGGER trg_offers_updated BEFORE UPDATE ON offers FOR EACH ROW EXECUTE FUNCTION fn_set_updated_at();

CREATE TABLE offer_allergens (
    offer_id      BIGINT NOT NULL REFERENCES offers(id) ON DELETE CASCADE,
    allergen_code TEXT   NOT NULL REFERENCES allergens(code),
    PRIMARY KEY (offer_id, allergen_code)
);
CREATE INDEX idx_offer_allergens_code ON offer_allergens (allergen_code, offer_id);

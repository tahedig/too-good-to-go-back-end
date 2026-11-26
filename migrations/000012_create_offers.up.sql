CREATE TYPE offer_type    AS ENUM ('surprise_bag', 'specific_item');
CREATE TYPE offer_status  AS ENUM ('draft', 'active', 'sold_out', 'expired', 'cancelled');
CREATE TYPE allergen_type AS ENUM ('gluten', 'nuts', 'peanut', 'milk', 'egg', 'soy', 'fish', 'shellfish', 'sesame');

-- Allergen list validator: no NULL elements, no duplicates (empty list = "no allergens").
CREATE OR REPLACE FUNCTION fn_valid_allergen_list(a allergen_type[]) RETURNS BOOLEAN AS $$
    SELECT a IS NOT NULL
       AND array_position(a, NULL::allergen_type) IS NULL
       AND cardinality(a) = (SELECT count(DISTINCT x) FROM unnest(a) AS x)
$$ LANGUAGE sql IMMUTABLE;

-- Recurring offers ("every day 20:00-21:00"); a job creates one offer per occurrence.
CREATE TABLE offer_templates (
    id                BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    store_id          BIGINT          NOT NULL,
    offer_type        offer_type      NOT NULL,
    title             TEXT            NOT NULL,
    description       TEXT            NOT NULL,
    allergens         allergen_type[] NOT NULL DEFAULT '{}',
    image_url         TEXT,
    category_id       SMALLINT,
    original_price    BIGINT          NOT NULL,
    discount_price    BIGINT          NOT NULL,
    quantity          INT             NOT NULL,
    pickup_start_time TIME            NOT NULL,
    pickup_end_time   TIME            NOT NULL,
    days_of_week      SMALLINT[]      NOT NULL,             -- 0 = Saturday ... 6 = Friday
    valid_from        DATE            NOT NULL DEFAULT current_date,
    valid_until       DATE,
    is_active         BOOLEAN         NOT NULL DEFAULT true,
    created_at        TIMESTAMPTZ     NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ     NOT NULL DEFAULT now(),
    CONSTRAINT offer_templates_store_fkey       FOREIGN KEY (store_id)    REFERENCES stores(id) ON DELETE CASCADE,
    CONSTRAINT offer_templates_category_fkey    FOREIGN KEY (category_id) REFERENCES categories(id),
    CONSTRAINT offer_templates_description_chk  CHECK (length(btrim(description)) >= 10),
    CONSTRAINT offer_templates_allergens_chk    CHECK (fn_valid_allergen_list(allergens)),
    CONSTRAINT offer_templates_original_chk     CHECK (original_price > 0),
    CONSTRAINT offer_templates_discount_chk     CHECK (discount_price >= 0),
    CONSTRAINT offer_templates_price_chk        CHECK (discount_price < original_price),
    CONSTRAINT offer_templates_quantity_chk     CHECK (quantity > 0),
    CONSTRAINT offer_templates_days_chk         CHECK (days_of_week <@ ARRAY[0,1,2,3,4,5,6]::smallint[])
);
CREATE INDEX idx_templates_store ON offer_templates(store_id) WHERE is_active;

CREATE TRIGGER trg_templates_updated
    BEFORE UPDATE ON offer_templates
    FOR EACH ROW EXECUTE FUNCTION fn_set_updated_at();

CREATE TABLE offers (
    id                 BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    public_id          UUID            NOT NULL DEFAULT gen_random_uuid(),
    store_id           BIGINT          NOT NULL,
    template_id        BIGINT,
    offer_type         offer_type      NOT NULL,
    title              TEXT            NOT NULL,
    description        TEXT            NOT NULL,             -- mandatory, esp. for surprise bags
    ai_generated       BOOLEAN         NOT NULL DEFAULT false,
    image_url          TEXT,
    category_id        SMALLINT,
    original_price     BIGINT          NOT NULL,
    discount_price     BIGINT          NOT NULL,
    discount_percent   SMALLINT        GENERATED ALWAYS AS
                         (round((1 - discount_price::numeric / original_price) * 100)::smallint) STORED,
    quantity_total     INT             NOT NULL,
    quantity_available INT             NOT NULL,
    max_per_user       SMALLINT        NOT NULL DEFAULT 2,
    pickup_start       TIMESTAMPTZ     NOT NULL,
    pickup_end         TIMESTAMPTZ     NOT NULL,
    allergens          allergen_type[] NOT NULL DEFAULT '{}',
    allergens_declared BOOLEAN         NOT NULL DEFAULT false,  -- vendor confirmed the list
    status             offer_status    NOT NULL DEFAULT 'draft',
    version            INT             NOT NULL DEFAULT 1,      -- optimistic locking for vendor edits
    search_vector      TSVECTOR        GENERATED ALWAYS AS
                         (to_tsvector('simple', coalesce(title, '') || ' ' || coalesce(description, ''))) STORED,
    published_at       TIMESTAMPTZ,
    created_at         TIMESTAMPTZ     NOT NULL DEFAULT now(),
    updated_at         TIMESTAMPTZ     NOT NULL DEFAULT now(),
    CONSTRAINT offers_public_id_key    UNIQUE (public_id),
    CONSTRAINT offers_id_store_uq      UNIQUE (id, store_id),
    CONSTRAINT offers_store_fkey       FOREIGN KEY (store_id)    REFERENCES stores(id),
    CONSTRAINT offers_template_fkey    FOREIGN KEY (template_id) REFERENCES offer_templates(id) ON DELETE SET NULL,
    CONSTRAINT offers_category_fkey    FOREIGN KEY (category_id) REFERENCES categories(id),
    CONSTRAINT offers_description_chk  CHECK (length(btrim(description)) >= 10),
    CONSTRAINT offers_original_chk     CHECK (original_price > 0),
    CONSTRAINT offers_discount_chk     CHECK (discount_price >= 0),
    CONSTRAINT offers_price_chk        CHECK (discount_price < original_price),
    CONSTRAINT offers_quantity_chk     CHECK (quantity_total > 0),
    CONSTRAINT offers_stock_chk        CHECK (quantity_available BETWEEN 0 AND quantity_total),
    CONSTRAINT offers_max_per_user_chk CHECK (max_per_user > 0),
    CONSTRAINT offers_window_chk       CHECK (pickup_end > pickup_start),
    CONSTRAINT offers_allergens_chk    CHECK (fn_valid_allergen_list(allergens)),
    CONSTRAINT offers_allergen_chk     CHECK (status <> 'active' OR allergens_declared)
);
CREATE INDEX idx_offers_store         ON offers(store_id, pickup_end DESC);
CREATE INDEX idx_offers_active_browse ON offers(category_id, pickup_end) WHERE status = 'active';
CREATE INDEX idx_offers_active_price  ON offers(discount_price)          WHERE status = 'active';
CREATE INDEX idx_offers_active_end    ON offers(pickup_end)              WHERE status IN ('active', 'sold_out');
CREATE INDEX idx_offers_template      ON offers(template_id)             WHERE template_id IS NOT NULL;
CREATE INDEX idx_offers_fts           ON offers USING GIN (search_vector);
CREATE INDEX idx_offers_allergens     ON offers USING GIN (allergens) WHERE status = 'active';
CREATE INDEX idx_offers_title_trgm    ON offers USING GIN (title gin_trgm_ops);
-- prevents the template job from creating the same occurrence twice
CREATE UNIQUE INDEX uq_offers_template_day ON offers(template_id, pickup_start) WHERE template_id IS NOT NULL;

CREATE TRIGGER trg_offers_updated
    BEFORE UPDATE ON offers
    FOR EACH ROW EXECUTE FUNCTION fn_set_updated_at();

-- Minimum discount (anti price-inflation), read from platform_settings.
CREATE OR REPLACE FUNCTION fn_offer_min_discount() RETURNS trigger AS $$
DECLARE
    min_pct INT;
    pct     INT;
BEGIN
    SELECT (value #>> '{}')::int INTO min_pct FROM platform_settings WHERE key = 'min_discount_percent';
    pct := round((1 - NEW.discount_price::numeric / NEW.original_price) * 100);
    IF pct < COALESCE(min_pct, 0) THEN
        RAISE EXCEPTION 'discount % %% is below platform minimum % %%', pct, min_pct
            USING ERRCODE = '23514';
    END IF;
    RETURN NEW;
END $$ LANGUAGE plpgsql;

CREATE TRIGGER trg_offers_min_discount
    BEFORE INSERT OR UPDATE OF original_price, discount_price ON offers
    FOR EACH ROW EXECUTE FUNCTION fn_offer_min_discount();

-- An offer can only be PUBLISHED (draft -> active, or inserted as active) for an
-- approved store. Other transitions are not blocked, so background jobs never
-- fail because a store was suspended meanwhile.
CREATE OR REPLACE FUNCTION fn_offer_requires_approved_store() RETURNS trigger AS $$
BEGIN
    IF NEW.status = 'active' AND (TG_OP = 'INSERT' OR OLD.status = 'draft') THEN
        IF NOT EXISTS (SELECT 1 FROM stores
                        WHERE id = NEW.store_id AND status = 'approved' AND deleted_at IS NULL) THEN
            RAISE EXCEPTION 'store is not approved' USING ERRCODE = '23514';
        END IF;
    END IF;
    RETURN NEW;
END $$ LANGUAGE plpgsql;

CREATE TRIGGER trg_offers_store_approved
    BEFORE INSERT OR UPDATE OF status ON offers
    FOR EACH ROW EXECUTE FUNCTION fn_offer_requires_approved_store();

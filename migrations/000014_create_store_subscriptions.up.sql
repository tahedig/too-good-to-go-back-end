-- Seller subscription plans and per-store subscription records.
CREATE TYPE subscription_status AS ENUM ('active', 'pending_review', 'expired', 'cancelled');

CREATE TABLE subscription_plans (
    id              SMALLINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    code            TEXT          NOT NULL,
    name_fa         TEXT          NOT NULL,
    commission_rate NUMERIC(5,2)  NOT NULL,
    monthly_price   BIGINT        NOT NULL DEFAULT 0,
    is_active       BOOLEAN       NOT NULL DEFAULT true,
    created_at      TIMESTAMPTZ   NOT NULL DEFAULT now(),
    CONSTRAINT subscription_plans_code_key       UNIQUE (code),
    CONSTRAINT subscription_plans_commission_chk CHECK (commission_rate BETWEEN 0 AND 100),
    CONSTRAINT subscription_plans_price_chk      CHECK (monthly_price >= 0)
);

INSERT INTO subscription_plans (code, name_fa, commission_rate, monthly_price) VALUES
    ('basic_free', 'پایه - رایگان', 10.00, 0),
    ('pro',        'حرفه‌ای',        7.00,  500000);

CREATE TABLE store_subscriptions (
    id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    public_id       UUID                NOT NULL DEFAULT gen_random_uuid(),
    store_id        BIGINT              NOT NULL,
    plan_id         SMALLINT            NOT NULL,
    upgrade_plan_id SMALLINT,
    status          subscription_status NOT NULL DEFAULT 'active',
    commission_rate NUMERIC(5,2)        NOT NULL,
    period_start    TIMESTAMPTZ         NOT NULL,
    period_end      TIMESTAMPTZ         NOT NULL,
    created_at      TIMESTAMPTZ         NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ         NOT NULL DEFAULT now(),
    CONSTRAINT store_subscriptions_public_id_key   UNIQUE (public_id),
    CONSTRAINT store_subscriptions_store_fkey      FOREIGN KEY (store_id)        REFERENCES stores(id) ON DELETE CASCADE,
    CONSTRAINT store_subscriptions_plan_fkey       FOREIGN KEY (plan_id)         REFERENCES subscription_plans(id),
    CONSTRAINT store_subscriptions_upgrade_fkey    FOREIGN KEY (upgrade_plan_id) REFERENCES subscription_plans(id),
    CONSTRAINT store_subscriptions_commission_chk  CHECK (commission_rate BETWEEN 0 AND 100),
    CONSTRAINT store_subscriptions_period_chk      CHECK (period_end > period_start)
);

CREATE INDEX idx_store_subscriptions_store ON store_subscriptions(store_id, created_at DESC, id DESC);

CREATE TRIGGER trg_store_subscriptions_updated
    BEFORE UPDATE ON store_subscriptions
    FOR EACH ROW EXECUTE FUNCTION fn_set_updated_at();

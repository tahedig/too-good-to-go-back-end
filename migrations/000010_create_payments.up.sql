CREATE TYPE payment_status AS ENUM ('initiated', 'succeeded', 'failed');
CREATE TYPE refund_status  AS ENUM ('pending', 'succeeded', 'failed');

CREATE TABLE payments (
    id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    order_id        BIGINT         NOT NULL,
    provider        TEXT           NOT NULL,              -- internal_sandbox, zarinpal, ...
    amount          BIGINT         NOT NULL,
    status          payment_status NOT NULL DEFAULT 'initiated',
    authority       TEXT,                                 -- gateway token
    provider_ref    TEXT,                                 -- final reference id
    card_pan_masked TEXT,
    error_code      TEXT,
    raw_response    JSONB,
    paid_at         TIMESTAMPTZ,
    created_at      TIMESTAMPTZ    NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ    NOT NULL DEFAULT now(),
    CONSTRAINT payments_provider_authority_uq UNIQUE (provider, authority),
    CONSTRAINT payments_order_fkey            FOREIGN KEY (order_id) REFERENCES orders(id),
    CONSTRAINT payments_amount_chk            CHECK (amount > 0)
);
-- the latest attempt per order is what "My orders" shows
CREATE INDEX idx_payments_order ON payments(order_id, created_at DESC);
-- at most ONE successful payment per order (double-charge guard)
CREATE UNIQUE INDEX uq_payments_one_success ON payments(order_id) WHERE status = 'succeeded';

CREATE TRIGGER trg_payments_updated
    BEFORE UPDATE ON payments
    FOR EACH ROW EXECUTE FUNCTION fn_set_updated_at();

CREATE TABLE refunds (
    id           BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    payment_id   BIGINT        NOT NULL,
    order_id     BIGINT        NOT NULL,
    amount       BIGINT        NOT NULL,
    reason       TEXT          NOT NULL,
    initiated_by actor_type    NOT NULL,
    status       refund_status NOT NULL DEFAULT 'pending',
    provider_ref TEXT,
    requested_by BIGINT,
    processed_at TIMESTAMPTZ,
    created_at   TIMESTAMPTZ   NOT NULL DEFAULT now(),
    CONSTRAINT refunds_payment_fkey   FOREIGN KEY (payment_id)   REFERENCES payments(id),
    CONSTRAINT refunds_order_fkey     FOREIGN KEY (order_id)     REFERENCES orders(id),
    CONSTRAINT refunds_requested_fkey FOREIGN KEY (requested_by) REFERENCES users(id),
    CONSTRAINT refunds_amount_chk     CHECK (amount > 0)
);
CREATE INDEX idx_refunds_order   ON refunds(order_id);
CREATE INDEX idx_refunds_payment ON refunds(payment_id);
CREATE INDEX idx_refunds_pending ON refunds(created_at) WHERE status = 'pending';

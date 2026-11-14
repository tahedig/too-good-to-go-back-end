CREATE TYPE order_status AS ENUM ('pending_payment', 'paid', 'ready', 'completed', 'cancelled', 'expired', 'no_show');
CREATE TYPE actor_type   AS ENUM ('customer', 'vendor', 'system', 'admin');

CREATE TABLE orders (
    id                     BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    public_id              UUID         NOT NULL DEFAULT gen_random_uuid(),
    user_id                BIGINT       NOT NULL,
    store_id               BIGINT       NOT NULL,
    address_id             BIGINT       NOT NULL,
    -- address snapshot: filled by trigger from address_id, unaffected by later edits
    delivery_street_name   TEXT         NOT NULL,
    delivery_plaque_number TEXT         NOT NULL,
    delivery_unit_number   TEXT         NOT NULL,
    status                 order_status NOT NULL DEFAULT 'pending_payment',
    idempotency_key        TEXT,
    -- money snapshot (never recomputed from live prices), in Toman
    items_total            BIGINT       NOT NULL,
    service_fee            BIGINT       NOT NULL DEFAULT 0,
    total_amount           BIGINT       NOT NULL,
    commission_rate        NUMERIC(5,2) NOT NULL,
    commission_amount      BIGINT       NOT NULL,
    vendor_payout          BIGINT       NOT NULL,
    -- pickup
    pickup_code            CHAR(4)      NOT NULL,
    qr_token               TEXT         NOT NULL,
    reserved_until         TIMESTAMPTZ  NOT NULL,          -- payment deadline
    confirmed_by           BIGINT,                         -- staff who scanned the code
    -- lifecycle timestamps (set by trigger)
    paid_at                TIMESTAMPTZ,
    ready_at               TIMESTAMPTZ,
    completed_at           TIMESTAMPTZ,
    cancelled_at           TIMESTAMPTZ,
    cancelled_by           actor_type,
    cancel_reason          TEXT,
    created_at             TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at             TIMESTAMPTZ  NOT NULL DEFAULT now(),
    CONSTRAINT orders_public_id_key     UNIQUE (public_id),
    CONSTRAINT orders_qr_token_key      UNIQUE (qr_token),
    CONSTRAINT orders_id_store_uq       UNIQUE (id, store_id),
    CONSTRAINT orders_id_user_store_uq  UNIQUE (id, user_id, store_id),
    CONSTRAINT orders_idem_uq           UNIQUE (user_id, idempotency_key),
    CONSTRAINT orders_user_fkey         FOREIGN KEY (user_id)      REFERENCES users(id),
    CONSTRAINT orders_store_fkey        FOREIGN KEY (store_id)     REFERENCES stores(id),
    CONSTRAINT orders_address_fkey      FOREIGN KEY (address_id)   REFERENCES user_addresses(id),
    CONSTRAINT orders_confirmed_fkey    FOREIGN KEY (confirmed_by) REFERENCES users(id),
    CONSTRAINT orders_items_total_chk   CHECK (items_total >= 0),
    CONSTRAINT orders_service_fee_chk   CHECK (service_fee >= 0),
    CONSTRAINT orders_total_amount_chk  CHECK (total_amount >= 0),
    CONSTRAINT orders_commission_chk    CHECK (commission_rate BETWEEN 0 AND 100),
    CONSTRAINT orders_commission_amt_chk CHECK (commission_amount >= 0),
    CONSTRAINT orders_vendor_payout_chk CHECK (vendor_payout >= 0),
    CONSTRAINT orders_pickup_code_chk   CHECK (pickup_code ~ '^[0-9]{4}$'),
    CONSTRAINT orders_total_chk         CHECK (total_amount = items_total + service_fee),
    CONSTRAINT orders_payout_chk        CHECK (vendor_payout + commission_amount <= items_total)
);
-- "My orders": newest first, keyset-paginated on (created_at, public_id).
CREATE INDEX idx_orders_user         ON orders(user_id, created_at DESC, public_id DESC);
CREATE INDEX idx_orders_address      ON orders(address_id);
CREATE INDEX idx_orders_store_status ON orders(store_id, status, created_at DESC);
CREATE INDEX idx_orders_created      ON orders(created_at);
CREATE INDEX idx_orders_expiry       ON orders(reserved_until) WHERE status = 'pending_payment';
-- a pickup code must be unique among the store's live orders only
CREATE UNIQUE INDEX uq_orders_live_code ON orders(store_id, pickup_code)
    WHERE status IN ('pending_payment', 'paid', 'ready');

CREATE TABLE order_items (
    id                  BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    order_id            BIGINT NOT NULL,
    store_id            BIGINT NOT NULL,
    offer_id            BIGINT NOT NULL,
    title_snapshot      TEXT   NOT NULL,
    quantity            INT    NOT NULL,
    unit_original_price BIGINT NOT NULL,
    unit_price          BIGINT NOT NULL,
    line_total          BIGINT GENERATED ALWAYS AS (unit_price * quantity) STORED,
    CONSTRAINT order_items_order_offer_uq UNIQUE (order_id, offer_id),
    -- composite FKs guarantee the order and the offer belong to the SAME store
    CONSTRAINT order_items_order_fkey FOREIGN KEY (order_id, store_id) REFERENCES orders(id, store_id) ON DELETE CASCADE,
    CONSTRAINT order_items_offer_fkey FOREIGN KEY (offer_id, store_id) REFERENCES offers(id, store_id),
    CONSTRAINT order_items_quantity_chk CHECK (quantity > 0),
    CONSTRAINT order_items_original_chk CHECK (unit_original_price > 0),
    CONSTRAINT order_items_price_chk    CHECK (unit_price >= 0)
);
CREATE INDEX idx_order_items_offer ON order_items(offer_id);

CREATE TABLE order_status_history (
    id            BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    order_id      BIGINT       NOT NULL,
    from_status   order_status,
    to_status     order_status NOT NULL,
    actor_user_id BIGINT,
    reason        TEXT,
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT now(),
    CONSTRAINT osh_order_fkey FOREIGN KEY (order_id)      REFERENCES orders(id) ON DELETE CASCADE,
    CONSTRAINT osh_actor_fkey FOREIGN KEY (actor_user_id) REFERENCES users(id)
);
CREATE INDEX idx_osh_order ON order_status_history(order_id, created_at);

-- Order state machine guard + lifecycle timestamps.
CREATE OR REPLACE FUNCTION fn_order_status_guard() RETURNS trigger AS $$
BEGIN
    IF NEW.status IS DISTINCT FROM OLD.status THEN
        IF NOT ((OLD.status::text, NEW.status::text) IN (
              ('pending_payment', 'paid'), ('pending_payment', 'expired'), ('pending_payment', 'cancelled'),
              ('paid', 'ready'), ('paid', 'completed'), ('paid', 'cancelled'), ('paid', 'no_show'),
              ('ready', 'completed'), ('ready', 'cancelled'), ('ready', 'no_show'))) THEN
            RAISE EXCEPTION 'invalid order transition % -> %', OLD.status, NEW.status USING ERRCODE = '23514';
        END IF;
        CASE NEW.status
            WHEN 'paid'      THEN NEW.paid_at      := COALESCE(NEW.paid_at, now());
            WHEN 'ready'     THEN NEW.ready_at     := COALESCE(NEW.ready_at, now());
            WHEN 'completed' THEN NEW.completed_at := COALESCE(NEW.completed_at, now());
            WHEN 'cancelled' THEN NEW.cancelled_at := COALESCE(NEW.cancelled_at, now());
            ELSE NULL;
        END CASE;
    END IF;
    NEW.updated_at := now();
    RETURN NEW;
END $$ LANGUAGE plpgsql;

CREATE TRIGGER trg_orders_guard
    BEFORE UPDATE ON orders
    FOR EACH ROW EXECUTE FUNCTION fn_order_status_guard();

-- Delivery address: must belong to the buyer; a snapshot is copied into the order.
CREATE OR REPLACE FUNCTION fn_order_address_snapshot() RETURNS trigger AS $$
DECLARE a user_addresses%ROWTYPE;
BEGIN
    SELECT * INTO a FROM user_addresses
     WHERE id = NEW.address_id AND user_id = NEW.user_id AND deleted_at IS NULL;
    IF NOT FOUND THEN
        RAISE EXCEPTION 'delivery address does not exist or does not belong to this user' USING ERRCODE = '23503';
    END IF;
    NEW.delivery_street_name   := a.street_name;
    NEW.delivery_plaque_number := a.plaque_number;
    NEW.delivery_unit_number   := a.unit_number;
    RETURN NEW;
END $$ LANGUAGE plpgsql;

CREATE TRIGGER trg_orders_address
    BEFORE INSERT ON orders
    FOR EACH ROW EXECUTE FUNCTION fn_order_address_snapshot();

-- Status history (the app passes the actor with: SET LOCAL app.user_id = '42').
CREATE OR REPLACE FUNCTION fn_order_status_history() RETURNS trigger AS $$
BEGIN
    IF TG_OP = 'INSERT' OR NEW.status IS DISTINCT FROM OLD.status THEN
        INSERT INTO order_status_history (order_id, from_status, to_status, actor_user_id, reason)
        VALUES (NEW.id,
                CASE WHEN TG_OP = 'UPDATE' THEN OLD.status END,
                NEW.status,
                NULLIF(current_setting('app.user_id', true), '')::bigint,
                NEW.cancel_reason);
    END IF;
    RETURN NULL;
END $$ LANGUAGE plpgsql;

CREATE TRIGGER trg_orders_history
    AFTER INSERT OR UPDATE OF status ON orders
    FOR EACH ROW EXECUTE FUNCTION fn_order_status_history();

-- Atomic stock reservation (no overselling, no explicit lock needed).
-- Returns TRUE if reserved. Call inside the same tx that inserts the order.
CREATE OR REPLACE FUNCTION fn_reserve_stock(p_offer_id BIGINT, p_qty INT) RETURNS BOOLEAN AS $$
DECLARE n INT;
BEGIN
    UPDATE offers
       SET quantity_available = quantity_available - p_qty,
           status = CASE WHEN quantity_available - p_qty = 0 THEN 'sold_out'::offer_status ELSE status END
     WHERE id = p_offer_id
       AND status = 'active'
       AND quantity_available >= p_qty
       AND now() < pickup_end;
    GET DIAGNOSTICS n = ROW_COUNT;
    RETURN n = 1;
END $$ LANGUAGE plpgsql;

-- Releases the stock of unpaid orders whose hold expired (run every minute by a worker).
CREATE OR REPLACE FUNCTION fn_release_expired_orders() RETURNS INT AS $$
DECLARE n INT;
BEGIN
    WITH due AS (
        SELECT id FROM orders
         WHERE status = 'pending_payment' AND reserved_until < now()
         FOR UPDATE SKIP LOCKED
    ), expired AS (
        UPDATE orders o SET status = 'expired' FROM due WHERE o.id = due.id RETURNING o.id
    ), restore AS (
        UPDATE offers f
           SET quantity_available = f.quantity_available + s.q,
               status = CASE WHEN f.status = 'sold_out' AND f.pickup_end > now()
                             THEN 'active'::offer_status ELSE f.status END
          FROM (SELECT oi.offer_id, sum(oi.quantity)::int AS q
                  FROM order_items oi JOIN expired e ON e.id = oi.order_id
                 GROUP BY oi.offer_id) s
         WHERE f.id = s.offer_id
         RETURNING f.id
    )
    SELECT count(*) INTO n FROM expired;
    RETURN n;
END $$ LANGUAGE plpgsql;

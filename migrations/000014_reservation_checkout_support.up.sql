-- Support pickup-based customer reservations on orders (no delivery address required),
-- idempotency payload verification, and store pickup instructions.

ALTER TABLE orders
    ALTER COLUMN address_id DROP NOT NULL,
    ALTER COLUMN delivery_street_name DROP NOT NULL,
    ALTER COLUMN delivery_plaque_number DROP NOT NULL,
    ALTER COLUMN delivery_unit_number DROP NOT NULL,
    ADD COLUMN IF NOT EXISTS idempotency_payload_hash TEXT;

ALTER TABLE stores
    ADD COLUMN IF NOT EXISTS pickup_instructions TEXT NOT NULL DEFAULT '';

CREATE OR REPLACE FUNCTION fn_order_address_snapshot() RETURNS trigger AS $$
DECLARE a user_addresses%ROWTYPE;
BEGIN
    IF NEW.address_id IS NULL THEN
        RETURN NEW;
    END IF;
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

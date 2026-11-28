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

ALTER TABLE orders
    DROP COLUMN IF EXISTS idempotency_payload_hash,
    ALTER COLUMN delivery_unit_number SET NOT NULL,
    ALTER COLUMN delivery_plaque_number SET NOT NULL,
    ALTER COLUMN delivery_street_name SET NOT NULL,
    ALTER COLUMN address_id SET NOT NULL;

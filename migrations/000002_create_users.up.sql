CREATE TYPE user_status AS ENUM ('active', 'suspended', 'deleted');

CREATE OR REPLACE FUNCTION fn_set_updated_at() RETURNS trigger AS $$
BEGIN
    NEW.updated_at := now();
    RETURN NEW;
END $$ LANGUAGE plpgsql;

-- Phone is the only identifier; authentication is OTP-only (no email / password).
CREATE TABLE users (
    id            BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    public_id     UUID        NOT NULL DEFAULT gen_random_uuid(),
    phone         TEXT,
    full_name     TEXT,
    avatar_url    TEXT,
    status        user_status NOT NULL DEFAULT 'active',  -- row exists only after OTP verification
    no_show_count INT         NOT NULL DEFAULT 0,
    penalty_until TIMESTAMPTZ,
    last_login_at TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at    TIMESTAMPTZ,
    CONSTRAINT users_public_id_key    UNIQUE (public_id),
    CONSTRAINT users_phone_key        UNIQUE (phone),
    CONSTRAINT users_phone_format_chk CHECK (phone ~ '^\+?[0-9]{10,15}$'),
    CONSTRAINT users_phone_chk        CHECK (phone IS NOT NULL OR status = 'deleted'),
    CONSTRAINT users_no_show_chk      CHECK (no_show_count >= 0)
);

CREATE TRIGGER trg_users_updated
    BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION fn_set_updated_at();

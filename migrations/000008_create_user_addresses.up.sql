CREATE TABLE user_addresses (
    id            BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id       BIGINT      NOT NULL,
    label         TEXT,                                   -- "home", "work" ...
    street_name   TEXT        NOT NULL,
    plaque_number TEXT        NOT NULL,                   -- TEXT: "12/1", "12-B"
    unit_number   TEXT        NOT NULL,                   -- TEXT: "5", "ground floor"
    is_default    BOOLEAN     NOT NULL DEFAULT false,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at    TIMESTAMPTZ,                            -- soft delete: orders keep referencing it
    CONSTRAINT user_addresses_user_fkey  FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    CONSTRAINT user_addresses_street_chk CHECK (length(btrim(street_name))   > 0),
    CONSTRAINT user_addresses_plaque_chk CHECK (length(btrim(plaque_number)) > 0),
    CONSTRAINT user_addresses_unit_chk   CHECK (length(btrim(unit_number))   > 0)
);
CREATE INDEX idx_addresses_user ON user_addresses(user_id) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX uq_addresses_one_default ON user_addresses(user_id) WHERE is_default AND deleted_at IS NULL;

CREATE TRIGGER trg_addresses_updated
    BEFORE UPDATE ON user_addresses
    FOR EACH ROW EXECUTE FUNCTION fn_set_updated_at();

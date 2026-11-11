-- One flow for register + login: verify OTP -> create the user if the phone is new.
CREATE TABLE otp_codes (
    id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    phone       TEXT        NOT NULL,
    code_hash   TEXT        NOT NULL,              -- never store the raw code
    attempts    SMALLINT    NOT NULL DEFAULT 0,
    expires_at  TIMESTAMPTZ NOT NULL,
    consumed_at TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT otp_phone_format_chk CHECK (phone ~ '^\+?[0-9]{10,15}$'),
    CONSTRAINT otp_attempts_chk     CHECK (attempts >= 0),
    CONSTRAINT otp_expiry_chk       CHECK (expires_at > created_at)
);
CREATE INDEX idx_otp_phone_created ON otp_codes(phone, created_at DESC);
CREATE INDEX idx_otp_expires       ON otp_codes(expires_at) WHERE consumed_at IS NULL;  -- cleanup job

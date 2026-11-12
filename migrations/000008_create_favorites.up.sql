CREATE TABLE favorite_offers (
    user_id    BIGINT      NOT NULL REFERENCES users(id)  ON DELETE CASCADE,
    offer_id   BIGINT      NOT NULL REFERENCES offers(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, offer_id)   -- makes PUT idempotent: INSERT ... ON CONFLICT DO NOTHING
);
CREATE INDEX idx_favorite_offers_offer ON favorite_offers (offer_id);

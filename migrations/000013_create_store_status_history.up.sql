-- Every admin decision on a store (approve, reject, suspend, reactivate).
-- stores.status stays the only source of truth; this table records who
-- changed it, when and why. The latest reject/suspend row is the decision
-- shown to the vendor.
CREATE TABLE store_status_history (
    id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    store_id    BIGINT       NOT NULL,
    from_status store_status NOT NULL,
    to_status   store_status NOT NULL,
    action      TEXT         NOT NULL,
    reason_code TEXT,
    message     TEXT,
    admin_id    BIGINT,                                  -- users.id of the deciding admin
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now(),
    CONSTRAINT ssh_store_fkey   FOREIGN KEY (store_id) REFERENCES stores(id) ON DELETE CASCADE,
    CONSTRAINT ssh_admin_fkey   FOREIGN KEY (admin_id) REFERENCES users(id)  ON DELETE SET NULL,
    CONSTRAINT ssh_action_chk   CHECK (action IN ('approve', 'reject', 'suspend', 'reactivate')),
    -- reject and suspend must carry a reason; approve and reactivate never do
    CONSTRAINT ssh_reason_chk   CHECK (
           (action IN ('reject', 'suspend')      AND reason_code IS NOT NULL AND message IS NOT NULL)
        OR (action IN ('approve', 'reactivate') AND reason_code IS NULL     AND message IS NULL)),
    CONSTRAINT ssh_message_chk  CHECK (message IS NULL OR char_length(btrim(message)) BETWEEN 20 AND 500),
    CONSTRAINT ssh_changed_chk  CHECK (from_status <> to_status)
);
CREATE INDEX idx_ssh_store ON store_status_history(store_id, created_at DESC, id DESC);

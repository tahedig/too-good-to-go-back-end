CREATE TYPE store_request_status AS ENUM ('pending_review', 'approved', 'rejected');

CREATE TABLE store_requests (
    id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    public_id   UUID                 NOT NULL DEFAULT gen_random_uuid(),
    vendor_id   BIGINT               NOT NULL,
    store_name  TEXT                 NOT NULL,
    owner_name  TEXT                 NOT NULL,
    category_id SMALLINT,
    address     TEXT                 NOT NULL,
    status      store_request_status NOT NULL DEFAULT 'pending_review',
    created_at  TIMESTAMPTZ          NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ          NOT NULL DEFAULT now(),
    CONSTRAINT store_requests_public_id_key UNIQUE (public_id),
    CONSTRAINT store_requests_vendor_fkey   FOREIGN KEY (vendor_id)   REFERENCES vendors(id) ON DELETE CASCADE,
    CONSTRAINT store_requests_category_fkey FOREIGN KEY (category_id) REFERENCES categories(id) ON DELETE SET NULL,
    CONSTRAINT store_requests_name_chk      CHECK (length(btrim(store_name)) > 0),
    CONSTRAINT store_requests_owner_chk     CHECK (length(btrim(owner_name)) > 0),
    CONSTRAINT store_requests_address_chk   CHECK (length(btrim(address)) > 0)
);
CREATE INDEX idx_store_requests_vendor ON store_requests(vendor_id);
CREATE INDEX idx_store_requests_status ON store_requests(status, created_at DESC);

CREATE TRIGGER trg_store_requests_updated
    BEFORE UPDATE ON store_requests
    FOR EACH ROW EXECUTE FUNCTION fn_set_updated_at();

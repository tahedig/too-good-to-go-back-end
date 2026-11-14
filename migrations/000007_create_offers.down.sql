DROP TABLE IF EXISTS offer_allergens;
DROP TRIGGER IF EXISTS trg_offers_updated ON offers;
DROP TABLE IF EXISTS offers;
DROP INDEX IF EXISTS idx_stores_name_trgm;   -- created by the up migration on stores
DROP EXTENSION IF EXISTS pg_trgm;

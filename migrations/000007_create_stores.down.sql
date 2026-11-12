DROP TRIGGER   IF EXISTS trg_stores_updated  ON stores;
DROP TABLE     IF EXISTS stores;
DROP TRIGGER   IF EXISTS trg_vendors_updated ON vendors;
DROP TABLE     IF EXISTS vendors;
DROP TABLE     IF EXISTS categories;
DROP TYPE      IF EXISTS store_status;
DROP EXTENSION IF EXISTS pg_trgm;
DROP EXTENSION IF EXISTS postgis;

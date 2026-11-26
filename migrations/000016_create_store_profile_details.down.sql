ALTER TABLE offers
    DROP COLUMN IF EXISTS currency;

DROP TRIGGER IF EXISTS trg_store_images_updated ON store_images;
DROP TABLE IF EXISTS store_images;

DROP TRIGGER IF EXISTS trg_store_working_hours_updated ON store_working_hours;
DROP TABLE IF EXISTS store_working_hours;

ALTER TABLE stores
    DROP COLUMN IF EXISTS pickup_instructions;

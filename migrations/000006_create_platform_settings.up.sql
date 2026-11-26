-- Platform-wide business settings (commission, minimum discount, reservation TTL ...).
CREATE TABLE platform_settings (
    key         TEXT        NOT NULL,
    value       JSONB       NOT NULL,
    description TEXT,
    updated_by  BIGINT,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT platform_settings_pkey         PRIMARY KEY (key),
    CONSTRAINT platform_settings_updated_fkey FOREIGN KEY (updated_by) REFERENCES users(id) ON DELETE SET NULL
);

-- Reference data: triggers and services read these keys.
INSERT INTO platform_settings (key, value, description) VALUES
    ('commission_default_percent', '15', 'Default commission (%)'),
    ('min_discount_percent',       '30', 'Minimum allowed discount (%)'),
    ('reservation_ttl_minutes',    '10', 'Payment hold before stock is released'),
    ('customer_service_fee',       '0',  'Flat service fee per order (Toman)'),
    ('no_show_ban_threshold',      '3',  'No-shows before temporary booking ban');

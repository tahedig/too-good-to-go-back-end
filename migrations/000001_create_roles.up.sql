CREATE TABLE roles (
    id      SMALLINT PRIMARY KEY,
    code    TEXT NOT NULL,
    name_fa TEXT NOT NULL,
    CONSTRAINT roles_code_key UNIQUE (code)
);

-- Reference data is part of the structure: the app relies on these ids/codes.
INSERT INTO roles (id, code, name_fa) VALUES
    (1, 'customer',     'مشتری'),
    (2, 'vendor_owner', 'صاحب کسب‌وکار'),
    (3, 'vendor_staff', 'کارمند فروشگاه'),
    (4, 'support',      'پشتیبان'),
    (5, 'admin',        'ادمین');

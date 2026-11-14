-- kind maps each category to one of the three top tabs on the home page.
CREATE TYPE category_kind AS ENUM ('bakery', 'ready_meal', 'grocery');

CREATE TABLE categories (
    id         SMALLINT PRIMARY KEY,
    slug       TEXT          NOT NULL,
    name_fa    TEXT          NOT NULL,
    kind       category_kind NOT NULL,
    sort_order SMALLINT      NOT NULL DEFAULT 0,
    is_active  BOOLEAN       NOT NULL DEFAULT TRUE,
    CONSTRAINT categories_slug_key UNIQUE (slug)
);

INSERT INTO categories (id, slug, name_fa, kind, sort_order) VALUES
    (1,  'bakery',          'نانوایی',               'bakery',     1),
    (2,  'sweets-desserts', 'شیرینی و دسر',          'bakery',     2),
    (3,  'cafe',            'کافه',                  'ready_meal', 3),
    (4,  'restaurant',      'رستوران',               'ready_meal', 4),
    (5,  'iranian',         'غذای ایرانی',           'ready_meal', 5),
    (6,  'fastfood',        'فست‌فود',               'ready_meal', 6),
    (7,  'pizza',           'پیتزا',                 'ready_meal', 7),
    (8,  'sandwich',        'ساندویچ',               'ready_meal', 8),
    (9,  'homemade',        'غذای خانگی',            'ready_meal', 9),
    (10, 'vegetarian',      'غذاهای گیاهی',          'ready_meal', 10),
    (11, 'burger',          'برگر',                  'ready_meal', 11),
    (12, 'salad-healthy',   'سالاد و غذای سالم',     'ready_meal', 12),
    (13, 'breakfast',       'صبحانه',                'ready_meal', 13),
    (14, 'drinks',          'نوشیدنی',               'ready_meal', 14),
    (15, 'juice-smoothie',  'آبمیوه و اسموتی',       'ready_meal', 15),
    (16, 'supermarket',     'سوپرمارکت',             'grocery',    16),
    (17, 'fruit-veg',       'میوه و سبزیجات',        'grocery',    17),
    (18, 'dairy',           'لبنیات',                'grocery',    18),
    (19, 'protein',         'پروتئینی',              'grocery',    19),
    (20, 'nuts-snacks',     'خشکبار و تنقلات',       'grocery',    20),
    (21, 'international',   'غذاهای بین‌المللی',     'ready_meal', 21);

-- Fixed, limited allergen list (product decision: no free-form diet field).
CREATE TABLE allergens (
    code    TEXT PRIMARY KEY,
    name_fa TEXT NOT NULL
);
INSERT INTO allergens (code, name_fa) VALUES
    ('gluten',    'گلوتن'),
    ('dairy',     'لبنیات'),
    ('egg',       'تخم‌مرغ'),
    ('peanut',    'بادام زمینی'),
    ('nuts',      'مغزها'),
    ('soy',       'سویا'),
    ('fish',      'ماهی'),
    ('shellfish', 'صدف دریایی'),
    ('sesame',    'کنجد');

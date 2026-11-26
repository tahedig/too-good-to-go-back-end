package seed

import (
	"time"

	"github.com/tahedig/too-good-to-go/backend/internal/domain/enum"
)

// Config controls demo account phone numbers and environment safety checks.
// Phone numbers are read from environment variables so environment-specific
// accounts stay out of source control.
type Config struct {
	AppEnv         string
	AllowNonDev    bool
	AdminPhone     string
	VendorPhones   []string
	CustomerPhones []string
}

type seedStore struct {
	PublicID           string
	VendorIndex        int
	CategoryID         int16
	OwnerName          string
	Name               string
	Slug               string
	Biography          string
	Phone              string
	City               string
	District           string
	AddressLine        string
	PickupInstructions string
	Lat                float64
	Lng                float64
	Status             enum.StoreStatus
	AcceptsOrders      bool
	RatingAvg          float64
	RatingCount        int
	MealsSaved         int
	ImageURL           string
}

type seedOffer struct {
	PublicID          string
	StoreSlug         string
	OfferType         string
	Title             string
	Description       string
	CategoryID        int16
	OriginalPrice     int64
	DiscountPrice     int64
	QuantityTotal     int
	QuantityAvailable int
	MaxPerUser        int16
	PickupStartOffset time.Duration
	PickupEndOffset   time.Duration
	Allergens         []string
	AllergensDeclared bool
	Status            string
	ImageURL          string
}

type seedOrder struct {
	PublicID       string
	IdempotencyKey string
	QRToken        string
	CustomerIndex  int
	StoreSlug      string
	OfferPublicID  string
	Quantity       int
	Status         enum.OrderStatus
	PickupCode     string
	ReservedOffset time.Duration
	CreatedOffset  time.Duration
	CancelledBy    string
	CancelReason   string
	WithPayment    bool
}

var defaultStores = []seedStore{
	{
		PublicID:           "11111111-1111-4111-8111-111111111001",
		VendorIndex:        0,
		CategoryID:         1, // bakery
		OwnerName:          "علی رضایی",
		Name:               "نانوایی و شیرینی‌پزی ولیعصر",
		Slug:               "seed-bakery-valiasr",
		Biography:          "نان تازه تنوری، کروسان و شیرینی‌های روزانه با بسته‌بندی بهداشتی.",
		Phone:              "+982188001001",
		City:               "تهران",
		District:           "ولیعصر",
		AddressLine:        "خیابان ولیعصر، بالاتر از میدان ونک، پلاک ۱۲۴",
		PickupInstructions: "از ورودی سمت راست مغازه به صندوق تحویل بسته‌های سورپرایز مراجعه کنید.",
		Lat:                35.7219,
		Lng:                51.4090,
		Status:             enum.StoreStatusApproved,
		AcceptsOrders:      true,
		RatingAvg:          4.70,
		RatingCount:        48,
		MealsSaved:         135,
		ImageURL:           "https://cdn.example.local/stores/bakery-valiasr.jpg",
	},
	{
		PublicID:           "11111111-1111-4111-8111-111111111002",
		VendorIndex:        0,
		CategoryID:         7, // pizza
		OwnerName:          "علی رضایی",
		Name:               "پیتزا آراز",
		Slug:               "seed-pizza-araz",
		Biography:          "پیتزاهای تنوری ایتالیایی و ساندویچ‌های گرم آماده تحویل در پایان روز.",
		Phone:              "+982177002002",
		City:               "تهران",
		District:           "تهرانپارس",
		AddressLine:        "فلکه اول تهرانپارس، خیابان گلبرگ، پلاک ۴۲",
		PickupInstructions: "کد تحویل چهار رقمی را به همکار صندوق اعلام کنید.",
		Lat:                35.7350,
		Lng:                51.5150,
		Status:             enum.StoreStatusApproved,
		AcceptsOrders:      true,
		RatingAvg:          4.50,
		RatingCount:        32,
		MealsSaved:         89,
		ImageURL:           "https://cdn.example.local/stores/pizza-araz.jpg",
	},
	{
		PublicID:           "11111111-1111-4111-8111-111111111003",
		VendorIndex:        1,
		CategoryID:         3, // cafe
		OwnerName:          "مریم کریمی",
		Name:               "کافه رشد",
		Slug:               "seed-cafe-roshd",
		Biography:          "کیک‌های خانگی، مافین، ساندویچ‌های سرد و نوشیدنی‌های تخصصی کافه.",
		Phone:              "+982188003003",
		City:               "تهران",
		District:           "ونک",
		AddressLine:        "میدان ونک، خیابان ملاصدرا، پلاک ۱۸",
		PickupInstructions: "تحویل سفارش از پیشخوان بار سرد انجام می‌شود.",
		Lat:                35.7575,
		Lng:                51.4100,
		Status:             enum.StoreStatusApproved,
		AcceptsOrders:      true,
		RatingAvg:          4.85,
		RatingCount:        64,
		MealsSaved:         210,
		ImageURL:           "https://cdn.example.local/stores/cafe-roshd.jpg",
	},
	{
		PublicID:           "11111111-1111-4111-8111-111111111004",
		VendorIndex:        1,
		CategoryID:         16, // supermarket
		OwnerName:          "مریم کریمی",
		Name:               "سوپرمارکت سبز",
		Slug:               "seed-supermarket-sabz",
		Biography:          "بسته‌های میوه، سبزیجات تازه، لبنیات و اقلام روزمره با تخفیف ویژه.",
		Phone:              "+982122004004",
		City:               "تهران",
		District:           "جردن",
		AddressLine:        "بلوار ناهید شرقی، پلاک ۷۶",
		PickupInstructions: "برای دریافت بسته به بخش بسته‌بندی میوه و تره‌بار مراجعه کنید.",
		Lat:                35.7700,
		Lng:                51.4200,
		Status:             enum.StoreStatusApproved,
		AcceptsOrders:      true,
		RatingAvg:          4.35,
		RatingCount:        21,
		MealsSaved:         54,
		ImageURL:           "https://cdn.example.local/stores/supermarket-sabz.jpg",
	},
	{
		PublicID:           "11111111-1111-4111-8111-111111111005",
		VendorIndex:        0,
		CategoryID:         5, // iranian
		OwnerName:          "علی رضایی",
		Name:               "رستوران گلستان",
		Slug:               "seed-iranian-golestan",
		Biography:          "غذاهای اصیل ایرانی، خورشت‌های روز و برنج ایرانی با کیفیت رستورانی.",
		Phone:              "+982166005005",
		City:               "تهران",
		District:           "انقلاب",
		AddressLine:        "خیابان انقلاب، نزدیک چهارراه ولیعصر، پلاک ۳۱۰",
		PickupInstructions: "لطفاً در بازه زمانی اعلام‌شده به قسمت بیرون‌بر مراجعه نمایید.",
		Lat:                35.7010,
		Lng:                51.3910,
		Status:             enum.StoreStatusApproved,
		AcceptsOrders:      true,
		RatingAvg:          4.60,
		RatingCount:        39,
		MealsSaved:         112,
		ImageURL:           "https://cdn.example.local/stores/iranian-golestan.jpg",
	},
	{
		PublicID:           "11111111-1111-4111-8111-111111111006",
		VendorIndex:        1,
		CategoryID:         12, // salad-healthy
		OwnerName:          "مریم کریمی",
		Name:               "سالاد و غذای سالم بهار",
		Slug:               "seed-salad-bahar",
		Biography:          "انواع سالاد بار، اسموتی و غذاهای رژیمی تازه آماده در انتظار تایید.",
		Phone:              "+982122006006",
		City:               "تهران",
		District:           "سعادت‌آباد",
		AddressLine:        "بلوار دریا، پلاک ۹۵",
		PickupInstructions: "تحویل از پیشخوان اصلی فروشگاه.",
		Lat:                35.7820,
		Lng:                51.3750,
		Status:             enum.StoreStatusPending,
		AcceptsOrders:      true,
		RatingAvg:          0,
		RatingCount:        0,
		MealsSaved:         0,
		ImageURL:           "https://cdn.example.local/stores/salad-bahar.jpg",
	},
}

var defaultOffers = []seedOffer{
	// Store 1: Bakery Valiasr (5 offers)
	{
		PublicID:          "22222222-2222-4222-8222-222222222001",
		StoreSlug:         "seed-bakery-valiasr",
		OfferType:         "surprise_bag",
		Title:             "بسته سورپرایز نان و شیرینی صبحگاهی",
		Description:       "شامل ترکیبی از نان سنگک، باگت و شیرینی‌های تازه پخت همان روز.",
		CategoryID:        1,
		OriginalPrice:     180000,
		DiscountPrice:     85000,
		QuantityTotal:     10,
		QuantityAvailable: 6,
		MaxPerUser:        2,
		PickupStartOffset: -30 * time.Minute,
		PickupEndOffset:   4 * time.Hour,
		Allergens:         []string{"gluten", "sesame"},
		AllergensDeclared: true,
		Status:            "active",
	},
	{
		PublicID:          "22222222-2222-4222-8222-222222222002",
		StoreSlug:         "seed-bakery-valiasr",
		OfferType:         "specific_item",
		Title:             "پک ۴ عددی کروسان کره‌ای",
		Description:       "چهار عدد کروسان کره‌ای تازه پخته‌شده مناسب عصرانه و صبحانه.",
		CategoryID:        2,
		OriginalPrice:     220000,
		DiscountPrice:     110000,
		QuantityTotal:     8,
		QuantityAvailable: 4,
		MaxPerUser:        2,
		PickupStartOffset: 1 * time.Hour,
		PickupEndOffset:   6 * time.Hour,
		Allergens:         []string{"gluten", "dairy", "egg"},
		AllergensDeclared: true,
		Status:            "active",
	},
	{
		PublicID:          "22222222-2222-4222-8222-222222222003",
		StoreSlug:         "seed-bakery-valiasr",
		OfferType:         "surprise_bag",
		Title:             "بسته شیرینی تر و خشک مجلسی",
		Description:       "بسته اقتصادی شامل انواع شیرینی خشک و تارت میوه‌ای تازه.",
		CategoryID:        2,
		OriginalPrice:     320000,
		DiscountPrice:     150000,
		QuantityTotal:     6,
		QuantityAvailable: 3,
		MaxPerUser:        1,
		PickupStartOffset: 2 * time.Hour,
		PickupEndOffset:   8 * time.Hour,
		Allergens:         []string{"gluten", "dairy", "nuts"},
		AllergensDeclared: true,
		Status:            "active",
	},
	{
		PublicID:          "22222222-2222-4222-8222-222222222004",
		StoreSlug:         "seed-bakery-valiasr",
		OfferType:         "specific_item",
		Title:             "نان بربری و تست سبوس‌دار",
		Description:       "دو عدد نان بربری کنجدی و یک بسته نان تست سبوس‌دار تازه.",
		CategoryID:        1,
		OriginalPrice:     120000,
		DiscountPrice:     60000,
		QuantityTotal:     5,
		QuantityAvailable: 0,
		MaxPerUser:        2,
		PickupStartOffset: -1 * time.Hour,
		PickupEndOffset:   3 * time.Hour,
		Allergens:         []string{"gluten", "sesame"},
		AllergensDeclared: true,
		Status:            "sold_out",
	},
	{
		PublicID:          "22222222-2222-4222-8222-222222222005",
		StoreSlug:         "seed-bakery-valiasr",
		OfferType:         "surprise_bag",
		Title:             "بسته عصرانه نانوایی دیروز",
		Description:       "بسته منقضی‌شده جهت نمایش تاریخچه پیشنهادهای پایان‌یافته فروشگاه.",
		CategoryID:        1,
		OriginalPrice:     150000,
		DiscountPrice:     70000,
		QuantityTotal:     4,
		QuantityAvailable: 2,
		MaxPerUser:        2,
		PickupStartOffset: -10 * time.Hour,
		PickupEndOffset:   -4 * time.Hour,
		Allergens:         []string{"gluten"},
		AllergensDeclared: true,
		Status:            "expired",
	},

	// Store 2: Pizza Araz (4 offers)
	{
		PublicID:          "22222222-2222-4222-8222-222222222006",
		StoreSlug:         "seed-pizza-araz",
		OfferType:         "surprise_bag",
		Title:             "بسته سورپرایز پیتزا و پیش‌غذا",
		Description:       "شامل برش‌های پیتزا مارگاریتا یا پپرونی به همراه نان سیر تازه.",
		CategoryID:        7,
		OriginalPrice:     420000,
		DiscountPrice:     195000,
		QuantityTotal:     12,
		QuantityAvailable: 7,
		MaxPerUser:        2,
		PickupStartOffset: 30 * time.Minute,
		PickupEndOffset:   5 * time.Hour,
		Allergens:         []string{"gluten", "dairy"},
		AllergensDeclared: true,
		Status:            "active",
	},
	{
		PublicID:          "22222222-2222-4222-8222-222222222007",
		StoreSlug:         "seed-pizza-araz",
		OfferType:         "specific_item",
		Title:             "پیتزا مخصوص ایتالیایی متوسط",
		Description:       "پیتزا تنوری با خمیر دست‌ساز، پنیر موزارلا و قارچ تازه.",
		CategoryID:        7,
		OriginalPrice:     380000,
		DiscountPrice:     210000,
		QuantityTotal:     6,
		QuantityAvailable: 4,
		MaxPerUser:        2,
		PickupStartOffset: 1 * time.Hour,
		PickupEndOffset:   7 * time.Hour,
		Allergens:         []string{"gluten", "dairy"},
		AllergensDeclared: true,
		Status:            "active",
	},
	{
		PublicID:          "22222222-2222-4222-8222-222222222008",
		StoreSlug:         "seed-pizza-araz",
		OfferType:         "specific_item",
		Title:             "ساندویچ چیکن پستو ویژه",
		Description:       "ساندویچ گرم فیله مرغ گریل با سس پستو و نان چاپاتا.",
		CategoryID:        8,
		OriginalPrice:     290000,
		DiscountPrice:     145000,
		QuantityTotal:     8,
		QuantityAvailable: 5,
		MaxPerUser:        2,
		PickupStartOffset: 2 * time.Hour,
		PickupEndOffset:   9 * time.Hour,
		Allergens:         []string{"gluten", "dairy", "nuts"},
		AllergensDeclared: true,
		Status:            "active",
	},
	{
		PublicID:          "22222222-2222-4222-8222-222222222009",
		StoreSlug:         "seed-pizza-araz",
		OfferType:         "surprise_bag",
		Title:             "بسته فست‌فود اقتصادی شبانه",
		Description:       "ترکیبی از برگر یا ساندویچ و سیب‌زمینی تنوری پایان شیفت.",
		CategoryID:        6,
		OriginalPrice:     350000,
		DiscountPrice:     160000,
		QuantityTotal:     5,
		QuantityAvailable: 3,
		MaxPerUser:        1,
		PickupStartOffset: 3 * time.Hour,
		PickupEndOffset:   10 * time.Hour,
		Allergens:         []string{},
		AllergensDeclared: false,
		Status:            "active",
	},

	// Store 3: Cafe Roshd (4 offers)
	{
		PublicID:          "22222222-2222-4222-8222-222222222010",
		StoreSlug:         "seed-cafe-roshd",
		OfferType:         "surprise_bag",
		Title:             "بسته سورپرایز کیک و پیستری کافه",
		Description:       "دو اسلایس کیک شکلاتی یا چیزکیک به همراه کوکی تازه کافه.",
		CategoryID:        3,
		OriginalPrice:     280000,
		DiscountPrice:     130000,
		QuantityTotal:     10,
		QuantityAvailable: 8,
		MaxPerUser:        2,
		PickupStartOffset: -15 * time.Minute,
		PickupEndOffset:   4 * time.Hour,
		Allergens:         []string{"gluten", "dairy", "egg"},
		AllergensDeclared: true,
		Status:            "active",
	},
	{
		PublicID:          "22222222-2222-4222-8222-222222222011",
		StoreSlug:         "seed-cafe-roshd",
		OfferType:         "specific_item",
		Title:             "چیزکیک سن‌سباستین دو نفره",
		Description:       "دو برش چیزکیک سن‌سباستین خامه‌ای با سس شکلات تلخ.",
		CategoryID:        2,
		OriginalPrice:     300000,
		DiscountPrice:     165000,
		QuantityTotal:     6,
		QuantityAvailable: 3,
		MaxPerUser:        2,
		PickupStartOffset: 1 * time.Hour,
		PickupEndOffset:   6 * time.Hour,
		Allergens:         []string{"dairy", "egg"},
		AllergensDeclared: true,
		Status:            "active",
	},
	{
		PublicID:          "22222222-2222-4222-8222-222222222012",
		StoreSlug:         "seed-cafe-roshd",
		OfferType:         "specific_item",
		Title:             "پک صبحانه کامل کافه رشد",
		Description:       "ساندویچ پنیر و گردو، املت سبزیجات و آب‌پرتقال طبیعی.",
		CategoryID:        13,
		OriginalPrice:     260000,
		DiscountPrice:     140000,
		QuantityTotal:     7,
		QuantityAvailable: 5,
		MaxPerUser:        2,
		PickupStartOffset: 2 * time.Hour,
		PickupEndOffset:   8 * time.Hour,
		Allergens:         []string{"gluten", "dairy", "nuts", "egg"},
		AllergensDeclared: true,
		Status:            "active",
	},
	{
		PublicID:          "22222222-2222-4222-8222-222222222013",
		StoreSlug:         "seed-cafe-roshd",
		OfferType:         "surprise_bag",
		Title:             "بسته کلاب و اسموتی عصرانه",
		Description:       "یک عدد کلاب ساندویچ مرغ و یک اسموتی میوه‌ای خنک.",
		CategoryID:        15,
		OriginalPrice:     240000,
		DiscountPrice:     115000,
		QuantityTotal:     5,
		QuantityAvailable: 0,
		MaxPerUser:        2,
		PickupStartOffset: 30 * time.Minute,
		PickupEndOffset:   5 * time.Hour,
		Allergens:         []string{"gluten", "egg"},
		AllergensDeclared: true,
		Status:            "sold_out",
	},

	// Store 4: Supermarket Sabz (4 offers)
	{
		PublicID:          "22222222-2222-4222-8222-222222222014",
		StoreSlug:         "seed-supermarket-sabz",
		OfferType:         "surprise_bag",
		Title:             "سبد میوه و سبزیجات تازه روز",
		Description:       "سه کیلوگرم میوه و سبزیجات دست‌چین‌شده مناسب مصرف روزانه خانواده.",
		CategoryID:        17,
		OriginalPrice:     250000,
		DiscountPrice:     115000,
		QuantityTotal:     15,
		QuantityAvailable: 10,
		MaxPerUser:        3,
		PickupStartOffset: 0,
		PickupEndOffset:   6 * time.Hour,
		Allergens:         []string{},
		AllergensDeclared: true,
		Status:            "active",
	},
	{
		PublicID:          "22222222-2222-4222-8222-222222222015",
		StoreSlug:         "seed-supermarket-sabz",
		OfferType:         "surprise_bag",
		Title:             "بسته لبنیات محلی و تازه",
		Description:       "شیر، ماست چکیده و پنیر سفید تازه با تاریخ مصرف نزدیک.",
		CategoryID:        18,
		OriginalPrice:     210000,
		DiscountPrice:     105000,
		QuantityTotal:     10,
		QuantityAvailable: 6,
		MaxPerUser:        2,
		PickupStartOffset: 1 * time.Hour,
		PickupEndOffset:   7 * time.Hour,
		Allergens:         []string{"dairy"},
		AllergensDeclared: true,
		Status:            "active",
	},
	{
		PublicID:          "22222222-2222-4222-8222-222222222016",
		StoreSlug:         "seed-supermarket-sabz",
		OfferType:         "specific_item",
		Title:             "پک خشکبار و آجیل مخلوط",
		Description:       "بسته ۴۰۰ گرمی بادام، پسته و کشمش درجه‌یک بسته‌بندی‌شده.",
		CategoryID:        20,
		OriginalPrice:     360000,
		DiscountPrice:     195000,
		QuantityTotal:     8,
		QuantityAvailable: 5,
		MaxPerUser:        2,
		PickupStartOffset: 2 * time.Hour,
		PickupEndOffset:   12 * time.Hour,
		Allergens:         []string{"nuts", "peanut"},
		AllergensDeclared: true,
		Status:            "active",
	},
	{
		PublicID:          "22222222-2222-4222-8222-222222222017",
		StoreSlug:         "seed-supermarket-sabz",
		OfferType:         "surprise_bag",
		Title:             "بسته اقتصادی سوپرمارکت سبز",
		Description:       "مجموعه‌ای از اقلام خواربار، بیسکویت و نوشیدنی با تخفیف ویژه.",
		CategoryID:        16,
		OriginalPrice:     300000,
		DiscountPrice:     150000,
		QuantityTotal:     10,
		QuantityAvailable: 7,
		MaxPerUser:        2,
		PickupStartOffset: 1 * time.Hour,
		PickupEndOffset:   9 * time.Hour,
		Allergens:         []string{"gluten", "soy"},
		AllergensDeclared: true,
		Status:            "active",
	},

	// Store 5: Iranian Restaurant Golestan (5 offers)
	{
		PublicID:          "22222222-2222-4222-8222-222222222018",
		StoreSlug:         "seed-iranian-golestan",
		OfferType:         "surprise_bag",
		Title:             "بسته سورپرایز غذای ایرانی روز",
		Description:       "یک پرس چلوخورشت روز یا زرشک‌پلو با مرغ با بسته‌بندی گرم.",
		CategoryID:        5,
		OriginalPrice:     450000,
		DiscountPrice:     215000,
		QuantityTotal:     12,
		QuantityAvailable: 8,
		MaxPerUser:        2,
		PickupStartOffset: 30 * time.Minute,
		PickupEndOffset:   5 * time.Hour,
		Allergens:         []string{"dairy"},
		AllergensDeclared: true,
		Status:            "active",
	},
	{
		PublicID:          "22222222-2222-4222-8222-222222222019",
		StoreSlug:         "seed-iranian-golestan",
		OfferType:         "specific_item",
		Title:             "چلو کباب کوبیده دو سیخ",
		Description:       "دو سیخ کباب کوبیده گوسفندی با برنج ایرانی و گوجه کبابی.",
		CategoryID:        5,
		OriginalPrice:     520000,
		DiscountPrice:     280000,
		QuantityTotal:     8,
		QuantityAvailable: 4,
		MaxPerUser:        2,
		PickupStartOffset: 1 * time.Hour,
		PickupEndOffset:   6 * time.Hour,
		Allergens:         []string{"dairy"},
		AllergensDeclared: true,
		Status:            "active",
	},
	{
		PublicID:          "22222222-2222-4222-8222-222222222020",
		StoreSlug:         "seed-iranian-golestan",
		OfferType:         "specific_item",
		Title:             "خوراک قورمه‌سبزی خانگی",
		Description:       "خورشت قورمه‌سبزی جاافتاده با سبزی معطر و برنج دم‌کشیده.",
		CategoryID:        9,
		OriginalPrice:     390000,
		DiscountPrice:     190000,
		QuantityTotal:     6,
		QuantityAvailable: 5,
		MaxPerUser:        2,
		PickupStartOffset: 2 * time.Hour,
		PickupEndOffset:   8 * time.Hour,
		Allergens:         []string{},
		AllergensDeclared: true,
		Status:            "active",
	},
	{
		PublicID:          "22222222-2222-4222-8222-222222222021",
		StoreSlug:         "seed-iranian-golestan",
		OfferType:         "specific_item",
		Title:             "خوراک ماهی قزل‌آلا سرخ‌شده",
		Description:       "ماهی قزل‌آلا تازه با سبزی‌پلو و دورچین لیمو و زیتون.",
		CategoryID:        4,
		OriginalPrice:     560000,
		DiscountPrice:     290000,
		QuantityTotal:     5,
		QuantityAvailable: 3,
		MaxPerUser:        1,
		PickupStartOffset: 3 * time.Hour,
		PickupEndOffset:   9 * time.Hour,
		Allergens:         []string{"fish"},
		AllergensDeclared: true,
		Status:            "active",
	},
	{
		PublicID:          "22222222-2222-4222-8222-222222222022",
		StoreSlug:         "seed-iranian-golestan",
		OfferType:         "surprise_bag",
		Title:             "بسته غیرفعال پیشنهادی رستوران",
		Description:       "پیشنهاد موقتاً غیرفعال برای بررسی وضعیت‌های مختلف پنل فروشنده.",
		CategoryID:        5,
		OriginalPrice:     300000,
		DiscountPrice:     150000,
		QuantityTotal:     4,
		QuantityAvailable: 4,
		MaxPerUser:        2,
		PickupStartOffset: 4 * time.Hour,
		PickupEndOffset:   10 * time.Hour,
		Allergens:         []string{},
		AllergensDeclared: false,
		Status:            "inactive",
	},
}

var defaultOrders = []seedOrder{
	// Pending (pending_payment): 4
	{PublicID: "33333333-3333-4333-8333-333333333001", IdempotencyKey: "seed-ord-001", QRToken: "seed-qr-001", CustomerIndex: 0, StoreSlug: "seed-bakery-valiasr", OfferPublicID: "22222222-2222-4222-8222-222222222001", Quantity: 1, Status: enum.OrderStatusPendingPayment, PickupCode: "1001", ReservedOffset: 10 * time.Minute, CreatedOffset: -2 * time.Minute},
	{PublicID: "33333333-3333-4333-8333-333333333002", IdempotencyKey: "seed-ord-002", QRToken: "seed-qr-002", CustomerIndex: 1, StoreSlug: "seed-pizza-araz", OfferPublicID: "22222222-2222-4222-8222-222222222006", Quantity: 1, Status: enum.OrderStatusPendingPayment, PickupCode: "1002", ReservedOffset: 9 * time.Minute, CreatedOffset: -3 * time.Minute},
	{PublicID: "33333333-3333-4333-8333-333333333003", IdempotencyKey: "seed-ord-003", QRToken: "seed-qr-003", CustomerIndex: 0, StoreSlug: "seed-cafe-roshd", OfferPublicID: "22222222-2222-4222-8222-222222222010", Quantity: 2, Status: enum.OrderStatusPendingPayment, PickupCode: "1003", ReservedOffset: 8 * time.Minute, CreatedOffset: -4 * time.Minute},
	{PublicID: "33333333-3333-4333-8333-333333333004", IdempotencyKey: "seed-ord-004", QRToken: "seed-qr-004", CustomerIndex: 1, StoreSlug: "seed-supermarket-sabz", OfferPublicID: "22222222-2222-4222-8222-222222222014", Quantity: 1, Status: enum.OrderStatusPendingPayment, PickupCode: "1004", ReservedOffset: 7 * time.Minute, CreatedOffset: -5 * time.Minute},

	// Confirmed - Paid (paid): 4
	{PublicID: "33333333-3333-4333-8333-333333333005", IdempotencyKey: "seed-ord-005", QRToken: "seed-qr-005", CustomerIndex: 0, StoreSlug: "seed-bakery-valiasr", OfferPublicID: "22222222-2222-4222-8222-222222222002", Quantity: 1, Status: enum.OrderStatusPaid, PickupCode: "2001", ReservedOffset: -5 * time.Minute, CreatedOffset: -20 * time.Minute, WithPayment: true},
	{PublicID: "33333333-3333-4333-8333-333333333006", IdempotencyKey: "seed-ord-006", QRToken: "seed-qr-006", CustomerIndex: 1, StoreSlug: "seed-pizza-araz", OfferPublicID: "22222222-2222-4222-8222-222222222007", Quantity: 1, Status: enum.OrderStatusPaid, PickupCode: "2002", ReservedOffset: -10 * time.Minute, CreatedOffset: -25 * time.Minute, WithPayment: true},
	{PublicID: "33333333-3333-4333-8333-333333333007", IdempotencyKey: "seed-ord-007", QRToken: "seed-qr-007", CustomerIndex: 0, StoreSlug: "seed-cafe-roshd", OfferPublicID: "22222222-2222-4222-8222-222222222011", Quantity: 1, Status: enum.OrderStatusPaid, PickupCode: "2003", ReservedOffset: -15 * time.Minute, CreatedOffset: -30 * time.Minute, WithPayment: true},
	{PublicID: "33333333-3333-4333-8333-333333333008", IdempotencyKey: "seed-ord-008", QRToken: "seed-qr-008", CustomerIndex: 1, StoreSlug: "seed-iranian-golestan", OfferPublicID: "22222222-2222-4222-8222-222222222018", Quantity: 2, Status: enum.OrderStatusPaid, PickupCode: "2004", ReservedOffset: -20 * time.Minute, CreatedOffset: -35 * time.Minute, WithPayment: true},

	// Confirmed - Ready for pickup (ready): 4
	{PublicID: "33333333-3333-4333-8333-333333333009", IdempotencyKey: "seed-ord-009", QRToken: "seed-qr-009", CustomerIndex: 0, StoreSlug: "seed-bakery-valiasr", OfferPublicID: "22222222-2222-4222-8222-222222222003", Quantity: 1, Status: enum.OrderStatusReady, PickupCode: "3001", ReservedOffset: -30 * time.Minute, CreatedOffset: -45 * time.Minute, WithPayment: true},
	{PublicID: "33333333-3333-4333-8333-333333333010", IdempotencyKey: "seed-ord-010", QRToken: "seed-qr-010", CustomerIndex: 1, StoreSlug: "seed-pizza-araz", OfferPublicID: "22222222-2222-4222-8222-222222222008", Quantity: 1, Status: enum.OrderStatusReady, PickupCode: "3002", ReservedOffset: -35 * time.Minute, CreatedOffset: -50 * time.Minute, WithPayment: true},
	{PublicID: "33333333-3333-4333-8333-333333333011", IdempotencyKey: "seed-ord-011", QRToken: "seed-qr-011", CustomerIndex: 0, StoreSlug: "seed-supermarket-sabz", OfferPublicID: "22222222-2222-4222-8222-222222222015", Quantity: 1, Status: enum.OrderStatusReady, PickupCode: "3003", ReservedOffset: -40 * time.Minute, CreatedOffset: -55 * time.Minute, WithPayment: true},
	{PublicID: "33333333-3333-4333-8333-333333333012", IdempotencyKey: "seed-ord-012", QRToken: "seed-qr-012", CustomerIndex: 1, StoreSlug: "seed-iranian-golestan", OfferPublicID: "22222222-2222-4222-8222-222222222019", Quantity: 1, Status: enum.OrderStatusReady, PickupCode: "3004", ReservedOffset: -45 * time.Minute, CreatedOffset: -60 * time.Minute, WithPayment: true},

	// Completed (completed): 4
	{PublicID: "33333333-3333-4333-8333-333333333013", IdempotencyKey: "seed-ord-013", QRToken: "seed-qr-013", CustomerIndex: 0, StoreSlug: "seed-bakery-valiasr", OfferPublicID: "22222222-2222-4222-8222-222222222001", Quantity: 2, Status: enum.OrderStatusCompleted, PickupCode: "4001", ReservedOffset: -24 * time.Hour, CreatedOffset: -25 * time.Hour, WithPayment: true},
	{PublicID: "33333333-3333-4333-8333-333333333014", IdempotencyKey: "seed-ord-014", QRToken: "seed-qr-014", CustomerIndex: 1, StoreSlug: "seed-cafe-roshd", OfferPublicID: "22222222-2222-4222-8222-222222222012", Quantity: 1, Status: enum.OrderStatusCompleted, PickupCode: "4002", ReservedOffset: -48 * time.Hour, CreatedOffset: -49 * time.Hour, WithPayment: true},
	{PublicID: "33333333-3333-4333-8333-333333333015", IdempotencyKey: "seed-ord-015", QRToken: "seed-qr-015", CustomerIndex: 0, StoreSlug: "seed-supermarket-sabz", OfferPublicID: "22222222-2222-4222-8222-222222222016", Quantity: 1, Status: enum.OrderStatusCompleted, PickupCode: "4003", ReservedOffset: -72 * time.Hour, CreatedOffset: -73 * time.Hour, WithPayment: true},
	{PublicID: "33333333-3333-4333-8333-333333333016", IdempotencyKey: "seed-ord-016", QRToken: "seed-qr-016", CustomerIndex: 1, StoreSlug: "seed-iranian-golestan", OfferPublicID: "22222222-2222-4222-8222-222222222020", Quantity: 1, Status: enum.OrderStatusCompleted, PickupCode: "4004", ReservedOffset: -96 * time.Hour, CreatedOffset: -97 * time.Hour, WithPayment: true},

	// Rejected / Cancelled (cancelled): 3
	{PublicID: "33333333-3333-4333-8333-333333333017", IdempotencyKey: "seed-ord-017", QRToken: "seed-qr-017", CustomerIndex: 0, StoreSlug: "seed-pizza-araz", OfferPublicID: "22222222-2222-4222-8222-222222222009", Quantity: 1, Status: enum.OrderStatusCancelled, PickupCode: "5001", ReservedOffset: -6 * time.Hour, CreatedOffset: -7 * time.Hour, CancelledBy: "customer", CancelReason: "تغییر برنامه شخصی پیش از مهلت لغو", WithPayment: true},
	{PublicID: "33333333-3333-4333-8333-333333333018", IdempotencyKey: "seed-ord-018", QRToken: "seed-qr-018", CustomerIndex: 1, StoreSlug: "seed-cafe-roshd", OfferPublicID: "22222222-2222-4222-8222-222222222013", Quantity: 1, Status: enum.OrderStatusCancelled, PickupCode: "5002", ReservedOffset: -12 * time.Hour, CreatedOffset: -13 * time.Hour, CancelledBy: "vendor", CancelReason: "اتمام موجودی پیش از آماده‌سازی سفارش", WithPayment: true},
	{PublicID: "33333333-3333-4333-8333-333333333019", IdempotencyKey: "seed-ord-019", QRToken: "seed-qr-019", CustomerIndex: 0, StoreSlug: "seed-supermarket-sabz", OfferPublicID: "22222222-2222-4222-8222-222222222017", Quantity: 1, Status: enum.OrderStatusCancelled, PickupCode: "5003", ReservedOffset: -18 * time.Hour, CreatedOffset: -19 * time.Hour, CancelledBy: "admin", CancelReason: "لغو توسط پشتیبانی به درخواست کاربر", WithPayment: false},

	// Expired (expired): 2 + No-Show (no_show): 1
	{PublicID: "33333333-3333-4333-8333-333333333020", IdempotencyKey: "seed-ord-020", QRToken: "seed-qr-020", CustomerIndex: 0, StoreSlug: "seed-bakery-valiasr", OfferPublicID: "22222222-2222-4222-8222-222222222005", Quantity: 1, Status: enum.OrderStatusExpired, PickupCode: "6001", ReservedOffset: -5 * time.Hour, CreatedOffset: -6 * time.Hour},
	{PublicID: "33333333-3333-4333-8333-333333333021", IdempotencyKey: "seed-ord-021", QRToken: "seed-qr-021", CustomerIndex: 1, StoreSlug: "seed-iranian-golestan", OfferPublicID: "22222222-2222-4222-8222-222222222021", Quantity: 1, Status: enum.OrderStatusExpired, PickupCode: "6002", ReservedOffset: -3 * time.Hour, CreatedOffset: -4 * time.Hour},
	{PublicID: "33333333-3333-4333-8333-333333333022", IdempotencyKey: "seed-ord-022", QRToken: "seed-qr-022", CustomerIndex: 1, StoreSlug: "seed-pizza-araz", OfferPublicID: "22222222-2222-4222-8222-222222222006", Quantity: 1, Status: enum.OrderStatusNoShow, PickupCode: "7001", ReservedOffset: -36 * time.Hour, CreatedOffset: -37 * time.Hour, WithPayment: true},
}

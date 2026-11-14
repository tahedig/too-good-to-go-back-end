package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/tahedig/too-good-to-go/backend/internal/application/dto"
	serviceinterface "github.com/tahedig/too-good-to-go/backend/internal/application/service/interface"
	handlerinterface "github.com/tahedig/too-good-to-go/backend/internal/presentation/handler/interface"
	"github.com/tahedig/too-good-to-go/backend/internal/presentation/middleware"
)

type catalogueHandler struct {
	svc serviceinterface.CatalogueService
}

// NewCatalogueHandler creates a CatalogueHandler.
func NewCatalogueHandler(svc serviceinterface.CatalogueService) handlerinterface.CatalogueHandler {
	return &catalogueHandler{svc: svc}
}

func homeQueryFrom(c *gin.Context) (dto.HomeQuery, error) {
	r := newQueryReader(c)
	q := dto.HomeQuery{
		Lat: r.floatp("lat"), Lng: r.floatp("lng"), RadiusKm: r.floatp("radius_km"),
		Cursor: r.str("cursor"), Limit: r.intp("limit"), UserID: middleware.UserIDPtr(c),
	}
	return q, r.err()
}

func catalogueQueryFrom(c *gin.Context) (dto.CatalogueQuery, error) {
	r := newQueryReader(c)
	q := dto.CatalogueQuery{
		Q:                r.str("q"),
		CategoryIDs:      r.ints("category"),
		Kind:             r.str("kind"),
		PriceMin:         r.int64p("price_min"),
		PriceMax:         r.int64p("price_max"),
		PickupFrom:       r.timep("pickup_from"),
		PickupTo:         r.timep("pickup_to"),
		ExcludeAllergens: r.list("exclude_allergens"),
		Lat:              r.floatp("lat"),
		Lng:              r.floatp("lng"),
		RadiusKm:         r.floatp("radius_km"),
		Sort:             r.str("sort"),
		Cursor:           r.str("cursor"),
		Limit:            r.intp("limit"),
		UserID:           middleware.UserIDPtr(c),
	}
	return q, r.err()
}

// ---------------------------------------------------------------- home

// HomeCategories godoc
// @Summary      Home category shortcuts
// @Description  دسته‌بندی‌های صفحه اصلی.
// @Tags         home
// @Produce      json
// @Success      200 {object} dto.HomeCategoriesResponse
// @Failure      500 {object} dto.ErrorResponse
// @Router       /home/categories [get]
func (h *catalogueHandler) HomeCategories(c *gin.Context) {
	res, err := h.svc.HomeCategories(c.Request.Context())
	if err != nil {
		abortWithError(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

// HomeToday godoc
// @Summary      Today's tasty opportunities
// @Description  فرصت‌های خوشمزه امروز: آفرهایی که بازه تحویل‌شان امروز (به وقت تهران) شروع می‌شود؛ بیشترین تخفیف اول. هر بار ۱۰ مورد؛ برای «مشاهده همه» next_cursor را به عنوان cursor بفرستید.
// @Tags         home
// @Produce      json
// @Param        lat       query number false "User latitude (fills distance)"
// @Param        lng       query number false "User longitude"
// @Param        limit     query int    false "Items per call" minimum(1) maximum(30) default(10)
// @Param        cursor    query string false "next_cursor of the previous call"
// @Param        X-Dev-User-Id header int false "DEV auth: users.id (fills is_favorite)"
// @Success      200 {object} dto.CatalogueOfferList
// @Failure      400 {object} dto.ErrorResponse
// @Failure      401 {object} dto.ErrorResponse
// @Router       /home/today [get]
func (h *catalogueHandler) HomeToday(c *gin.Context) {
	q, err := homeQueryFrom(c)
	if err != nil {
		abortWithError(c, err)
		return
	}
	res, err := h.svc.HomeToday(c.Request.Context(), q)
	if err != nil {
		abortWithError(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

// HomePopularNearby godoc
// @Summary      Popular near you
// @Description  محبوب‌های اطراف شما: داخل شعاع، بیشترین علاقه‌مندی اول، سپس نزدیک‌ترین. بدون lat/lng لیست خالی برمی‌گردد (نه خطا).
// @Tags         home
// @Produce      json
// @Param        lat       query number false "User latitude"
// @Param        lng       query number false "User longitude"
// @Param        radius_km query number false "Radius in km" minimum(0) maximum(100) default(8)
// @Param        limit     query int    false "Items per call" minimum(1) maximum(30) default(10)
// @Param        cursor    query string false "next_cursor of the previous call"
// @Param        X-Dev-User-Id header int false "DEV auth: users.id (fills is_favorite)"
// @Success      200 {object} dto.CatalogueOfferList
// @Failure      400 {object} dto.ErrorResponse
// @Failure      401 {object} dto.ErrorResponse
// @Router       /home/popular-nearby [get]
func (h *catalogueHandler) HomePopularNearby(c *gin.Context) {
	q, err := homeQueryFrom(c)
	if err != nil {
		abortWithError(c, err)
		return
	}
	res, err := h.svc.HomePopularNearby(c.Request.Context(), q)
	if err != nil {
		abortWithError(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

// HomeLastChance godoc
// @Summary      Last chance to save food
// @Description  آخرین فرصت‌های نجات غذا: حداکثر ۳ عدد باقی‌مانده یا پایان بازه تحویل تا ۲ ساعت دیگر؛ زودترین پایان اول.
// @Tags         home
// @Produce      json
// @Param        lat       query number false "User latitude (fills distance)"
// @Param        lng       query number false "User longitude"
// @Param        limit     query int    false "Items per call" minimum(1) maximum(30) default(10)
// @Param        cursor    query string false "next_cursor of the previous call"
// @Param        X-Dev-User-Id header int false "DEV auth: users.id (fills is_favorite)"
// @Success      200 {object} dto.CatalogueOfferList
// @Failure      400 {object} dto.ErrorResponse
// @Failure      401 {object} dto.ErrorResponse
// @Router       /home/last-chance [get]
func (h *catalogueHandler) HomeLastChance(c *gin.Context) {
	q, err := homeQueryFrom(c)
	if err != nil {
		abortWithError(c, err)
		return
	}
	res, err := h.svc.HomeLastChance(c.Request.Context(), q)
	if err != nil {
		abortWithError(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

// ---------------------------------------------------------------- catalogue

// SearchOffers godoc
// @Summary      Offer catalogue
// @Description  کاتالوگ آفرها با جستجو، فیلتر، مرتب‌سازی و pagination (cursor). فقط آفرهای قابل خرید نمایش داده می‌شوند. اگر sort ارسال نشود: با lat/lng نزدیک‌ترین، وگرنه جدیدترین. با تغییر فیلتر یا sort باید cursor را از اول شروع کرد.
// @Tags         offers
// @Produce      json
// @Param        q                 query string   false "Search text (offer title or store name)"
// @Param        category          query []int    false "Category ids, e.g. 1,7 (see /home/categories)" collectionFormat(csv)
// @Param        kind              query string   false "Top tab" Enums(bakery,ready_meal,grocery)
// @Param        price_min         query int      false "Min price (Toman)" minimum(0)
// @Param        price_max         query int      false "Max price (Toman)" minimum(0)
// @Param        pickup_from       query string   false "RFC3339: delivery window must end after this instant"
// @Param        pickup_to         query string   false "RFC3339: delivery window must start before this instant"
// @Param        exclude_allergens query []string false "Hide offers containing ANY of these" Enums(gluten,dairy,egg,peanut,nuts,soy,fish,shellfish,sesame) collectionFormat(csv)
// @Param        lat               query number   false "User latitude (fills distance_m)"
// @Param        lng               query number   false "User longitude"
// @Param        radius_km         query number   false "Only offers within this radius (needs lat & lng)" minimum(0) maximum(100)
// @Param        sort              query string   false "Sort order" Enums(nearest,price_asc,price_desc,newest)
// @Param        limit             query int      false "Page size" minimum(1) maximum(50) default(20)
// @Param        cursor            query string   false "next_cursor of the previous page"
// @Param        X-Dev-User-Id     header int     false "DEV auth: users.id (fills is_favorite)"
// @Success      200 {object} dto.CatalogueOfferList
// @Failure      400 {object} dto.ErrorResponse
// @Failure      401 {object} dto.ErrorResponse
// @Router       /offers [get]
func (h *catalogueHandler) SearchOffers(c *gin.Context) {
	q, err := catalogueQueryFrom(c)
	if err != nil {
		abortWithError(c, err)
		return
	}
	res, err := h.svc.SearchOffers(c.Request.Context(), q)
	if err != nil {
		abortWithError(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

// StoreOffers godoc
// @Summary      Offers of one store
// @Description  آفرهای یک فروشگاه با همان فیلترها و cursor کاتالوگ. {id} همان public id فروشگاه است. فروشگاه ناشناس یا غیرفعال لیست خالی می‌دهد.
// @Tags         offers
// @Produce      json
// @Param        id                path   string   true  "Store public id (UUID)"
// @Param        q                 query string   false "Search text (offer title)"
// @Param        category          query []int    false "Category ids" collectionFormat(csv)
// @Param        kind              query string   false "Top tab" Enums(bakery,ready_meal,grocery)
// @Param        price_min         query int      false "Min price (Toman)" minimum(0)
// @Param        price_max         query int      false "Max price (Toman)" minimum(0)
// @Param        pickup_from       query string   false "RFC3339: delivery window must end after this instant"
// @Param        pickup_to         query string   false "RFC3339: delivery window must start before this instant"
// @Param        exclude_allergens query []string false "Hide offers containing ANY of these" Enums(gluten,dairy,egg,peanut,nuts,soy,fish,shellfish,sesame) collectionFormat(csv)
// @Param        lat               query number   false "User latitude (fills distance_m)"
// @Param        lng               query number   false "User longitude"
// @Param        sort              query string   false "Sort order" Enums(nearest,price_asc,price_desc,newest)
// @Param        limit             query int      false "Page size" minimum(1) maximum(50) default(20)
// @Param        cursor            query string   false "next_cursor of the previous page"
// @Param        X-Dev-User-Id     header int     false "DEV auth: users.id (fills is_favorite)"
// @Success      200 {object} dto.CatalogueOfferList
// @Failure      400 {object} dto.ErrorResponse
// @Failure      401 {object} dto.ErrorResponse
// @Router       /stores/{id}/offers [get]
func (h *catalogueHandler) StoreOffers(c *gin.Context) {
	q, err := catalogueQueryFrom(c)
	if err != nil {
		abortWithError(c, err)
		return
	}
	res, err := h.svc.StoreOffers(c.Request.Context(), c.Param("id"), q)
	if err != nil {
		abortWithError(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

// Filters godoc
// @Summary      Filter options
// @Description  گزینه‌های فیلتر: تب‌ها، دسته‌ها، آلرژن‌ها، بازه قیمت آفرهای فعال و گزینه‌های مرتب‌سازی.
// @Tags         offers
// @Produce      json
// @Success      200 {object} dto.CatalogueFiltersResponse
// @Failure      500 {object} dto.ErrorResponse
// @Router       /offers/filters [get]
func (h *catalogueHandler) Filters(c *gin.Context) {
	res, err := h.svc.Filters(c.Request.Context())
	if err != nil {
		abortWithError(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

// MapOffers godoc
// @Summary      Offers around a point (map)
// @Description  آفرهای اطراف برای نقشه، نزدیک‌ترین اول. lat و lng اجباری است. برای یک پین به ازای هر فروشگاه، سمت کلاینت روی store.id گروه‌بندی کنید.
// @Tags         offers
// @Produce      json
// @Param        lat           query number false "Map centre latitude (required)"
// @Param        lng           query number false "Map centre longitude (required)"
// @Param        radius_km     query number false "Radius in km" minimum(0) maximum(100) default(5)
// @Param        category      query []int  false "Category ids" collectionFormat(csv)
// @Param        kind          query string false "Top tab" Enums(bakery,ready_meal,grocery)
// @Param        limit         query int    false "Max pins" minimum(1) maximum(300) default(100)
// @Param        X-Dev-User-Id header int   false "DEV auth: users.id (fills is_favorite)"
// @Success      200 {object} dto.CatalogueMapResponse
// @Failure      400 {object} dto.ErrorResponse
// @Failure      401 {object} dto.ErrorResponse
// @Router       /offers/map [get]
func (h *catalogueHandler) MapOffers(c *gin.Context) {
	r := newQueryReader(c)
	q := dto.MapQuery{
		Lat: r.floatp("lat"), Lng: r.floatp("lng"), RadiusKm: r.floatp("radius_km"),
		CategoryIDs: r.ints("category"), Kind: r.str("kind"), Limit: r.intp("limit"),
		UserID: middleware.UserIDPtr(c),
	}
	if err := r.err(); err != nil {
		abortWithError(c, err)
		return
	}
	res, err := h.svc.MapOffers(c.Request.Context(), q)
	if err != nil {
		abortWithError(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

// ---------------------------------------------------------------- favorites

// ListFavorites godoc
// @Summary      My favorite offers
// @Description  لیست علاقه‌مندی‌ها، جدیدترین اول. فقط آفرهایی که همین الان قابل خرید هستند نمایش داده می‌شوند.
// @Tags         favorites
// @Produce      json
// @Param        lat           query number false "User latitude (fills distance)"
// @Param        lng           query number false "User longitude"
// @Param        limit         query int    false "Page size" minimum(1) maximum(50) default(20)
// @Param        cursor        query string false "next_cursor of the previous page"
// @Param        X-Dev-User-Id header int   true  "DEV auth: users.id"
// @Success      200 {object} dto.CatalogueOfferList
// @Failure      400 {object} dto.ErrorResponse
// @Failure      401 {object} dto.ErrorResponse
// @Router       /favorites/offers [get]
func (h *catalogueHandler) ListFavorites(c *gin.Context) {
	userID, ok := middleware.UserID(c)
	if !ok { // Required auth runs first; this is a safety net
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}
	r := newQueryReader(c)
	q := dto.FavoritesQuery{Lat: r.floatp("lat"), Lng: r.floatp("lng"), Cursor: r.str("cursor"), Limit: r.intp("limit")}
	if err := r.err(); err != nil {
		abortWithError(c, err)
		return
	}
	res, err := h.svc.ListFavorites(c.Request.Context(), userID, q)
	if err != nil {
		abortWithError(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

// AddFavorite godoc
// @Summary      Add offer to favorites
// @Description  افزودن به علاقه‌مندی. idempotent است: تکرار درخواست خطا نمی‌دهد. {id} همان public id آفر است. آفر ناشناس یا غیرقابل‌نمایش ۴۰۴ می‌دهد.
// @Tags         favorites
// @Produce      json
// @Param        id            path   string true "Offer public id (UUID)"
// @Param        X-Dev-User-Id header int    true "DEV auth: users.id"
// @Success      200 {object} dto.FavoriteOfferResponse
// @Failure      400 {object} dto.ErrorResponse
// @Failure      401 {object} dto.ErrorResponse
// @Failure      404 {object} dto.ErrorResponse
// @Router       /favorites/offers/{id} [put]
func (h *catalogueHandler) AddFavorite(c *gin.Context) {
	userID, ok := middleware.UserID(c)
	if !ok {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}
	res, err := h.svc.AddFavorite(c.Request.Context(), userID, c.Param("id"))
	if err != nil {
		abortWithError(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

// RemoveFavorite godoc
// @Summary      Remove offer from favorites
// @Description  حذف از علاقه‌مندی. idempotent است و همیشه ۲۰۴ برمی‌گرداند (وجود یا عدم وجود آفر لو نمی‌رود).
// @Tags         favorites
// @Param        id            path   string true "Offer public id (UUID)"
// @Param        X-Dev-User-Id header int    true "DEV auth: users.id"
// @Success      204 "No Content"
// @Failure      400 {object} dto.ErrorResponse
// @Failure      401 {object} dto.ErrorResponse
// @Router       /favorites/offers/{id} [delete]
func (h *catalogueHandler) RemoveFavorite(c *gin.Context) {
	userID, ok := middleware.UserID(c)
	if !ok {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}
	if err := h.svc.RemoveFavorite(c.Request.Context(), userID, c.Param("id")); err != nil {
		abortWithError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

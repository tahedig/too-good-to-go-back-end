package handler

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/tahedig/too-good-to-go/backend/internal/application/dto"
	serviceinterface "github.com/tahedig/too-good-to-go/backend/internal/application/service/interface"
	"github.com/tahedig/too-good-to-go/backend/internal/bootstrap"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/exception"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/model"
	handlerinterface "github.com/tahedig/too-good-to-go/backend/internal/presentation/handler/interface"
)

type storeHandler struct {
	svc serviceinterface.StoreService
}

// NewStoreHandler creates a StoreHandler.
func NewStoreHandler(svc serviceinterface.StoreService) handlerinterface.StoreHandler {
	return &storeHandler{svc: svc}
}

// ListStores godoc
// @Summary      List vendor stores
// @Description  Retrieves all stores owned by the authenticated vendor.
// @Tags         vendor
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      200 {object} dto.StoreListResponse
// @Failure      401 {object} dto.ErrorResponse
// @Failure      500 {object} dto.ErrorResponse
// @Router       /vendor/stores [get]
func (h *storeHandler) ListStores(c *gin.Context) {
	userID, ok := storeHandlerUserID(c)
	if !ok {
		abortWithError(c, exception.NewUnauthorized("authentication required"))
		return
	}

	stores, err := h.svc.ListStores(c.Request.Context(), userID)
	if err != nil {
		abortWithError(c, err)
		return
	}

	response := dto.StoreListResponse{
		Stores: make([]dto.StoreResponse, 0, len(stores)),
	}

	for _, store := range stores {
		if store != nil {
			response.Stores = append(response.Stores, toStoreResponse(store))
		}
	}

	c.JSON(http.StatusOK, response)
}

// GetStore godoc
// @Summary      Get vendor store details
// @Description  Retrieves store information for a store owned by the authenticated vendor.
// @Tags         vendor
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "Store Public ID"
// @Success      200 {object} dto.StoreResponse
// @Failure      401 {object} dto.ErrorResponse
// @Failure      404 {object} dto.ErrorResponse
// @Failure      500 {object} dto.ErrorResponse
// @Router       /vendor/stores/{id} [get]
func (h *storeHandler) GetStore(c *gin.Context) {
	userID, ok := storeHandlerUserID(c)
	if !ok {
		abortWithError(c, exception.NewUnauthorized("authentication required"))
		return
	}

	store, err := h.svc.GetStore(
		c.Request.Context(),
		userID,
		getStoreID(c),
	)
	if err != nil {
		abortWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, toStoreResponse(store))
}

// UpdateStore godoc
// @Summary      Update vendor store information
// @Description  Updates editable fields (name, pickup address, category, phone, biography, instructions, working hours, images, location) for a store owned by the authenticated vendor.
// @Tags         vendor
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "Store Public ID"
// @Param        body body dto.UpdateStoreRequest true "Store update payload"
// @Success      200 {object} dto.StoreResponse
// @Failure      400 {object} dto.ErrorResponse
// @Failure      401 {object} dto.ErrorResponse
// @Failure      404 {object} dto.ErrorResponse
// @Failure      500 {object} dto.ErrorResponse
// @Router       /vendor/stores/{id} [patch]
func (h *storeHandler) UpdateStore(c *gin.Context) {
	userID, ok := storeHandlerUserID(c)
	if !ok {
		abortWithError(c, exception.NewUnauthorized("authentication required"))
		return
	}

	var req dto.UpdateStoreRequest
	if err := bindJSON(c, &req); err != nil {
		abortWithError(c, err)
		return
	}

	updated := &model.Store{
		Name:               req.Name,
		AddressLine:        req.PickupAddress,
		CategoryID:         req.CategoryID,
		Phone:              req.Phone,
		Biography:          req.Biography,
		PickupInstructions: req.PickupInstructions,
	}

	if req.Location != nil {
		updated.Location = model.StoreLocation{
			Latitude:  req.Location.Latitude,
			Longitude: req.Location.Longitude,
		}
	}

	if len(req.WorkingHours) > 0 {
		hours := make([]model.WorkingHour, 0, len(req.WorkingHours))
		for _, wh := range req.WorkingHours {
			hours = append(hours, model.WorkingHour{
				DayOfWeek: wh.DayOfWeek,
				DayName:   wh.DayName,
				DayNameFa: wh.DayNameFa,
				OpenTime:  wh.OpenTime,
				CloseTime: wh.CloseTime,
				IsClosed:  wh.IsClosed,
			})
		}
		updated.WorkingHours = hours
	}

	if len(req.Images) > 0 {
		imgs := make([]model.StoreImage, 0, len(req.Images))
		for _, img := range req.Images {
			imgs = append(imgs, model.StoreImage{
				ID:        img.ID,
				URL:       img.URL,
				IsPrimary: img.IsPrimary,
				SortOrder: img.SortOrder,
			})
		}
		updated.Images = imgs
	}

	store, err := h.svc.UpdateStore(
		c.Request.Context(),
		userID,
		getStoreID(c),
		updated,
	)
	if err != nil {
		abortWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, toStoreResponse(store))
}

func getStoreID(c *gin.Context) string {
	if id := c.Param("id"); id != "" {
		return id
	}
	return c.Param("storeID")
}

func storeHandlerUserID(c *gin.Context) (int64, bool) {
	value, exists := c.Get(bootstrap.CtxKeyUserID)
	if !exists {
		return 0, false
	}

	userID, ok := value.(int64)
	if !ok || userID <= 0 {
		return 0, false
	}

	return userID, true
}

func toStoreResponse(store *model.Store) dto.StoreResponse {
	availabilityStatus := "available"

	if !store.IsActive {
		availabilityStatus = "inactive"
	} else if !store.AcceptsOrders {
		availabilityStatus = "not_accepting_orders"
	}

	hoursDTO := make([]dto.WorkingHourDTO, 0, len(store.WorkingHours))
	for _, wh := range store.WorkingHours {
		dayNameFa := wh.DayNameFa
		if dayNameFa == "" {
			dayNameFa = defaultDayNameFa(wh.DayOfWeek)
		}
		hoursDTO = append(hoursDTO, dto.WorkingHourDTO{
			DayOfWeek: wh.DayOfWeek,
			DayName:   wh.DayName,
			DayNameFa: dayNameFa,
			OpenTime:  wh.OpenTime,
			CloseTime: wh.CloseTime,
			IsClosed:  wh.IsClosed,
		})
	}

	workingSummary, closedSummary := buildWorkingHoursSummary(store.WorkingHours)

	imagesDTO := make([]dto.StoreImageDTO, 0, len(store.Images))
	var primaryCover *string
	for _, img := range store.Images {
		imagesDTO = append(imagesDTO, dto.StoreImageDTO{
			ID:        img.ID,
			URL:       img.URL,
			IsPrimary: img.IsPrimary,
			SortOrder: img.SortOrder,
		})
		if img.IsPrimary && primaryCover == nil {
			url := img.URL
			primaryCover = &url
		}
	}

	if primaryCover == nil {
		primaryCover = store.CoverImageURL
	}

	return dto.StoreResponse{
		PublicID:            store.PublicID,
		Name:                store.Name,
		CategoryID:          store.CategoryID,
		CategoryName:        store.CategoryName,
		OwnerName:           store.OwnerName,
		Phone:               store.Phone,
		Biography:           store.Biography,
		PickupAddress:       store.AddressLine,
		PickupInstructions:  store.PickupInstructions,
		Location: dto.StoreLocationResponse{
			Latitude:  store.Location.Latitude,
			Longitude: store.Location.Longitude,
		},
		WorkingHours:        hoursDTO,
		WorkingHoursSummary: workingSummary,
		ClosedDaysSummary:   closedSummary,
		Images:              imagesDTO,
		CoverImageURL:       primaryCover,
		ActiveProductsCount: store.ActiveProductsCount,
		RatingAvg:           store.RatingAvg,
		TrustScore:          store.TrustScore,
		ApprovalStatus:      store.VendorStatus,
		AvailabilityStatus:  availabilityStatus,
		CreatedAt:           store.CreatedAt.Format(time.RFC3339),
		UpdatedAt:           store.UpdatedAt.Format(time.RFC3339),
	}
}

func defaultDayNameFa(dayOfWeek int) string {
	switch dayOfWeek {
	case 0:
		return "شنبه"
	case 1:
		return "یکشنبه"
	case 2:
		return "دوشنبه"
	case 3:
		return "سه‌شنبه"
	case 4:
		return "چهارشنبه"
	case 5:
		return "پنج‌شنبه"
	case 6:
		return "جمعه"
	default:
		return ""
	}
}

func buildWorkingHoursSummary(hours []model.WorkingHour) (string, string) {
	if len(hours) == 0 {
		return "", ""
	}

	var openDays []model.WorkingHour
	var closedDayNames []string

	for _, wh := range hours {
		faName := wh.DayNameFa
		if faName == "" {
			faName = defaultDayNameFa(wh.DayOfWeek)
		}
		if wh.IsClosed {
			closedDayNames = append(closedDayNames, faName)
		} else {
			openDays = append(openDays, wh)
		}
	}

	var workingSummary string
	if len(openDays) > 0 {
		first := openDays[0]
		last := openDays[len(openDays)-1]
		firstFa := first.DayNameFa
		if firstFa == "" {
			firstFa = defaultDayNameFa(first.DayOfWeek)
		}
		lastFa := last.DayNameFa
		if lastFa == "" {
			lastFa = defaultDayNameFa(last.DayOfWeek)
		}

		openTime := toPersianDigits(first.OpenTime)
		closeTime := toPersianDigits(first.CloseTime)

		if len(openDays) == 1 {
			workingSummary = fmt.Sprintf("%s · %s تا %s", firstFa, openTime, closeTime)
		} else {
			workingSummary = fmt.Sprintf("%s تا %s · %s تا %s", firstFa, lastFa, openTime, closeTime)
		}
	}

	var closedSummary string
	if len(closedDayNames) > 0 {
		closedSummary = fmt.Sprintf("%s تعطیل", strings.Join(closedDayNames, " و "))
	}

	return workingSummary, closedSummary
}

func toPersianDigits(s string) string {
	r := strings.NewReplacer(
		"0", "۰",
		"1", "۱",
		"2", "۲",
		"3", "۳",
		"4", "۴",
		"5", "۵",
		"6", "۶",
		"7", "۷",
		"8", "۸",
		"9", "۹",
	)
	return r.Replace(s)
}

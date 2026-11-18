package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/tahedig/too-good-to-go/backend/internal/application/dto"
	serviceinterface "github.com/tahedig/too-good-to-go/backend/internal/application/service/interface"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/model"
	handlerinterface "github.com/tahedig/too-good-to-go/backend/internal/presentation/handler/interface"
)

type adminStoreHandler struct {
	svc serviceinterface.AdminStoreService
}

func NewAdminStoreHandler(svc serviceinterface.AdminStoreService) handlerinterface.AdminStoreHandler {
	return &adminStoreHandler{svc: svc}
}

// GetStore godoc
// @Summary      Get full store details (admin)
// @Description  Returns store info, working hours, and location for the admin store details page.
// @Tags         admin
// @Produce      json
// @Security     BearerAuth
// @Param        id  path  string  true  "Store ID"
// @Success      200 {object} dto.AdminStoreDetailResponse
// @Failure      401 {object} dto.ErrorResponse
// @Failure      403 {object} dto.ErrorResponse
// @Failure      404 {object} dto.ErrorResponse
// @Router       /admin/stores/{id} [get]
func (h *adminStoreHandler) GetStore(c *gin.Context) {
	store, err := h.svc.GetStore(c.Request.Context(), c.Param("id"))
	if err != nil {
		abortWithError(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, toAdminStoreDetailResponse(store))
}

// UpdateStoreStatus godoc
// @Summary      Activate or deactivate a store (admin)
// @Description  Toggles whether a store is active and accepting orders.
// @Tags         admin
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id    path  string                        true  "Store ID"
// @Param        body  body  dto.UpdateAdminStatusRequest  true  "Status"
// @Success      200 {object} dto.AdminStoreDetailResponse
// @Failure      400 {object} dto.ErrorResponse
// @Failure      401 {object} dto.ErrorResponse
// @Failure      403 {object} dto.ErrorResponse
// @Failure      404 {object} dto.ErrorResponse
// @Router       /admin/stores/{id}/status [patch]
func (h *adminStoreHandler) UpdateStoreStatus(c *gin.Context) {
	var body dto.UpdateAdminStatusRequest
	if err := bindJSON(c, &body); err != nil {
		abortWithError(c, err)
		return
	}
	store, err := h.svc.UpdateStoreStatus(c.Request.Context(), c.Param("id"), body.IsActive)
	if err != nil {
		abortWithError(c, err)
		return
	}
	c.JSON(http.StatusOK, toAdminStoreDetailResponse(store))
}

// GetSubscription godoc
// @Summary      Get store seller subscription (admin)
// @Description  Returns the store's current subscription plan, commission rate, period, remaining days, and upgrade request.
// @Tags         admin
// @Produce      json
// @Security     BearerAuth
// @Param        id  path  string  true  "Store ID"
// @Success      200 {object} dto.AdminStoreSubscriptionResponse
// @Failure      401 {object} dto.ErrorResponse
// @Failure      403 {object} dto.ErrorResponse
// @Failure      404 {object} dto.ErrorResponse
// @Router       /admin/stores/{id}/subscription [get]
func (h *adminStoreHandler) GetSubscription(c *gin.Context) {
	sub, err := h.svc.GetStoreSubscription(c.Request.Context(), c.Param("id"))
	if err != nil {
		abortWithError(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, dto.AdminStoreSubscriptionResponse{
		ID:             sub.ID,
		StoreID:        sub.StoreID,
		Status:         sub.Status,
		CurrentPlan:    sub.CurrentPlan,
		CommissionRate: sub.CommissionRate,
		PeriodStart:    sub.PeriodStart,
		PeriodEnd:      sub.PeriodEnd,
		RemainingDays:  sub.RemainingDays,
		UpgradeRequest: sub.UpgradeRequest,
	})
}

// ListOffers godoc
// @Summary      List store offers / surprise bags (admin)
// @Tags         admin
// @Produce      json
// @Security     BearerAuth
// @Param        id  path  string  true  "Store ID"
// @Success      200 {object} dto.AdminOfferListResponse
// @Failure      401 {object} dto.ErrorResponse
// @Failure      403 {object} dto.ErrorResponse
// @Failure      404 {object} dto.ErrorResponse
// @Router       /admin/stores/{id}/offers [get]
func (h *adminStoreHandler) ListOffers(c *gin.Context) {
	offers, err := h.svc.ListStoreOffers(c.Request.Context(), c.Param("id"))
	if err != nil {
		abortWithError(c, err)
		return
	}
	items := make([]dto.AdminOfferResponse, 0, len(offers))
	for i := range offers {
		items = append(items, toAdminOfferResponse(&offers[i]))
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, dto.AdminOfferListResponse{Items: items})
}

// GetOffer godoc
// @Summary      Get offer details (admin)
// @Tags         admin
// @Produce      json
// @Security     BearerAuth
// @Param        offerId  path  string  true  "Offer ID"
// @Success      200 {object} dto.AdminOfferResponse
// @Failure      401 {object} dto.ErrorResponse
// @Failure      403 {object} dto.ErrorResponse
// @Failure      404 {object} dto.ErrorResponse
// @Router       /admin/offers/{offerId} [get]
func (h *adminStoreHandler) GetOffer(c *gin.Context) {
	offer, err := h.svc.GetOffer(c.Request.Context(), c.Param("offerId"))
	if err != nil {
		abortWithError(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, toAdminOfferResponse(offer))
}

// UpdateOfferStatus godoc
// @Summary      Activate or deactivate an offer (admin)
// @Tags         admin
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        offerId  path  string                        true  "Offer ID"
// @Param        body     body  dto.UpdateAdminStatusRequest  true  "Status"
// @Success      200 {object} dto.AdminOfferResponse
// @Failure      400 {object} dto.ErrorResponse
// @Failure      401 {object} dto.ErrorResponse
// @Failure      403 {object} dto.ErrorResponse
// @Failure      404 {object} dto.ErrorResponse
// @Router       /admin/offers/{offerId}/status [patch]
func (h *adminStoreHandler) UpdateOfferStatus(c *gin.Context) {
	var body dto.UpdateAdminStatusRequest
	if err := bindJSON(c, &body); err != nil {
		abortWithError(c, err)
		return
	}
	offer, err := h.svc.UpdateOfferStatus(c.Request.Context(), c.Param("offerId"), body.IsActive)
	if err != nil {
		abortWithError(c, err)
		return
	}
	c.JSON(http.StatusOK, toAdminOfferResponse(offer))
}

func toAdminOfferResponse(o *model.AdminOffer) dto.AdminOfferResponse {
	return dto.AdminOfferResponse{
		ID:                o.ID,
		StoreID:           o.StoreID,
		StoreName:         o.StoreName,
		OwnerName:         o.OwnerName,
		Category:          o.Category,
		Title:             o.Title,
		Description:       o.Description,
		OfferType:         o.OfferType,
		ImageURL:          o.ImageURL,
		OriginalPrice:     o.OriginalPrice,
		DiscountPrice:     o.DiscountPrice,
		QuantityTotal:     o.QuantityTotal,
		QuantityAvailable: o.QuantityAvailable,
		Status:            o.Status,
		IsActive:          o.IsActive,
		PickupStart:       o.PickupStart,
		PickupEnd:         o.PickupEnd,
		CreatedAt:         o.CreatedAt,
	}
}

func toAdminStoreDetailResponse(s *model.AdminStoreDetail) dto.AdminStoreDetailResponse {
	hours := make([]dto.AdminStoreWorkingHourDTO, 0, len(s.WorkingHours))
	for _, h := range s.WorkingHours {
		hours = append(hours, dto.AdminStoreWorkingHourDTO{
			DayOfWeek: h.DayOfWeek,
			OpenTime:  h.OpenTime,
			CloseTime: h.CloseTime,
			IsClosed:  h.IsClosed,
		})
	}
	return dto.AdminStoreDetailResponse{
		ID:           s.ID,
		Name:         s.Name,
		OwnerName:    s.OwnerName,
		Category:     s.Category,
		Phone:        s.Phone,
		Biography:    s.Biography,
		ImageURL:     s.ImageURL,
		IsActive:     s.IsActive,
		VendorStatus: string(s.VendorStatus),
		CreatedAt:    s.CreatedAt,
		Location: dto.AdminStoreLocationResponse{
			Latitude:    s.Location.Latitude,
			Longitude:   s.Location.Longitude,
			City:        s.Location.City,
			District:    s.Location.District,
			AddressLine: s.Location.AddressLine,
		},
		WorkingHours: hours,
	}
}

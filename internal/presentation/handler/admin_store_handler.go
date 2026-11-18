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

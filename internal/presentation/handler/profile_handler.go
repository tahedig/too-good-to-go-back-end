package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/tahedig/too-good-to-go/backend/internal/application/dto"
	serviceinterface "github.com/tahedig/too-good-to-go/backend/internal/application/service/interface"
	"github.com/tahedig/too-good-to-go/backend/internal/bootstrap"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/exception"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/model"
	handlerinterface "github.com/tahedig/too-good-to-go/backend/internal/presentation/handler/interface"
)

type profileHandler struct {
	svc serviceinterface.ProfileService
}

// NewProfileHandler creates a ProfileHandler.
func NewProfileHandler(svc serviceinterface.ProfileService) handlerinterface.ProfileHandler {
	return &profileHandler{svc: svc}
}

// GetProfile godoc
// @Summary   Get the customer profile
// @Tags      customer
// @Produce   json
// @Security  BearerAuth
// @Success   200 {object} dto.ProfileResponse
// @Failure   401 {object} dto.ErrorResponse
// @Failure   403 {object} dto.ErrorResponse
// @Router    /customer/profile [get]
func (h *profileHandler) GetProfile(c *gin.Context) {
	userID := c.GetString(bootstrap.CtxKeyUserID)
	if userID == "" {
		abortWithError(c, exception.NewUnauthorized("authentication required"))
		return
	}

	user, err := h.svc.Get(c.Request.Context(), userID)
	if err != nil {
		abortWithError(c, err)
		return
	}
	c.JSON(http.StatusOK, toProfileResponse(user))
}

// UpdateProfile godoc
// @Summary   Update the customer display name
// @Tags      customer
// @Accept    json
// @Produce   json
// @Security  BearerAuth
// @Param     body body dto.UpdateProfileRequest true "Profile update payload"
// @Success   200 {object} dto.ProfileResponse
// @Failure   400 {object} dto.ErrorResponse
// @Failure   401 {object} dto.ErrorResponse
// @Failure   403 {object} dto.ErrorResponse
// @Router    /customer/profile [patch]
func (h *profileHandler) UpdateProfile(c *gin.Context) {
	userID := c.GetString(bootstrap.CtxKeyUserID)
	if userID == "" {
		abortWithError(c, exception.NewUnauthorized("authentication required"))
		return
	}

	var req dto.UpdateProfileRequest
	if err := bindJSON(c, &req); err != nil {
		abortWithError(c, err)
		return
	}
	user, err := h.svc.UpdateFullName(c.Request.Context(), userID, req.DisplayName)
	if err != nil {
		abortWithError(c, err)
		return
	}
	c.JSON(http.StatusOK, toProfileResponse(user))
}

func toProfileResponse(user *model.User) dto.ProfileResponse {
	return dto.ProfileResponse{UserID: user.ID, DisplayName: user.FullName, Phone: user.Phone}
}

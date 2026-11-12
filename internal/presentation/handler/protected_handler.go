package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/tahedig/too-good-to-go/backend/internal/application/dto"
	"github.com/tahedig/too-good-to-go/backend/internal/bootstrap"
	handlerinterface "github.com/tahedig/too-good-to-go/backend/internal/presentation/handler/interface"
)

type protectedHandler struct{}

func NewProtectedHandler() handlerinterface.ProtectedHandler { return &protectedHandler{} }

func respondIdentity(c *gin.Context) {
	c.JSON(http.StatusOK, dto.MeResponse{
		UserID: c.GetString(bootstrap.CtxKeyUserID),
		Role:   c.GetString(bootstrap.CtxKeyUserRole),
	})
}

// Me godoc
// @Summary  Identity from the access token (any authenticated role)
// @Tags     auth
// @Produce  json
// @Security BearerAuth
// @Success  200 {object} dto.MeResponse
// @Failure  401 {object} dto.ErrorResponse
// @Router   /auth/me [get]
func (h *protectedHandler) Me(c *gin.Context) { respondIdentity(c) }

// CustomerArea godoc
// @Summary  Customer-only test route
// @Tags     rbac
// @Produce  json
// @Security BearerAuth
// @Success  200 {object} dto.MeResponse
// @Failure  401 {object} dto.ErrorResponse
// @Failure  403 {object} dto.ErrorResponse
// @Router   /test/customer [get]
func (h *protectedHandler) CustomerArea(c *gin.Context) { respondIdentity(c) }

// VendorArea godoc
// @Summary  Vendor-only test route
// @Tags     rbac
// @Produce  json
// @Security BearerAuth
// @Success  200 {object} dto.MeResponse
// @Failure  401 {object} dto.ErrorResponse
// @Failure  403 {object} dto.ErrorResponse
// @Router   /test/vendor [get]
func (h *protectedHandler) VendorArea(c *gin.Context) { respondIdentity(c) }

// AdminArea godoc
// @Summary  Admin-only test route
// @Tags     rbac
// @Produce  json
// @Security BearerAuth
// @Success  200 {object} dto.MeResponse
// @Failure  401 {object} dto.ErrorResponse
// @Failure  403 {object} dto.ErrorResponse
// @Router   /test/admin [get]
func (h *protectedHandler) AdminArea(c *gin.Context) { respondIdentity(c) }

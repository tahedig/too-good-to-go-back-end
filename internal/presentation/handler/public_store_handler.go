package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	serviceinterface "github.com/tahedig/too-good-to-go/backend/internal/application/service/interface"
	handlerinterface "github.com/tahedig/too-good-to-go/backend/internal/presentation/handler/interface"
)

type publicStoreHandler struct {
	svc serviceinterface.PublicStoreService
}

// NewPublicStoreHandler creates a PublicStoreHandler.
func NewPublicStoreHandler(svc serviceinterface.PublicStoreService) handlerinterface.PublicStoreHandler {
	return &publicStoreHandler{svc: svc}
}

// GetPublicStore godoc
// @Summary      Get public store details and available offers
// @Description  Returns public store information, schedule, and available food offers for eligible stores. Unknown or unavailable stores return 404 without exposing private fields.
// @Tags         stores
// @Produce      json
// @Param        id path string true "Store Public ID"
// @Success      200 {object} dto.PublicStoreResponse
// @Failure      404 {object} dto.ErrorResponse
// @Failure      500 {object} dto.ErrorResponse
// @Router       /stores/{id} [get]
func (h *publicStoreHandler) GetPublicStore(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		id = c.Param("storeID")
	}

	res, err := h.svc.GetPublicStore(c.Request.Context(), id)
	if err != nil {
		abortWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, res)
}

package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/tahedig/too-good-to-go/backend/internal/application/dto"
	serviceinterface "github.com/tahedig/too-good-to-go/backend/internal/application/service/interface"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/exception"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/model"
	handlerinterface "github.com/tahedig/too-good-to-go/backend/internal/presentation/handler/interface"
)

type storeRequestHandler struct {
	svc serviceinterface.StoreRequestService
}

// NewStoreRequestHandler creates a StoreRequestHandler.
func NewStoreRequestHandler(svc serviceinterface.StoreRequestService) handlerinterface.StoreRequestHandler {
	return &storeRequestHandler{svc: svc}
}

// ListStoreRequests godoc
// @Summary      List vendor store requests
// @Description  Retrieves all store creation/branch requests submitted by the authenticated vendor.
// @Tags         vendor
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      200 {object} dto.StoreRequestListResponse
// @Failure      401 {object} dto.ErrorResponse
// @Failure      500 {object} dto.ErrorResponse
// @Router       /vendor/store-requests [get]
func (h *storeRequestHandler) ListStoreRequests(c *gin.Context) {
	userID, ok := storeHandlerUserID(c)
	if !ok {
		abortWithError(c, exception.NewUnauthorized("authentication required"))
		return
	}

	requests, err := h.svc.ListStoreRequests(c.Request.Context(), userID)
	if err != nil {
		abortWithError(c, err)
		return
	}

	response := dto.StoreRequestListResponse{
		Requests: make([]dto.StoreRequestResponse, 0, len(requests)),
	}

	for _, req := range requests {
		if req != nil {
			response.Requests = append(response.Requests, toStoreRequestResponse(req))
		}
	}

	c.JSON(http.StatusOK, response)
}

// GetStoreRequest godoc
// @Summary      Get store request details
// @Description  Retrieves status and details of a store creation request.
// @Tags         vendor
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "Store Request Public ID"
// @Success      200 {object} dto.StoreRequestResponse
// @Failure      401 {object} dto.ErrorResponse
// @Failure      404 {object} dto.ErrorResponse
// @Failure      500 {object} dto.ErrorResponse
// @Router       /vendor/store-requests/{id} [get]
func (h *storeRequestHandler) GetStoreRequest(c *gin.Context) {
	userID, ok := storeHandlerUserID(c)
	if !ok {
		abortWithError(c, exception.NewUnauthorized("authentication required"))
		return
	}

	req, err := h.svc.GetStoreRequest(
		c.Request.Context(),
		userID,
		getStoreID(c),
	)
	if err != nil {
		abortWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, toStoreRequestResponse(req))
}

// CreateStoreRequest godoc
// @Summary      Submit a new store request
// @Description  Submits a request to create a new store or branch.
// @Tags         vendor
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body body dto.CreateStoreRequestInput true "Store creation request payload"
// @Success      201 {object} dto.StoreRequestResponse
// @Failure      400 {object} dto.ErrorResponse
// @Failure      401 {object} dto.ErrorResponse
// @Failure      500 {object} dto.ErrorResponse
// @Router       /vendor/store-requests [post]
func (h *storeRequestHandler) CreateStoreRequest(c *gin.Context) {
	userID, ok := storeHandlerUserID(c)
	if !ok {
		abortWithError(c, exception.NewUnauthorized("authentication required"))
		return
	}

	var input dto.CreateStoreRequestInput
	if err := bindJSON(c, &input); err != nil {
		abortWithError(c, err)
		return
	}

	reqModel := &model.StoreRequest{
		StoreName:  input.StoreName,
		CategoryID: input.CategoryID,
		OwnerName:  input.OwnerName,
		Address:    input.Address,
	}

	created, err := h.svc.CreateStoreRequest(c.Request.Context(), userID, reqModel)
	if err != nil {
		abortWithError(c, err)
		return
	}

	c.JSON(http.StatusCreated, toStoreRequestResponse(created))
}

func toStoreRequestResponse(req *model.StoreRequest) dto.StoreRequestResponse {
	statusFa := req.StatusFa
	if statusFa == "" {
		switch req.Status {
		case "pending_review":
			statusFa = "در انتظار بررسی"
		case "approved":
			statusFa = "تأییدشده"
		case "rejected":
			statusFa = "رد شده"
		default:
			statusFa = req.Status
		}
	}

	dateStr := req.CreatedAt.Format("2006-01-02")

	return dto.StoreRequestResponse{
		PublicID:     req.PublicID,
		StoreName:    req.StoreName,
		CategoryID:   req.CategoryID,
		CategoryName: req.CategoryName,
		OwnerName:    req.OwnerName,
		Date:         dateStr,
		Address:      req.Address,
		Status:       req.Status,
		StatusFa:     statusFa,
		Notice:       "این شعبه پس از تأیید، امکان انتشار محصول خواهد داشت.",
		CreatedAt:    req.CreatedAt.Format(time.RFC3339),
	}
}

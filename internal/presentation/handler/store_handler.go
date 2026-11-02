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

type storeHandler struct {
	svc serviceinterface.StoreService
}

// NewStoreHandler creates a StoreHandler.
func NewStoreHandler(svc serviceinterface.StoreService) handlerinterface.StoreHandler {
	return &storeHandler{svc: svc}
}

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

func (h *storeHandler) GetStore(c *gin.Context) {
	userID, ok := storeHandlerUserID(c)
	if !ok {
		abortWithError(c, exception.NewUnauthorized("authentication required"))
		return
	}

	store, err := h.svc.GetStore(
		c.Request.Context(),
		userID,
		c.Param("storeID"),
	)
	if err != nil {
		abortWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, toStoreResponse(store))
}

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
		Biography:          req.Biography,
		PickupAddress:      req.PickupAddress,
		PickupInstructions: req.PickupInstructions,
	}

	store, err := h.svc.UpdateStore(
		c.Request.Context(),
		userID,
		c.Param("storeID"),
		updated,
	)
	if err != nil {
		abortWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, toStoreResponse(store))
}

// storeHandlerUserID retrieves the authenticated user's internal ID
// from the Gin context. The expected type is provisional until JWT
// middleware integration is finalized.
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
	return dto.StoreResponse{
		PublicID:  store.PublicID,
		Name:      store.Name,
		Biography: store.Biography,
		Location: dto.StoreLocationResponse{
			Latitude:  store.Location.Latitude,
			Longitude: store.Location.Longitude,
		},
		PickupAddress:      store.PickupAddress,
		PickupInstructions: store.PickupInstructions,
		RatingAvg:          store.RatingAvg,
		TrustScore:         store.TrustScore,
		ApprovalStatus:     store.ApprovalStatus,
		AvailabilityStatus: store.AvailabilityStatus,
		CreatedAt:          store.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:          store.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}

package handler

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/tahedig/too-good-to-go/backend/internal/application/dto"
	serviceinterface "github.com/tahedig/too-good-to-go/backend/internal/application/service/interface"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/model"
	handlerinterface "github.com/tahedig/too-good-to-go/backend/internal/presentation/handler/interface"
)

type adminStoreManagementHandler struct {
	svc serviceinterface.AdminStoreManagementService
}

func NewAdminStoreManagementHandler(svc serviceinterface.AdminStoreManagementService) handlerinterface.AdminStoreManagementHandler {
	return &adminStoreManagementHandler{svc: svc}
}

// ListStores godoc
// @Summary      List and search stores (admin)
// @Description  Filters stores by status, category, city, and accepts_orders. Deleted stores are excluded unless include_deleted=true. Limit defaults to 20 and is capped at 50.
// @Tags         admin
// @Produce      json
// @Security     BearerAuth
// @Param        search query string false "Search store name, slug, city, or owner"
// @Param        status query string false "Approval status"
// @Param        category_id query int false "Category ID"
// @Param        city query string false "Exact city filter"
// @Param        accepts_orders query bool false "Order acceptance filter"
// @Param        include_deleted query bool false "Include soft-deleted stores"
// @Param        sort query string false "Sort order" Enums(newest,oldest)
// @Param        limit query int false "Page size" minimum(1) maximum(50) default(20)
// @Param        cursor query string false "Cursor from the previous page"
// @Success      200 {object} dto.AdminManagedStoreListResponse
// @Failure      400 {object} dto.ErrorResponse
// @Failure      401 {object} dto.ErrorResponse
// @Failure      403 {object} dto.ErrorResponse
// @Router       /admin/stores [get]
func (h *adminStoreManagementHandler) ListStores(c *gin.Context) {
	query := newQueryReader(c)
	in := serviceinterface.AdminStoreListInput{
		Search:     query.str("search"),
		Status:     query.str("status"),
		CategoryID: queryIntPointer(query, "category_id"),
		City:       query.str("city"),
		Sort:       query.str("sort"),
		Limit:      queryIntPointer(query, "limit"),
		Cursor:     query.str("cursor"),
	}
	in.AcceptsOrders = queryBoolPointer(query, "accepts_orders")
	in.IncludeDeleted = queryBoolPointer(query, "include_deleted")
	if err := query.err(); err != nil {
		abortWithError(c, err)
		return
	}

	page, err := h.svc.ListStores(c.Request.Context(), in)
	if err != nil {
		abortWithError(c, err)
		return
	}
	response := dto.AdminManagedStoreListResponse{
		Items:      make([]dto.AdminManagedStoreResponse, 0, len(page.Items)),
		Total:      page.Total,
		HasMore:    page.HasMore,
		NextCursor: page.NextCursor,
	}
	for _, store := range page.Items {
		if store != nil {
			response.Items = append(response.Items, toAdminManagedStoreResponse(store))
		}
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, response)
}

// GetManagedStore godoc
// @Summary      Get store profile (admin)
// @Tags         admin
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "Store public ID"
// @Success      200 {object} dto.AdminManagedStoreResponse
// @Failure      400 {object} dto.ErrorResponse
// @Failure      401 {object} dto.ErrorResponse
// @Failure      403 {object} dto.ErrorResponse
// @Failure      404 {object} dto.ErrorResponse
// @Router       /admin/stores/{id} [get]
func (h *adminStoreManagementHandler) GetManagedStore(c *gin.Context) {
	store, err := h.svc.GetManagedStore(c.Request.Context(), c.Param("id"))
	if err != nil {
		abortWithError(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, toAdminManagedStoreResponse(store))
}

// UpdateManagedStore godoc
// @Summary      Update permitted store profile fields (admin)
// @Description  Updates store profile and accepts_orders. Ownership, approval status, and computed fields are ignored.
// @Tags         admin
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "Store public ID"
// @Param        body body dto.AdminManagedStoreUpdateRequest true "Permitted store profile fields"
// @Success      200 {object} dto.AdminManagedStoreResponse
// @Failure      400 {object} dto.ErrorResponse
// @Failure      401 {object} dto.ErrorResponse
// @Failure      403 {object} dto.ErrorResponse
// @Failure      404 {object} dto.ErrorResponse
// @Router       /admin/stores/{id} [patch]
func (h *adminStoreManagementHandler) UpdateManagedStore(c *gin.Context) {
	var req dto.AdminManagedStoreUpdateRequest
	if err := bindJSON(c, &req); err != nil {
		abortWithError(c, err)
		return
	}
	updated, err := h.svc.UpdateManagedStore(c.Request.Context(), c.Param("id"), toAdminStoreUpdate(&req))
	if err != nil {
		abortWithError(c, err)
		return
	}
	c.JSON(http.StatusOK, toAdminManagedStoreResponse(updated))
}

func queryBoolPointer(query *queryReader, name string) *bool {
	value, exists := query.c.GetQuery(name)
	if !exists {
		return nil
	}
	var parsed bool
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "true":
		parsed = true
	case "false":
		parsed = false
	default:
		query.fail(name, "must be true or false")
		return nil
	}
	return &parsed
}

func queryIntPointer(query *queryReader, name string) *int {
	value, exists := query.c.GetQuery(name)
	if !exists {
		return nil
	}
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		query.fail(name, "must be an integer")
		return nil
	}
	return &parsed
}

func toAdminStoreUpdate(req *dto.AdminManagedStoreUpdateRequest) *model.AdminStoreUpdate {
	updated := &model.AdminStoreUpdate{
		Name:               req.Name,
		Biography:          req.Biography,
		PickupInstructions: req.PickupInstructions,
		Phone:              req.Phone,
		City:               req.City,
		District:           req.District,
		AddressLine:        req.AddressLine,
		CategoryID:         req.CategoryID,
		LogoURL:            req.LogoURL,
		CoverImageURL:      req.CoverImageURL,
		AcceptsOrders:      req.AcceptsOrders,
	}
	if req.Location != nil {
		updated.Location = &model.StoreLocation{
			Latitude:  req.Location.Latitude,
			Longitude: req.Location.Longitude,
		}
	}
	if req.WorkingHours != nil {
		hours := make([]model.WorkingHour, 0, len(*req.WorkingHours))
		for _, hour := range *req.WorkingHours {
			hours = append(hours, model.WorkingHour{
				DayOfWeek: hour.DayOfWeek,
				DayName:   hour.DayName,
				DayNameFa: hour.DayNameFa,
				OpenTime:  hour.OpenTime,
				CloseTime: hour.CloseTime,
				IsClosed:  hour.IsClosed,
			})
		}
		updated.WorkingHours = &hours
	}
	if req.Images != nil {
		images := make([]model.StoreImage, 0, len(*req.Images))
		for _, image := range *req.Images {
			images = append(images, model.StoreImage{
				ID:        image.ID,
				URL:       image.URL,
				IsPrimary: image.IsPrimary,
				SortOrder: image.SortOrder,
			})
		}
		updated.Images = &images
	}
	return updated
}

func toAdminManagedStoreResponse(store *model.Store) dto.AdminManagedStoreResponse {
	hours := make([]dto.AdminStoreWorkingHourDTO, 0, len(store.WorkingHours))
	for _, hour := range store.WorkingHours {
		var openTime, closeTime *string
		if hour.OpenTime != "" {
			open := hour.OpenTime
			openTime = &open
		}
		if hour.CloseTime != "" {
			close := hour.CloseTime
			closeTime = &close
		}
		hours = append(hours, dto.AdminStoreWorkingHourDTO{
			DayOfWeek: hour.DayOfWeek,
			OpenTime:  openTime,
			CloseTime: closeTime,
			IsClosed:  hour.IsClosed,
		})
	}
	images := make([]dto.StoreImageDTO, 0, len(store.Images))
	for _, image := range store.Images {
		images = append(images, dto.StoreImageDTO{
			ID:        image.ID,
			URL:       image.URL,
			IsPrimary: image.IsPrimary,
			SortOrder: image.SortOrder,
		})
	}
	phone := store.ContactPhone
	if store.Phone != nil {
		phone = *store.Phone
	}
	imageURL := store.CoverImageURL
	if imageURL == nil {
		imageURL = store.LogoURL
	}
	return dto.AdminManagedStoreResponse{
		ID: store.PublicID, Name: store.Name, OwnerName: store.OwnerName,
		Category: store.CategoryName, CategoryID: store.CategoryID, CategoryName: store.CategoryName, Phone: phone,
		Biography: store.Biography, PickupInstructions: store.PickupInstructions,
		City: store.City, District: store.District, AddressLine: store.AddressLine,
		Location: dto.AdminStoreLocationResponse{
			Latitude: store.Location.Latitude, Longitude: store.Location.Longitude,
			City: store.City, District: store.District, AddressLine: store.AddressLine,
		},
		ImageURL: imageURL, LogoURL: store.LogoURL, CoverImageURL: store.CoverImageURL,
		Status: store.VendorStatus, IsActive: store.AcceptsOrders,
		VendorStatus: store.VendorStatus, ApprovalStatus: store.VendorStatus,
		AcceptsOrders:       store.AcceptsOrders,
		ActiveProductsCount: store.ActiveProductsCount, RatingAvg: store.RatingAvg,
		RatingCount: store.RatingCount, TrustScore: store.TrustScore, MealsSaved: store.MealsSaved,
		WorkingHours: hours, Images: images, CreatedAt: store.CreatedAt, UpdatedAt: store.UpdatedAt,
	}
}

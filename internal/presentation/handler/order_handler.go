package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/tahedig/too-good-to-go/backend/internal/application/dto"
	serviceinterface "github.com/tahedig/too-good-to-go/backend/internal/application/service/interface"
	"github.com/tahedig/too-good-to-go/backend/internal/bootstrap"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/enum"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/exception"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/model"
	handlerinterface "github.com/tahedig/too-good-to-go/backend/internal/presentation/handler/interface"
)

type orderHandler struct {
	svc serviceinterface.OrderService
}

func NewOrderHandler(svc serviceinterface.OrderService) handlerinterface.OrderHandler {
	return &orderHandler{svc: svc}
}

// ListMyOrders godoc
// @Summary      List my orders
// @Description  Order history of the signed-in user, newest first, with cursor pagination. Only the caller's own orders are returned. upcoming = pending_payment, paid, ready; past = completed, cancelled, expired, no_show. An empty history is 200 with an empty list.
// @Tags         orders
// @Produce      json
// @Security     BearerAuth
// @Param        group   query  string  false  "all (default), upcoming or past"  Enums(all, upcoming, past)
// @Param        status  query  string  false  "Comma-separated statuses, e.g. completed,cancelled. Combined with group, only statuses in both apply."
// @Param        limit   query  int     false  "Page size, 1-50 (default 20)"
// @Param        cursor  query  string  false  "next_cursor from the previous page"
// @Success      200 {object} dto.OrderListResponse
// @Failure      400 {object} dto.ErrorResponse "BAD_REQUEST: invalid group, status, limit or cursor"
// @Failure      401 {object} dto.ErrorResponse
// @Failure      403 {object} dto.ErrorResponse "FORBIDDEN: the account is suspended, deleted or has no role"
// @Failure      500 {object} dto.ErrorResponse
// @Router       /orders [get]
func (h *orderHandler) ListMyOrders(c *gin.Context) {
	page, err := h.svc.ListMyOrders(c.Request.Context(), c.GetString(bootstrap.CtxKeyUserID), serviceinterface.ListOrdersInput{
		Group:  c.Query("group"),
		Status: c.Query("status"),
		Limit:  c.Query("limit"),
		Cursor: c.Query("cursor"),
	})
	if err != nil {
		abortWithError(c, err)
		return
	}

	resp := dto.OrderListResponse{Orders: make([]dto.OrderResponse, 0, len(page.Orders))}
	for i := range page.Orders {
		resp.Orders = append(resp.Orders, toOrderResponse(&page.Orders[i]))
	}
	if page.NextCursor != "" {
		resp.NextCursor = &page.NextCursor
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, resp)
}

func toOrderResponse(o *model.Order) dto.OrderResponse {
	resp := dto.OrderResponse{
		ID:     o.ID,
		Status: string(o.Status),
		Group:  string(o.Status.Group()),
		Store: dto.OrderStoreResponse{
			ID:            o.Store.ID,
			Name:          o.Store.Name,
			PickupAddress: joinAddress(o.Store.City, o.Store.AddressLine),
		},
		Items:       make([]dto.OrderItemResponse, 0, len(o.Items)),
		ItemsTotal:  o.ItemsTotal,
		ServiceFee:  o.ServiceFee,
		TotalAmount: o.TotalAmount,
		PickupStart: o.PickupStart,
		PickupEnd:   o.PickupEnd,
		CreatedAt:   o.CreatedAt,
	}
	if o.PaymentStatus != "" {
		ps := o.PaymentStatus
		resp.PaymentStatus = &ps
	}
	if o.Status == enum.OrderStatusPendingPayment {
		until := o.ReservedUntil
		resp.ReservedUntil = &until
	}
	for _, it := range o.Items {
		resp.Items = append(resp.Items, dto.OrderItemResponse{
			Title: it.Title, OfferType: it.OfferType, Quantity: it.Quantity, UnitPrice: it.UnitPrice,
		})
	}
	return resp
}

// joinAddress formats "city، address line" the way the design shows it.
func joinAddress(city, line string) string {
	switch {
	case city == "":
		return line
	case line == "":
		return city
	}
	return city + "، " + line
}

func getContextUserID(c *gin.Context) string {
	if uid := c.GetString(bootstrap.CtxKeyUserID); uid != "" {
		return uid
	}
	if v, exists := c.Get(bootstrap.CtxKeyUserID); exists {
		if idInt, ok := v.(int64); ok && idInt > 0 {
			return strconv.FormatInt(idInt, 10)
		}
	}
	return ""
}

// GetVendorStore godoc
// @Summary      Get active vendor store
// @Description  Retrieves store information for the active store of the authenticated vendor.
// @Tags         vendor
// @Produce      json
// @Security     BearerAuth
// @Success      200 {object} dto.VendorStoreActiveResponse
// @Failure      401 {object} dto.ErrorResponse
// @Failure      404 {object} dto.ErrorResponse
// @Failure      500 {object} dto.ErrorResponse
// @Router       /vendor/store [get]
func (h *orderHandler) GetVendorStore(c *gin.Context) {
	vendorUserID := getContextUserID(c)
	if vendorUserID == "" {
		abortWithError(c, exception.NewUnauthorized("authentication required"))
		return
	}

	store, err := h.svc.GetVendorStore(c.Request.Context(), vendorUserID)
	if err != nil {
		abortWithError(c, err)
		return
	}

	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, toVendorStoreActiveResponse(store))
}

// ListVendorOrders godoc
// @Summary      List vendor reservations and orders
// @Description  Lists orders and reservations for the vendor's active store with filtering by date and status, and pagination.
// @Tags         vendor
// @Produce      json
// @Security     BearerAuth
// @Param        date   query string false "Filter by pickup date (YYYY-MM-DD)"
// @Param        status query string false "Filter by order status: all, active, ready_for_pickup, delivered, cancelled" Enums(all, active, ready_for_pickup, delivered, cancelled)
// @Param        page   query int    false "Page number (default 1)"
// @Param        limit  query int    false "Items per page (default 20, max 50)"
// @Success      200 {object} dto.VendorOrderListResponse
// @Failure      400 {object} dto.ErrorResponse
// @Failure      401 {object} dto.ErrorResponse
// @Failure      500 {object} dto.ErrorResponse
// @Router       /vendor/orders [get]
func (h *orderHandler) ListVendorOrders(c *gin.Context) {
	vendorUserID := getContextUserID(c)
	if vendorUserID == "" {
		abortWithError(c, exception.NewUnauthorized("authentication required"))
		return
	}

	page := 1
	if pStr := c.Query("page"); pStr != "" {
		if p, err := strconv.Atoi(pStr); err == nil && p > 0 {
			page = p
		}
	}

	limit := 20
	if lStr := c.Query("limit"); lStr != "" {
		if l, err := strconv.Atoi(lStr); err == nil && l > 0 {
			limit = l
		}
	}

	result, err := h.svc.ListVendorOrders(c.Request.Context(), vendorUserID, serviceinterface.VendorOrderListInput{
		Date:   c.Query("date"),
		Status: c.Query("status"),
		Page:   page,
		Limit:  limit,
	})
	if err != nil {
		abortWithError(c, err)
		return
	}

	ordersDTO := make([]dto.VendorOrderItemResponse, 0, len(result.Orders))
	for i := range result.Orders {
		ordersDTO = append(ordersDTO, toVendorOrderItemResponse(&result.Orders[i]))
	}

	resp := dto.VendorOrderListResponse{
		Orders: ordersDTO,
		Pagination: dto.VendorPaginationDTO{
			Page:       result.Page,
			Limit:      result.Limit,
			TotalItems: result.TotalItems,
			TotalPages: result.TotalPages,
		},
	}

	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, resp)
}

// GetVendorOrderSummary godoc
// @Summary      Get vendor orders summary stats
// @Description  Aggregates daily order count, active reservations count, delivered count and total revenue for the vendor.
// @Tags         vendor
// @Produce      json
// @Security     BearerAuth
// @Param        date query string false "Summary date (YYYY-MM-DD, defaults to today)"
// @Success      200 {object} dto.VendorOrderSummaryResponse
// @Failure      401 {object} dto.ErrorResponse
// @Failure      500 {object} dto.ErrorResponse
// @Router       /vendor/orders/summary [get]
func (h *orderHandler) GetVendorOrderSummary(c *gin.Context) {
	vendorUserID := getContextUserID(c)
	if vendorUserID == "" {
		abortWithError(c, exception.NewUnauthorized("authentication required"))
		return
	}

	summary, err := h.svc.GetVendorOrderSummary(c.Request.Context(), vendorUserID, c.Query("date"))
	if err != nil {
		abortWithError(c, err)
		return
	}

	resp := dto.VendorOrderSummaryResponse{
		Date:                    summary.Date,
		TotalReservedPackages:   summary.TotalReservedPackages,
		ActiveReservationsCount: summary.ActiveReservationsCount,
		DeliveredPackagesCount:  summary.DeliveredPackagesCount,
		TotalRevenue:            summary.TotalRevenue,
	}

	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, resp)
}

// GetVendorOrder godoc
// @Summary      Get vendor order details
// @Description  Retrieves detailed information for a specific vendor order.
// @Tags         vendor
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "Order ID"
// @Success      200 {object} dto.VendorOrderItemResponse
// @Failure      400 {object} dto.ErrorResponse
// @Failure      401 {object} dto.ErrorResponse
// @Failure      404 {object} dto.ErrorResponse
// @Failure      500 {object} dto.ErrorResponse
// @Router       /vendor/orders/{id} [get]
func (h *orderHandler) GetVendorOrder(c *gin.Context) {
	vendorUserID := getContextUserID(c)
	if vendorUserID == "" {
		abortWithError(c, exception.NewUnauthorized("authentication required"))
		return
	}

	orderID := c.Param("id")
	if orderID == "" {
		abortWithError(c, exception.NewBadRequest("order id is required", nil))
		return
	}

	order, err := h.svc.GetVendorOrder(c.Request.Context(), vendorUserID, orderID)
	if err != nil {
		abortWithError(c, err)
		return
	}

	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, toVendorOrderItemResponse(order))
}

// VerifyPickup godoc
// @Summary      Verify order pickup
// @Description  Marks an order as delivered/completed after checking pickup code and vendor store ownership.
// @Tags         vendor
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id   path string                  true  "Order ID"
// @Param        body body dto.VerifyPickupRequest false "Pickup verification body"
// @Success      200 {object} dto.VerifyPickupResponse
// @Failure      400 {object} dto.ErrorResponse
// @Failure      401 {object} dto.ErrorResponse
// @Failure      404 {object} dto.ErrorResponse
// @Failure      409 {object} dto.ErrorResponse
// @Failure      500 {object} dto.ErrorResponse
// @Router       /vendor/orders/{id}/verify-pickup [post]
func (h *orderHandler) VerifyPickup(c *gin.Context) {
	vendorUserID := getContextUserID(c)
	if vendorUserID == "" {
		abortWithError(c, exception.NewUnauthorized("authentication required"))
		return
	}

	orderID := c.Param("id")
	if orderID == "" {
		abortWithError(c, exception.NewBadRequest("order id is required", nil))
		return
	}

	var req dto.VerifyPickupRequest
	if err := c.ShouldBindJSON(&req); err != nil && c.Request.ContentLength > 0 {
		abortWithError(c, exception.NewBadRequest("invalid request body", nil))
		return
	}

	res, err := h.svc.VerifyPickup(c.Request.Context(), vendorUserID, orderID, req.PickupCode)
	if err != nil {
		abortWithError(c, err)
		return
	}

	resp := dto.VerifyPickupResponse{
		Success:     res.Success,
		OrderID:     res.OrderID,
		Status:      string(res.Status),
		DeliveredAt: res.DeliveredAt.UTC().Format(time.RFC3339),
		Message:     res.Message,
	}

	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, resp)
}

func toVendorStoreActiveResponse(s *model.VendorStoreActive) dto.VendorStoreActiveResponse {
	return dto.VendorStoreActiveResponse{
		ID:       s.ID,
		Name:     s.Name,
		Address:  s.Address,
		Phone:    s.Phone,
		IsActive: s.IsActive,
	}
}

func toVendorOrderItemResponse(o *model.VendorOrder) dto.VendorOrderItemResponse {
	return dto.VendorOrderItemResponse{
		ID:          o.ID,
		OrderNumber: o.OrderNumber,
		Customer: dto.VendorCustomerDTO{
			ID:    o.Customer.ID,
			Name:  o.Customer.Name,
			Phone: o.Customer.Phone,
		},
		Package: dto.VendorPackageDTO{
			ID:       o.Package.ID,
			Title:    o.Package.Title,
			ImageURL: o.Package.ImageURL,
		},
		Quantity:   o.Quantity,
		UnitPrice:  o.UnitPrice,
		TotalPrice: o.TotalPrice,
		Status:     string(o.Status),
		PickupDate: o.PickupDate,
		PickupTimeSlot: dto.VendorTimeSlotDTO{
			StartTime: o.PickupTimeSlot.StartTime,
			EndTime:   o.PickupTimeSlot.EndTime,
		},
		CreatedAt: o.CreatedAt.UTC().Format(time.RFC3339),
	}
}


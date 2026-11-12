package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/tahedig/too-good-to-go/backend/internal/application/dto"
	serviceinterface "github.com/tahedig/too-good-to-go/backend/internal/application/service/interface"
	"github.com/tahedig/too-good-to-go/backend/internal/bootstrap"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/enum"
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

package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/tahedig/too-good-to-go/backend/internal/application/dto"
	serviceinterface "github.com/tahedig/too-good-to-go/backend/internal/application/service/interface"
	"github.com/tahedig/too-good-to-go/backend/internal/bootstrap"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/model"
	handlerinterface "github.com/tahedig/too-good-to-go/backend/internal/presentation/handler/interface"
)

type vendorDashboardHandler struct {
	svc serviceinterface.VendorDashboardService
}

// NewVendorDashboardHandler creates a VendorDashboardHandler.
func NewVendorDashboardHandler(svc serviceinterface.VendorDashboardService) handlerinterface.VendorDashboardHandler {
	return &vendorDashboardHandler{svc: svc}
}

// GetDashboard godoc
// @Summary      Vendor dashboard summary
// @Description  Returns dashboard metrics, reservation counts by status, today's activity (Asia/Tehran), financial overview, nearest active reservation, pending store applications, and recent reservations scoped strictly to stores owned by the authenticated vendor.
// @Tags         vendor
// @Produce      json
// @Security     BearerAuth
// @Param        store_id      query  string  false  "Optional UUID of an owned store to narrow the summary"
// @Param        recent_limit  query  int     false  "Maximum number of recent reservations, 1-50 (default 10)"
// @Success      200 {object} dto.VendorDashboardResponse
// @Failure      400 {object} dto.ErrorResponse "BAD_REQUEST: invalid store_id or recent_limit"
// @Failure      401 {object} dto.ErrorResponse
// @Failure      403 {object} dto.ErrorResponse "FORBIDDEN: not a vendor"
// @Failure      404 {object} dto.ErrorResponse "NOT_FOUND: store not found or not owned by vendor"
// @Failure      500 {object} dto.ErrorResponse
// @Router       /vendor/dashboard [get]
func (h *vendorDashboardHandler) GetDashboard(c *gin.Context) {
	dash, err := h.svc.GetDashboard(c.Request.Context(), c.GetString(bootstrap.CtxKeyUserID), serviceinterface.VendorDashboardInput{
		StoreID:     c.Query("store_id"),
		RecentLimit: c.Query("recent_limit"),
	})
	if err != nil {
		abortWithError(c, err)
		return
	}

	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, toVendorDashboardResponse(dash))
}

func toVendorDashboardResponse(d *model.VendorDashboard) dto.VendorDashboardResponse {
	resp := dto.VendorDashboardResponse{
		Stores:               make([]dto.VendorDashboardStoreResponse, 0, len(d.Stores)),
		PendingStoreRequests: make([]dto.VendorPendingStoreBannerResponse, 0, len(d.PendingStoreRequests)),
		KPIs: dto.VendorDashboardKPIsResponse{
			ReceivedRevenue:   d.KPIs.ReceivedRevenue,
			ReservedPackages:  d.KPIs.ReservedPackages,
			AvailablePackages: d.KPIs.AvailablePackages,
			ActiveOffers:      d.KPIs.ActiveOffers,
		},
		ReservationCounts: dto.VendorReservationCountsResponse{
			PendingPayment: d.ReservationCounts.PendingPayment,
			Paid:           d.ReservationCounts.Paid,
			Ready:          d.ReservationCounts.Ready,
			Completed:      d.ReservationCounts.Completed,
			Cancelled:      d.ReservationCounts.Cancelled,
			Expired:        d.ReservationCounts.Expired,
			NoShow:         d.ReservationCounts.NoShow,
			Active:         d.ReservationCounts.Active,
			Total:          d.ReservationCounts.Total,
		},
		Today: dto.VendorTodayActivityResponse{
			ReservationsCount: d.Today.ReservationsCount,
			PackagesReserved:  d.Today.PackagesReserved,
			CompletedCount:    d.Today.CompletedCount,
			Revenue:           d.Today.Revenue,
		},
		Financial: dto.VendorDashboardFinancialResponse{
			ReceivedPayout:   d.Financial.ReceivedPayout,
			NetPeriodRevenue: d.Financial.NetPeriodRevenue,
			PendingPayout:    d.Financial.PendingPayout,
			AvailableBalance: d.Financial.AvailableBalance,
			Plan: dto.VendorDashboardPlanResponse{
				Name:           d.Financial.Plan.Name,
				CommissionRate: d.Financial.Plan.CommissionRate,
				PeriodStart:    d.Financial.Plan.PeriodStart,
				PeriodEnd:      d.Financial.Plan.PeriodEnd,
				RemainingDays:  d.Financial.Plan.RemainingDays,
			},
		},
		RecentReservations: make([]dto.VendorDashboardReservationSummary, 0, len(d.RecentReservations)),
	}

	if d.ActiveStore != nil {
		s := toVendorDashboardStoreResponse(*d.ActiveStore)
		resp.ActiveStore = &s
	}
	for _, s := range d.Stores {
		resp.Stores = append(resp.Stores, toVendorDashboardStoreResponse(s))
	}
	for _, p := range d.PendingStoreRequests {
		resp.PendingStoreRequests = append(resp.PendingStoreRequests, dto.VendorPendingStoreBannerResponse{
			ID:        p.ID,
			StoreName: p.StoreName,
			Address:   joinAddress(p.City, p.AddressLine),
			Status:    string(p.Status),
			CreatedAt: p.CreatedAt,
		})
	}
	if n := d.NearestReservation; n != nil {
		resp.NearestReservation = &dto.VendorNearestReservationResponse{
			ID:                     n.ID,
			Status:                 string(n.Status),
			PickupCode:             n.PickupCode,
			CustomerName:           n.CustomerName,
			StoreID:                n.StoreID,
			StoreName:              n.StoreName,
			OfferTitle:             n.OfferTitle,
			OfferImageURL:          n.OfferImageURL,
			Quantity:               n.Quantity,
			TotalAmount:            n.TotalAmount,
			PickupStart:            n.PickupStart,
			PickupEnd:              n.PickupEnd,
			OfferQuantityAvailable: n.OfferQuantityAvailable,
			OfferQuantityReserved:  n.OfferQuantityReserved,
			OfferQuantityTotal:     n.OfferQuantityTotal,
			CreatedAt:              n.CreatedAt,
		}
	}
	for _, r := range d.RecentReservations {
		items := make([]dto.OrderItemResponse, 0, len(r.Items))
		for _, it := range r.Items {
			items = append(items, dto.OrderItemResponse{
				Title:     it.Title,
				OfferType: it.OfferType,
				Quantity:  it.Quantity,
				UnitPrice: it.UnitPrice,
			})
		}
		var payStatus *string
		if r.PaymentStatus != "" {
			ps := r.PaymentStatus
			payStatus = &ps
		}
		resp.RecentReservations = append(resp.RecentReservations, dto.VendorDashboardReservationSummary{
			ID:            r.ID,
			Status:        string(r.Status),
			PaymentStatus: payStatus,
			PickupCode:    r.PickupCode,
			CustomerName:  r.CustomerName,
			StoreID:       r.StoreID,
			StoreName:     r.StoreName,
			Items:         items,
			ItemsTotal:    r.ItemsTotal,
			TotalAmount:   r.TotalAmount,
			VendorPayout:  r.VendorPayout,
			PickupStart:   r.PickupStart,
			PickupEnd:     r.PickupEnd,
			CreatedAt:     r.CreatedAt,
		})
	}

	return resp
}

func toVendorDashboardStoreResponse(s model.VendorDashboardStore) dto.VendorDashboardStoreResponse {
	return dto.VendorDashboardStoreResponse{
		ID:            s.ID,
		Name:          s.Name,
		Address:       joinAddress(s.City, s.AddressLine),
		Status:        string(s.Status),
		AcceptsOrders: s.AcceptsOrders,
		CreatedAt:     s.CreatedAt,
	}
}

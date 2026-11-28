package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/tahedig/too-good-to-go/backend/internal/application/dto"
	serviceinterface "github.com/tahedig/too-good-to-go/backend/internal/application/service/interface"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/model"
	handlerinterface "github.com/tahedig/too-good-to-go/backend/internal/presentation/handler/interface"
)

type adminDashboardHandler struct {
	svc serviceinterface.AdminDashboardService
}

// NewAdminDashboardHandler creates an AdminDashboardHandler.
func NewAdminDashboardHandler(svc serviceinterface.AdminDashboardService) handlerinterface.AdminDashboardHandler {
	return &adminDashboardHandler{svc: svc}
}

// GetDashboard godoc
// @Summary      Admin dashboard summary
// @Description  Returns admin-only platform totals for customers, vendors, stores, active offers, reservations, platform commission, application status breakdown, reservation status breakdown, and a concise recent-activity feed.
// @Tags         admin
// @Produce      json
// @Security     BearerAuth
// @Param        activity_limit  query  int     false  "Maximum number of recent-activity items, 1-50 (default 10)"
// @Param        activity_type   query  string  false  "Comma-separated filter: store_application, store_decision, reservation"
// @Success      200 {object} dto.AdminDashboardResponse
// @Failure      400 {object} dto.ErrorResponse "BAD_REQUEST: invalid activity_limit or activity_type"
// @Failure      401 {object} dto.ErrorResponse
// @Failure      403 {object} dto.ErrorResponse "FORBIDDEN: not an admin"
// @Failure      500 {object} dto.ErrorResponse
// @Router       /admin/dashboard [get]
func (h *adminDashboardHandler) GetDashboard(c *gin.Context) {
	dash, err := h.svc.GetDashboard(c.Request.Context(), serviceinterface.AdminDashboardInput{
		ActivityLimit: c.Query("activity_limit"),
		ActivityType:  c.Query("activity_type"),
	})
	if err != nil {
		abortWithError(c, err)
		return
	}

	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, toAdminDashboardResponse(dash))
}

func toAdminDashboardResponse(d *model.AdminDashboard) dto.AdminDashboardResponse {
	resp := dto.AdminDashboardResponse{
		Totals: dto.AdminDashboardTotalsResponse{
			Customers:             d.Totals.Customers,
			Vendors:               d.Totals.Vendors,
			RegisteredStores:      d.Totals.RegisteredStores,
			ActiveStores:          d.Totals.ActiveStores,
			ActiveOffers:          d.Totals.ActiveOffers,
			PendingApplications:   d.Totals.PendingApplications,
			TotalReservations:     d.Totals.TotalReservations,
			ActiveReservations:    d.Totals.ActiveReservations,
			CompletedReservations: d.Totals.CompletedReservations,
			PlatformCommission:    d.Totals.PlatformCommission,
		},
		Applications: dto.AdminApplicationBreakdownResponse{
			Pending:   d.Applications.Pending,
			Approved:  d.Applications.Approved,
			Rejected:  d.Applications.Rejected,
			Suspended: d.Applications.Suspended,
		},
		Reservations: dto.AdminReservationBreakdownResponse{
			PendingPayment: d.Reservations.PendingPayment,
			Paid:           d.Reservations.Paid,
			Ready:          d.Reservations.Ready,
			Completed:      d.Reservations.Completed,
			Cancelled:      d.Reservations.Cancelled,
			Expired:        d.Reservations.Expired,
			NoShow:         d.Reservations.NoShow,
			Active:         d.Reservations.Active,
			Total:          d.Reservations.Total,
		},
		RecentActivity: make([]dto.AdminActivityItemResponse, 0, len(d.RecentActivity)),
	}

	for _, item := range d.RecentActivity {
		resp.RecentActivity = append(resp.RecentActivity, dto.AdminActivityItemResponse{
			Type:       string(item.Type),
			EntityID:   item.EntityID,
			Title:      item.Title,
			ActorName:  item.ActorName,
			Status:     item.Status,
			Action:     item.Action,
			Amount:     item.Amount,
			OccurredAt: item.OccurredAt,
		})
	}
	return resp
}

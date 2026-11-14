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

type vendorApprovalHandler struct {
	svc serviceinterface.VendorApprovalService
}

func NewVendorApprovalHandler(svc serviceinterface.VendorApprovalService) handlerinterface.VendorApprovalHandler {
	return &vendorApprovalHandler{svc: svc}
}

// ListVendors godoc
// @Summary      List stores for review (admin)
// @Description  Stores newest submitted first, with filters and page-number pagination. total counts the filtered stores; counts counts every store per status, ignoring the filters. Approval is per store, so a "vendor" here is a store.
// @Tags         admin
// @Produce      json
// @Security     BearerAuth
// @Param        status     query  string  false  "Comma-separated: pending, approved, rejected, suspended"
// @Param        search     query  string  false  "Case-insensitive substring of store name, owner name, category or address (max 100 characters)"
// @Param        page       query  int     false  "1-based page (default 1)"
// @Param        page_size  query  int     false  "1-100 (default 20)"
// @Success      200 {object} dto.VendorListResponse
// @Failure      400 {object} dto.ErrorResponse
// @Failure      401 {object} dto.ErrorResponse
// @Failure      403 {object} dto.ErrorResponse "FORBIDDEN: not an admin"
// @Router       /admin/vendors [get]
func (h *vendorApprovalHandler) ListVendors(c *gin.Context) {
	res, err := h.svc.ListVendors(c.Request.Context(), serviceinterface.VendorListInput{
		Status:   c.Query("status"),
		Search:   c.Query("search"),
		Page:     c.Query("page"),
		PageSize: c.Query("page_size"),
	})
	if err != nil {
		abortWithError(c, err)
		return
	}
	resp := dto.VendorListResponse{
		Items:    make([]dto.VendorResponse, 0, len(res.Items)),
		Total:    res.Total,
		Page:     res.Page,
		PageSize: res.PageSize,
		Counts: dto.VendorCountsResponse{
			Pending:   res.Counts[enum.StoreStatusPending],
			Approved:  res.Counts[enum.StoreStatusApproved],
			Rejected:  res.Counts[enum.StoreStatusRejected],
			Suspended: res.Counts[enum.StoreStatusSuspended],
		},
	}
	for i := range res.Items {
		resp.Items = append(resp.Items, toVendorResponse(&res.Items[i]))
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, resp)
}

// GetVendor godoc
// @Summary      Get one store for review (admin)
// @Tags         admin
// @Produce      json
// @Security     BearerAuth
// @Param        id  path  string  true  "Store ID"
// @Success      200 {object} dto.VendorResponse
// @Failure      401 {object} dto.ErrorResponse
// @Failure      403 {object} dto.ErrorResponse
// @Failure      404 {object} dto.ErrorResponse
// @Router       /admin/vendors/{id} [get]
func (h *vendorApprovalHandler) GetVendor(c *gin.Context) {
	store, err := h.svc.GetVendor(c.Request.Context(), c.Param("id"))
	if err != nil {
		abortWithError(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, toVendorResponse(store))
}

// Approve godoc
// @Summary      Approve a pending store (admin)
// @Description  pending -> approved. The store can then publish offers and is shown to customers. No body.
// @Tags         admin
// @Produce      json
// @Security     BearerAuth
// @Param        id  path  string  true  "Store ID"
// @Success      200 {object} dto.VendorResponse
// @Failure      401 {object} dto.ErrorResponse
// @Failure      403 {object} dto.ErrorResponse
// @Failure      404 {object} dto.ErrorResponse
// @Failure      409 {object} dto.ErrorResponse "CONFLICT: not pending, or changed meanwhile"
// @Router       /admin/vendors/{id}/approve [post]
func (h *vendorApprovalHandler) Approve(c *gin.Context) {
	h.decide(c, enum.StoreActionApprove)
}

// Reject godoc
// @Summary      Reject a pending store (admin)
// @Description  pending -> rejected (final). reason_code: unverifiable_address, invalid_owner_info, invalid_contact, missing_documents, ineligible_category, other. message: 20-500 characters, shown to the vendor.
// @Tags         admin
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id    path  string                     true  "Store ID"
// @Param        body  body  dto.VendorDecisionRequest  true  "Reason"
// @Success      200 {object} dto.VendorResponse
// @Failure      400 {object} dto.ErrorResponse "BAD_REQUEST: invalid reason_code or message"
// @Failure      401 {object} dto.ErrorResponse
// @Failure      403 {object} dto.ErrorResponse
// @Failure      404 {object} dto.ErrorResponse
// @Failure      409 {object} dto.ErrorResponse "CONFLICT: not pending, or changed meanwhile"
// @Router       /admin/vendors/{id}/reject [post]
func (h *vendorApprovalHandler) Reject(c *gin.Context) {
	h.decide(c, enum.StoreActionReject)
}

// Suspend godoc
// @Summary      Suspend an approved store (admin)
// @Description  approved -> suspended. Its offers stop being publishable and visible. Existing paid orders stay valid and must be fulfilled. reason_code: customer_complaints, missed_pickups, frequent_cancellations, expired_license, policy_violation, unpaid_balance, vendor_request, other. message: 20-500 characters.
// @Tags         admin
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id    path  string                     true  "Store ID"
// @Param        body  body  dto.VendorDecisionRequest  true  "Reason"
// @Success      200 {object} dto.VendorResponse
// @Failure      400 {object} dto.ErrorResponse "BAD_REQUEST: invalid reason_code or message"
// @Failure      401 {object} dto.ErrorResponse
// @Failure      403 {object} dto.ErrorResponse
// @Failure      404 {object} dto.ErrorResponse
// @Failure      409 {object} dto.ErrorResponse "CONFLICT: not approved, or changed meanwhile"
// @Router       /admin/vendors/{id}/suspend [post]
func (h *vendorApprovalHandler) Suspend(c *gin.Context) {
	h.decide(c, enum.StoreActionSuspend)
}

// Reactivate godoc
// @Summary      Reactivate a suspended store (admin)
// @Description  suspended -> approved; the decision is cleared. No body.
// @Tags         admin
// @Produce      json
// @Security     BearerAuth
// @Param        id  path  string  true  "Store ID"
// @Success      200 {object} dto.VendorResponse
// @Failure      401 {object} dto.ErrorResponse
// @Failure      403 {object} dto.ErrorResponse
// @Failure      404 {object} dto.ErrorResponse
// @Failure      409 {object} dto.ErrorResponse "CONFLICT: not suspended, or changed meanwhile"
// @Router       /admin/vendors/{id}/reactivate [post]
func (h *vendorApprovalHandler) Reactivate(c *gin.Context) {
	h.decide(c, enum.StoreActionReactivate)
}

func (h *vendorApprovalHandler) decide(c *gin.Context, action enum.StoreAction) {
	var body dto.VendorDecisionRequest
	if action.NeedsReason() {
		if err := bindJSON(c, &body); err != nil {
			abortWithError(c, err)
			return
		}
	}
	store, err := h.svc.Decide(c.Request.Context(), c.GetString(bootstrap.CtxKeyUserID), c.Param("id"), action,
		serviceinterface.VendorDecisionInput{ReasonCode: body.ReasonCode, Message: body.Message})
	if err != nil {
		abortWithError(c, err)
		return
	}
	c.JSON(http.StatusOK, toVendorResponse(store))
}

func toVendorResponse(s *model.VendorStore) dto.VendorResponse {
	resp := dto.VendorResponse{
		ID:              s.ID,
		StoreName:       s.StoreName,
		OwnerName:       s.OwnerName,
		Category:        s.Category,
		Address:         s.Address,
		Phone:           s.Phone,
		Description:     s.Description,
		ImageURL:        s.ImageURL,
		SubmittedAt:     s.SubmittedAt,
		Status:          string(s.Status),
		StatusUpdatedAt: s.StatusUpdatedAt,
	}
	if d := s.Decision; d != nil {
		decisionType := "rejection"
		if d.Action == enum.StoreActionSuspend {
			decisionType = "suspension"
		}
		resp.Decision = &dto.VendorDecisionResponse{
			Type: decisionType, ReasonCode: d.ReasonCode, Message: d.Message,
			DecidedAt: d.DecidedAt, DecidedBy: d.DecidedBy,
		}
	}
	return resp
}

package service

import (
	"context"
	"errors"
	"log/slog"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	serviceinterface "github.com/tahedig/too-good-to-go/backend/internal/application/service/interface"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/enum"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/exception"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/model"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/repository"
	"github.com/tahedig/too-good-to-go/backend/pkg/logger"
)

const (
	DefaultVendorPageSize = 20
	MaxVendorPageSize     = 100
	maxVendorSearchLen    = 100
	maxVendorPage         = 100000 // keeps (page-1)*pageSize far from overflow
	minDecisionMessageLen = 20
	maxDecisionMessageLen = 500
)

// Messages the admin panel shows to the admin as they are.
const (
	msgVendorNotFound      = "فروشگاه پیدا نشد."
	msgVendorInvalidInput  = "اطلاعات واردشده معتبر نیست."
	msgVendorStatusChanged = "وضعیت این فروشگاه در این فاصله تغییر کرده است. صفحه را تازه کنید."
)

var msgTransitionNotAllowed = map[enum.StoreAction]string{
	enum.StoreActionApprove:    "فقط درخواست‌های در انتظار بررسی را می‌توان تأیید کرد.",
	enum.StoreActionReject:     "فقط درخواست‌های در انتظار بررسی را می‌توان رد کرد.",
	enum.StoreActionSuspend:    "فقط فروشگاه‌های تأییدشده را می‌توان تعلیق کرد.",
	enum.StoreActionReactivate: "فقط فروشگاه‌های تعلیق‌شده را می‌توان دوباره فعال کرد.",
}

type vendorApprovalService struct {
	stores repository.VendorApprovalRepository
	log    *slog.Logger
}

func NewVendorApprovalService(stores repository.VendorApprovalRepository, log *slog.Logger) serviceinterface.VendorApprovalService {
	return &vendorApprovalService{stores: stores, log: log}
}

func (s *vendorApprovalService) ListVendors(ctx context.Context, in serviceinterface.VendorListInput) (*serviceinterface.VendorStoreList, error) {
	var fields []exception.FieldError
	page, pageSize := 1, DefaultVendorPageSize
	if in.Page != "" {
		n, err := strconv.Atoi(in.Page)
		if err != nil || n < 1 || n > maxVendorPage {
			fields = append(fields, exception.FieldError{Field: "page", Message: "شماره صفحه باید ۱ یا بیشتر باشد."})
		}
		page = n
	}
	if in.PageSize != "" {
		n, err := strconv.Atoi(in.PageSize)
		if err != nil || n < 1 || n > MaxVendorPageSize {
			fields = append(fields, exception.FieldError{Field: "page_size", Message: "اندازه صفحه باید بین ۱ تا ۱۰۰ باشد."})
		}
		pageSize = n
	}
	var statuses []enum.StoreStatus
	if in.Status != "" {
		for _, raw := range strings.Split(in.Status, ",") {
			st := enum.StoreStatus(strings.TrimSpace(raw))
			if !st.Valid() {
				fields = append(fields, exception.FieldError{Field: "status",
					Message: "وضعیت باید یکی از pending، approved، rejected یا suspended باشد."})
				break
			}
			if !slices.Contains(statuses, st) {
				statuses = append(statuses, st)
			}
		}
	}
	search := strings.TrimSpace(in.Search)
	if utf8.RuneCountInString(search) > maxVendorSearchLen {
		fields = append(fields, exception.FieldError{Field: "search", Message: "عبارت جستجو حداکثر ۱۰۰ نویسه است."})
	}
	if len(fields) > 0 {
		return nil, exception.NewBadRequest(msgVendorInvalidInput, fields)
	}

	res, err := s.stores.List(ctx, repository.VendorStoreQuery{
		Statuses: statuses, Search: search, Offset: (page - 1) * pageSize, Limit: pageSize,
	})
	if err != nil {
		return nil, exception.NewInternalServer(err)
	}
	return &serviceinterface.VendorStoreList{
		Items: res.Items, Total: res.Total, Page: page, PageSize: pageSize, Counts: res.Counts,
	}, nil
}

func (s *vendorApprovalService) GetVendor(ctx context.Context, storeID string) (*model.VendorStore, error) {
	store, err := s.stores.Get(ctx, storeID)
	if errors.Is(err, repository.ErrStoreNotFound) {
		return nil, exception.NewNotFound(msgVendorNotFound)
	}
	if err != nil {
		return nil, exception.NewInternalServer(err)
	}
	return store, nil
}

// Decide checks, in order: the store exists (404), the body is valid (400),
// the transition is allowed from the current status (409). The repository
// re-checks the status atomically, so a concurrent decision also gets 409.
func (s *vendorApprovalService) Decide(ctx context.Context, adminID, storeID string, action enum.StoreAction,
	in serviceinterface.VendorDecisionInput) (*model.VendorStore, error) {
	from, to, ok := action.Transition()
	if !ok {
		return nil, exception.NewNotFound("route not found")
	}
	if adminID == "" {
		return nil, exception.NewUnauthorized("authentication required")
	}

	current, err := s.GetVendor(ctx, storeID)
	if err != nil {
		return nil, err
	}

	t := model.StoreTransition{StoreID: storeID, Action: action, From: from, To: to, AdminUserID: adminID}
	if action.NeedsReason() {
		reason, message := strings.TrimSpace(in.ReasonCode), strings.TrimSpace(in.Message)
		var fields []exception.FieldError
		if !action.ValidReason(reason) {
			fields = append(fields, exception.FieldError{Field: "reason_code", Message: "دلیل نامعتبر است."})
		}
		if n := utf8.RuneCountInString(message); n < minDecisionMessageLen || n > maxDecisionMessageLen {
			fields = append(fields, exception.FieldError{Field: "message", Message: "حداقل ۲۰ و حداکثر ۵۰۰ نویسه بنویسید."})
		}
		if len(fields) > 0 {
			return nil, exception.NewBadRequest(msgVendorInvalidInput, fields)
		}
		t.ReasonCode, t.Message = reason, message
	}

	if current.Status != from {
		return nil, exception.NewConflict(msgTransitionNotAllowed[action])
	}
	store, err := s.stores.Transition(ctx, t)
	switch {
	case errors.Is(err, repository.ErrStoreStatusChanged):
		return nil, exception.NewConflict(msgVendorStatusChanged)
	case errors.Is(err, repository.ErrStoreNotFound):
		return nil, exception.NewNotFound(msgVendorNotFound)
	case err != nil:
		return nil, exception.NewInternalServer(err)
	}

	logger.FromContext(ctx, s.log).Info("store status changed",
		"store_id", storeID, "action", string(action), "from", string(from), "to", string(to),
		"reason_code", t.ReasonCode, "admin_id", adminID)
	return store, nil
}

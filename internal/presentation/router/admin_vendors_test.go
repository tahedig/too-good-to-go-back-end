package router_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/tahedig/too-good-to-go/backend/internal/application/dto"
	"github.com/tahedig/too-good-to-go/backend/internal/application/service"
	"github.com/tahedig/too-good-to-go/backend/internal/bootstrap"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/enum"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/exception"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/model"
	"github.com/tahedig/too-good-to-go/backend/internal/infrastructure/repository/memory"
	"github.com/tahedig/too-good-to-go/backend/internal/infrastructure/token"
	"github.com/tahedig/too-good-to-go/backend/internal/presentation/handler"
	"github.com/tahedig/too-good-to-go/backend/internal/presentation/middleware"
	"github.com/tahedig/too-good-to-go/backend/internal/presentation/router"
	"github.com/tahedig/too-good-to-go/backend/pkg/logger"
)

const (
	pendingStore  = "00000000-0000-4000-8000-000000000001"
	approvedStore = "00000000-0000-4000-8000-000000000002"
	adminOrigin   = "http://localhost:5173"
)

type adminEnv struct {
	r      *gin.Engine
	stores *memory.VendorApprovalRepository
	tokens map[string]string // role -> access token
}

func newAdminEnv(t *testing.T) *adminEnv {
	t.Helper()
	log := logger.New("debug", &bytes.Buffer{})
	sessions := service.NewSessionService(memory.NewUserRepository(), token.NewJWTManager(rbacSecret, rbacIssuer, time.Hour),
		map[string]enum.Role{rbacVendorPhone: enum.RoleVendor, rbacAdminPhone: enum.RoleAdmin}, log)
	stores := memory.NewVendorApprovalRepository()
	at := time.Date(2026, 10, 19, 7, 10, 0, 0, time.UTC)
	stores.Add(model.VendorStore{ID: pendingStore, StoreName: "نانوایی سنتی", OwnerName: "سمیرا محمدی",
		Category: "نانوایی", Address: "خیابان شریعتی، پلاک ۲۴", Phone: "+989121111111",
		Description: "نان و شیرینی پایان روز", SubmittedAt: at, Status: enum.StoreStatusPending})
	stores.Add(model.VendorStore{ID: approvedStore, StoreName: "کافه رز", SubmittedAt: at.Add(-time.Hour),
		Status: enum.StoreStatusApproved})

	r, err := router.New(&bootstrap.Config{CORSAllowedOrigins: []string{adminOrigin}}, log, middleware.NewErrorHandler(log),
		middleware.NewAuthMiddleware(sessions), nil, router.Handlers{
			Health:         handler.NewHealthHandler(service.NewHealthService("test", time.Second, log)),
			Auth:           handler.NewAuthHandler(nil, nil),
			VendorApproval: handler.NewVendorApprovalHandler(service.NewVendorApprovalService(stores, log)),
		})
	if err != nil {
		t.Fatal(err)
	}
	e := &adminEnv{r: r, stores: stores, tokens: map[string]string{}}
	for role, phone := range map[string]string{"customer": rbacCustomerPhone, "vendor": rbacVendorPhone, "admin": rbacAdminPhone} {
		res, err := sessions.Login(context.Background(), phone)
		if err != nil {
			t.Fatal(err)
		}
		e.tokens[role] = res.AccessToken
	}
	return e
}

func (e *adminEnv) call(method, path, role, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if role != "" {
		req.Header.Set("Authorization", "Bearer "+e.tokens[role])
	}
	w := httptest.NewRecorder()
	e.r.ServeHTTP(w, req)
	return w
}

func TestAdminVendorsRequireAdmin(t *testing.T) {
	e := newAdminEnv(t)
	routes := []struct{ method, path string }{
		{http.MethodGet, "/admin/vendors"},
		{http.MethodGet, "/admin/vendors/" + pendingStore},
		{http.MethodPost, "/admin/vendors/" + pendingStore + "/approve"},
		{http.MethodPost, "/admin/vendors/" + pendingStore + "/reject"},
		{http.MethodPost, "/admin/vendors/" + approvedStore + "/suspend"},
		{http.MethodPost, "/admin/vendors/" + approvedStore + "/reactivate"},
	}
	for _, rt := range routes {
		for role, want := range map[string]int{"": http.StatusUnauthorized, "customer": http.StatusForbidden, "vendor": http.StatusForbidden} {
			if w := e.call(rt.method, rt.path, role, `{}`); w.Code != want {
				t.Errorf("%s %s as %q: got %d, want %d", rt.method, rt.path, role, w.Code, want)
			}
		}
	}
	if len(e.stores.History()) != 0 {
		t.Fatal("a non-admin request changed a store")
	}
}

func TestAdminListAndGetVendor(t *testing.T) {
	e := newAdminEnv(t)
	w := e.call(http.MethodGet, "/admin/vendors?status=pending,approved&page_size=1", "admin", "")
	var list dto.VendorListResponse
	if w.Code != http.StatusOK || json.Unmarshal(w.Body.Bytes(), &list) != nil {
		t.Fatalf("list: %d %s", w.Code, w.Body.String())
	}
	if list.Total != 2 || list.Page != 1 || list.PageSize != 1 || len(list.Items) != 1 || list.Items[0].ID != pendingStore ||
		list.Counts != (dto.VendorCountsResponse{Pending: 1, Approved: 1}) {
		t.Fatalf("list: %+v", list)
	}
	for _, field := range []string{`"email":null`, `"decision":null`, `"status_updated_at":null`, `"submitted_at":"2026-10-19T07:10:00Z"`} {
		if !strings.Contains(w.Body.String(), field) {
			t.Errorf("list body misses %s: %s", field, w.Body.String())
		}
	}

	if w := e.call(http.MethodGet, "/admin/vendors/"+pendingStore, "admin", ""); w.Code != http.StatusOK ||
		!strings.Contains(w.Body.String(), `"owner_name":"سمیرا محمدی"`) {
		t.Fatalf("get: %d %s", w.Code, w.Body.String())
	}
	w = e.call(http.MethodGet, "/admin/vendors/00000000-0000-4000-8000-000000000099", "admin", "")
	if w.Code != http.StatusNotFound || decodeError(t, w).Message != "فروشگاه پیدا نشد." {
		t.Fatalf("missing store: %d %s", w.Code, w.Body.String())
	}
	if w := e.call(http.MethodGet, "/admin/vendors?page_size=500", "admin", ""); w.Code != http.StatusBadRequest {
		t.Fatalf("bad page_size: %d", w.Code)
	}
}

func TestAdminDecisionsOverHTTP(t *testing.T) {
	e := newAdminEnv(t)

	// Fields the client must not control (status, decided_by) are ignored.
	w := e.call(http.MethodPost, "/admin/vendors/"+pendingStore+"/reject", "admin",
		`{"reason_code":"unverifiable_address","message":"نشانی تحویل قابل تأیید نبود. لطفاً نشانی را اصلاح کنید.","status":"approved","decided_by":"someone else"}`)
	var v dto.VendorResponse
	if w.Code != http.StatusOK || json.Unmarshal(w.Body.Bytes(), &v) != nil {
		t.Fatalf("reject: %d %s", w.Code, w.Body.String())
	}
	if v.Status != "rejected" || v.Decision == nil || v.Decision.Type != "rejection" ||
		v.Decision.ReasonCode != "unverifiable_address" || v.Decision.DecidedBy == "someone else" || v.StatusUpdatedAt == nil {
		t.Fatalf("reject result: %+v", v)
	}

	// rejected is final: approving it is a conflict with a Persian message.
	w = e.call(http.MethodPost, "/admin/vendors/"+pendingStore+"/approve", "admin", "")
	if w.Code != http.StatusConflict || decodeError(t, w).Code != exception.CodeConflict ||
		!strings.Contains(decodeError(t, w).Message, "در انتظار بررسی") {
		t.Fatalf("approve rejected: %d %s", w.Code, w.Body.String())
	}

	w = e.call(http.MethodPost, "/admin/vendors/"+approvedStore+"/suspend", "admin", `{"reason_code":"other","message":"short"}`)
	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), `"message"`) {
		t.Fatalf("short message: %d %s", w.Code, w.Body.String())
	}
	if w := e.call(http.MethodPost, "/admin/vendors/"+approvedStore+"/suspend", "admin", `not json`); w.Code != http.StatusBadRequest {
		t.Fatalf("malformed body: %d", w.Code)
	}

	w = e.call(http.MethodPost, "/admin/vendors/"+approvedStore+"/suspend", "admin",
		`{"reason_code":"missed_pickups","message":"سه سفارش در هفته‌ی گذشته تحویل داده نشد."}`)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"type":"suspension"`) {
		t.Fatalf("suspend: %d %s", w.Code, w.Body.String())
	}
	w = e.call(http.MethodPost, "/admin/vendors/"+approvedStore+"/reactivate", "admin", "")
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"status":"approved"`) || !strings.Contains(w.Body.String(), `"decision":null`) {
		t.Fatalf("reactivate: %d %s", w.Code, w.Body.String())
	}
}

func TestCORSForAdminPanel(t *testing.T) {
	e := newAdminEnv(t)

	req := httptest.NewRequest(http.MethodOptions, "/admin/vendors/"+pendingStore+"/approve", nil)
	req.Header.Set("Origin", adminOrigin)
	req.Header.Set("Access-Control-Request-Method", "POST")
	req.Header.Set("Access-Control-Request-Headers", "authorization")
	w := httptest.NewRecorder()
	e.r.ServeHTTP(w, req)
	if w.Code != http.StatusNoContent || w.Header().Get("Access-Control-Allow-Origin") != adminOrigin ||
		!strings.Contains(w.Header().Get("Access-Control-Allow-Headers"), "Authorization") ||
		!strings.Contains(w.Header().Get("Access-Control-Allow-Methods"), "POST") {
		t.Fatalf("preflight: %d %v", w.Code, w.Header())
	}

	req = httptest.NewRequest(http.MethodGet, "/admin/vendors", nil)
	req.Header.Set("Origin", adminOrigin)
	req.Header.Set("Authorization", "Bearer "+e.tokens["admin"])
	w = httptest.NewRecorder()
	e.r.ServeHTTP(w, req)
	if w.Code != http.StatusOK || w.Header().Get("Access-Control-Allow-Origin") != adminOrigin {
		t.Fatalf("actual request: %d %v", w.Code, w.Header())
	}

	req = httptest.NewRequest(http.MethodOptions, "/admin/vendors", nil)
	req.Header.Set("Origin", "https://evil.example")
	req.Header.Set("Access-Control-Request-Method", "GET")
	w = httptest.NewRecorder()
	e.r.ServeHTTP(w, req)
	if w.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatalf("unknown origin must get no CORS headers: %v", w.Header())
	}
}

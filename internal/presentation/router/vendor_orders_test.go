package router_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/tahedig/too-good-to-go/backend/internal/application/dto"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/enum"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/model"
)

func TestVendorRoutesRBAC(t *testing.T) {
	e := newOrdersEnv(t)
	cust := e.login(t, rbacCustomerPhone)
	vendor := e.login(t, rbacVendorPhone)

	routes := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/vendor/store"},
		{http.MethodGet, "/api/v1/vendor/store"},
		{http.MethodGet, "/vendor/orders"},
		{http.MethodGet, "/api/v1/vendor/orders"},
		{http.MethodGet, "/vendor/orders/summary"},
		{http.MethodGet, "/api/v1/vendor/orders/summary"},
		{http.MethodGet, "/vendor/orders/test-id"},
		{http.MethodGet, "/api/v1/vendor/orders/test-id"},
		{http.MethodPost, "/vendor/orders/test-id/verify-pickup"},
		{http.MethodPost, "/api/v1/vendor/orders/test-id/verify-pickup"},
	}

	for _, rt := range routes {
		// 1. Unauthenticated -> 401
		req := httptest.NewRequest(rt.method, rt.path, nil)
		w := httptest.NewRecorder()
		e.r.ServeHTTP(w, req)
		if w.Code != http.StatusUnauthorized {
			t.Errorf("%s %s unauthenticated: got %d, want 401", rt.method, rt.path, w.Code)
		}

		// 2. Customer token -> 403
		req = httptest.NewRequest(rt.method, rt.path, nil)
		req.Header.Set("Authorization", "Bearer "+cust.AccessToken)
		w = httptest.NewRecorder()
		e.r.ServeHTTP(w, req)
		if w.Code != http.StatusForbidden {
			t.Errorf("%s %s customer role: got %d, want 403", rt.method, rt.path, w.Code)
		}

		// 3. Vendor token -> reaches handler (not 401 or 403)
		req = httptest.NewRequest(rt.method, rt.path, nil)
		req.Header.Set("Authorization", "Bearer "+vendor.AccessToken)
		w = httptest.NewRecorder()
		e.r.ServeHTTP(w, req)
		if w.Code == http.StatusUnauthorized || w.Code == http.StatusForbidden {
			t.Errorf("%s %s vendor role: got %d", rt.method, rt.path, w.Code)
		}
	}
}

func TestVendorStoreEndpoints(t *testing.T) {
	e := newOrdersEnv(t)
	vendor := e.login(t, rbacVendorPhone)

	e.orders.AddVendorStore(vendor.User.ID, model.VendorStoreActive{
		ID:       "store-uuid-1",
		Name:     "نانوایی شمیران",
		Address:  "تهران، تجریش",
		Phone:    "+982122000000",
		IsActive: true,
	})

	for _, path := range []string{"/vendor/store", "/api/v1/vendor/store"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("Authorization", "Bearer "+vendor.AccessToken)
		w := httptest.NewRecorder()
		e.r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("%s: got %d, body: %s", path, w.Code, w.Body.String())
		}

		var resp dto.VendorStoreActiveResponse
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("%s unmarshal: %v", path, err)
		}
		if resp.ID != "store-uuid-1" || resp.Name != "نانوایی شمیران" || !resp.IsActive {
			t.Fatalf("%s unexpected payload: %+v", path, resp)
		}
	}
}

func TestVendorOrdersFlow(t *testing.T) {
	e := newOrdersEnv(t)
	vendor := e.login(t, rbacVendorPhone)

	orderDate := "2026-10-19"
	e.orders.AddVendorOrder(vendor.User.ID, model.VendorOrder{
		ID:          "ord-12345",
		NumericID:   9821,
		OrderNumber: "TG-9821",
		Customer: model.VendorOrderCustomer{
			ID:    "cust_101",
			Name:  "لیلا علیزاده",
			Phone: "+989123456789",
		},
		Package: model.VendorOrderPackage{
			ID:       "pkg_501",
			Title:    "بستهٔ غافلگیری نانوایی",
			ImageURL: "https://example.com/bread.jpg",
		},
		Quantity:   1,
		UnitPrice:  40000,
		TotalPrice: 40000,
		Status:     enum.VendorOrderStatusActive,
		PickupDate: orderDate,
		PickupTimeSlot: model.VendorOrderTimeSlot{
			StartTime: "18:00",
			EndTime:   "19:00",
		},
		PickupCode: "1234",
		CreatedAt:  time.Date(2026, 10, 19, 14, 30, 0, 0, time.UTC),
	})

	e.orders.AddVendorOrder(vendor.User.ID, model.VendorOrder{
		ID:          "ord-12346",
		NumericID:   9822,
		OrderNumber: "TG-9822",
		Customer: model.VendorOrderCustomer{
			ID:    "cust_102",
			Name:  "علی رضایی",
			Phone: "+989123456780",
		},
		Package: model.VendorOrderPackage{
			ID:       "pkg_502",
			Title:    "بستهٔ شیرینی",
			ImageURL: "https://example.com/pastry.jpg",
		},
		Quantity:   2,
		UnitPrice:  40000,
		TotalPrice: 80000,
		Status:     enum.VendorOrderStatusDelivered,
		PickupDate: orderDate,
		PickupTimeSlot: model.VendorOrderTimeSlot{
			StartTime: "17:00",
			EndTime:   "18:00",
		},
		PickupCode: "5678",
		CreatedAt:  time.Date(2026, 10, 19, 15, 0, 0, 0, time.UTC),
	})

	// 1. List vendor orders
	for _, path := range []string{"/vendor/orders?page=1&limit=20", "/api/v1/vendor/orders?page=1&limit=20"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("Authorization", "Bearer "+vendor.AccessToken)
		w := httptest.NewRecorder()
		e.r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("%s got %d %s", path, w.Code, w.Body.String())
		}

		var listResp dto.VendorOrderListResponse
		if err := json.Unmarshal(w.Body.Bytes(), &listResp); err != nil {
			t.Fatalf("%s unmarshal: %v", path, err)
		}
		if len(listResp.Orders) != 2 || listResp.Pagination.TotalItems != 2 {
			t.Fatalf("%s expected 2 orders, got %+v", path, listResp)
		}
	}

	// 2. Summary stats
	for _, path := range []string{"/vendor/orders/summary?date=" + orderDate, "/api/v1/vendor/orders/summary?date=" + orderDate} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("Authorization", "Bearer "+vendor.AccessToken)
		w := httptest.NewRecorder()
		e.r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("%s got %d %s", path, w.Code, w.Body.String())
		}

		var summaryResp dto.VendorOrderSummaryResponse
		if err := json.Unmarshal(w.Body.Bytes(), &summaryResp); err != nil {
			t.Fatalf("%s unmarshal: %v", path, err)
		}
		if summaryResp.TotalReservedPackages != 3 || summaryResp.ActiveReservationsCount != 1 ||
			summaryResp.DeliveredPackagesCount != 1 || summaryResp.TotalRevenue != 120000 {
			t.Fatalf("%s unexpected summary: %+v", path, summaryResp)
		}
	}

	// 3. Get single order detail
	for _, path := range []string{"/vendor/orders/ord-12345", "/api/v1/vendor/orders/ord-12345"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("Authorization", "Bearer "+vendor.AccessToken)
		w := httptest.NewRecorder()
		e.r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("%s got %d %s", path, w.Code, w.Body.String())
		}

		var detailResp dto.VendorOrderItemResponse
		if err := json.Unmarshal(w.Body.Bytes(), &detailResp); err != nil {
			t.Fatalf("%s unmarshal: %v", path, err)
		}
		if detailResp.ID != "ord-12345" || detailResp.OrderNumber != "TG-9821" || detailResp.Customer.Name != "لیلا علیزاده" {
			t.Fatalf("%s unexpected detail: %+v", path, detailResp)
		}
	}

	// 4. Verify pickup with wrong code -> 400
	bodyWrong, _ := json.Marshal(dto.VerifyPickupRequest{PickupCode: "9999"})
	req := httptest.NewRequest(http.MethodPost, "/vendor/orders/ord-12345/verify-pickup", bytes.NewReader(bodyWrong))
	req.Header.Set("Authorization", "Bearer "+vendor.AccessToken)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	e.r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for wrong pickup code, got %d %s", w.Code, w.Body.String())
	}

	// 5. Verify pickup with correct code -> 200
	bodyCorrect, _ := json.Marshal(dto.VerifyPickupRequest{PickupCode: "1234"})
	req = httptest.NewRequest(http.MethodPost, "/vendor/orders/ord-12345/verify-pickup", bytes.NewReader(bodyCorrect))
	req.Header.Set("Authorization", "Bearer "+vendor.AccessToken)
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	e.r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for correct pickup code, got %d %s", w.Code, w.Body.String())
	}

	var verifyResp dto.VerifyPickupResponse
	if err := json.Unmarshal(w.Body.Bytes(), &verifyResp); err != nil {
		t.Fatalf("unmarshal verify response: %v", err)
	}
	if !verifyResp.Success || verifyResp.Status != "delivered" || verifyResp.OrderID != "ord-12345" {
		t.Fatalf("unexpected verify response: %+v", verifyResp)
	}

	// 6. Verify again -> 409 Conflict (already delivered)
	req = httptest.NewRequest(http.MethodPost, "/vendor/orders/ord-12345/verify-pickup", bytes.NewReader(bodyCorrect))
	req.Header.Set("Authorization", "Bearer "+vendor.AccessToken)
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	e.r.ServeHTTP(w, req)
	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409 for already delivered, got %d %s", w.Code, w.Body.String())
	}
}

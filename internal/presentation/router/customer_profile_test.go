package router_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/tahedig/too-good-to-go/backend/internal/application/dto"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/exception"
)

func getCustomerProfile(t *testing.T, e *rbacEnv, auth string) dto.ProfileResponse {
	t.Helper()
	w := e.get("/customer/profile", auth)
	var profile dto.ProfileResponse
	if w.Code != http.StatusOK || json.Unmarshal(w.Body.Bytes(), &profile) != nil {
		t.Fatalf("GET /customer/profile: %d %s", w.Code, w.Body.String())
	}
	return profile
}

func TestCustomerProfileIsolationBetweenCustomers(t *testing.T) {
	e := newRBACEnv(t)
	aPhone := "+989130000001"
	bPhone := "+989130000002"
	a := e.login(t, aPhone)
	b := e.login(t, bPhone)
	aAuth := "Bearer " + a.AccessToken
	bAuth := "Bearer " + b.AccessToken

	w := e.patch("/customer/profile", aAuth, `{"display_name":"Customer A"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("set customer A profile: %d %s", w.Code, w.Body.String())
	}
	w = e.patch("/customer/profile", bAuth, `{"display_name":"Customer B"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("set customer B profile: %d %s", w.Code, w.Body.String())
	}

	aProfile := getCustomerProfile(t, e, aAuth)
	bProfile := getCustomerProfile(t, e, bAuth)
	if aProfile.UserID != a.User.ID || aProfile.Phone != aPhone || aProfile.DisplayName != "Customer A" {
		t.Fatalf("customer A received an unexpected profile: %+v", aProfile)
	}
	if bProfile.UserID != b.User.ID || bProfile.Phone != bPhone || bProfile.DisplayName != "Customer B" {
		t.Fatalf("customer B received an unexpected profile: %+v", bProfile)
	}
	if aProfile.UserID == bProfile.UserID || aProfile.Phone == bProfile.Phone {
		t.Fatalf("customer sessions crossed identities: A=%+v B=%+v", aProfile, bProfile)
	}

	w = e.patch("/customer/profile", aAuth,
		`{"display_name":"Customer A Updated","user_id":"`+b.User.ID+`","id":"`+b.User.ID+`","phone":"`+bPhone+`"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("PATCH with another customer's identity fields: %d %s", w.Code, w.Body.String())
	}

	aProfile = getCustomerProfile(t, e, aAuth)
	bProfile = getCustomerProfile(t, e, bAuth)
	if aProfile.UserID != a.User.ID || aProfile.Phone != aPhone || aProfile.DisplayName != "Customer A Updated" {
		t.Fatalf("customer A update did not stay on A: %+v", aProfile)
	}
	if bProfile.UserID != b.User.ID || bProfile.Phone != bPhone || bProfile.DisplayName != "Customer B" {
		t.Fatalf("customer B was changed or replaced: %+v", bProfile)
	}
	if aProfile.UserID == bProfile.UserID {
		t.Fatalf("customer profiles returned the same user_id: A=%+v B=%+v", aProfile, bProfile)
	}
}

func TestCustomerProfilePatchRejectsInvalidBodiesAndPreservesProfile(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{name: "malformed JSON", body: `{"display_name":`},
		{name: "missing display_name", body: `{}`},
		{name: "number", body: `{"display_name":123}`},
		{name: "null", body: `{"display_name":null}`},
		{name: "object", body: `{"display_name":{"first":"Sam"}}`},
		{name: "array", body: `{"display_name":["Sam"]}`},
		{name: "empty string", body: `{"display_name":""}`},
		{name: "whitespace only", body: `{"display_name":" \t\n "}`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newRBACEnv(t)
			customer := e.login(t, rbacCustomerPhone)
			auth := "Bearer " + customer.AccessToken

			w := e.patch("/customer/profile", auth, `{"display_name":"Original Name"}`)
			if w.Code != http.StatusOK {
				t.Fatalf("set original profile: %d %s", w.Code, w.Body.String())
			}

			w = e.patch("/customer/profile", auth, tc.body)
			if w.Code != http.StatusBadRequest || decodeError(t, w).Code != exception.CodeBadRequest {
				t.Fatalf("invalid PATCH %q: got %d %s, want 400 %s", tc.name, w.Code, w.Body.String(), exception.CodeBadRequest)
			}

			profile := getCustomerProfile(t, e, auth)
			if profile.DisplayName != "Original Name" {
				t.Fatalf("invalid PATCH %q changed stored profile: got %+v, want unchanged display_name", tc.name, profile)
			}
		})
	}
}

func TestCustomerProfileDisplayNameRuneBoundaries(t *testing.T) {
	cases := []struct {
		name        string
		displayName string
		wantStatus  int
	}{
		{name: "exactly 1 character", displayName: "x", wantStatus: http.StatusOK},
		{name: "exactly 100 characters", displayName: strings.Repeat("x", 100), wantStatus: http.StatusOK},
		{name: "exactly 100 Persian runes", displayName: strings.Repeat("ن", 100), wantStatus: http.StatusOK},
		{name: "exactly 101 Persian runes", displayName: strings.Repeat("ن", 101), wantStatus: http.StatusBadRequest},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newRBACEnv(t)
			customer := e.login(t, rbacCustomerPhone)
			auth := "Bearer " + customer.AccessToken
			body, err := json.Marshal(map[string]string{"display_name": tc.displayName})
			if err != nil {
				t.Fatal(err)
			}

			w := e.patch("/customer/profile", auth, string(body))
			if w.Code != tc.wantStatus {
				t.Fatalf("PATCH display_name length: got %d %s, want %d", w.Code, w.Body.String(), tc.wantStatus)
			}

			if tc.wantStatus == http.StatusOK {
				profile := getCustomerProfile(t, e, auth)
				if profile.DisplayName != tc.displayName {
					t.Fatalf("successful boundary update stored %q, want %q", profile.DisplayName, tc.displayName)
				}
				return
			}

			if decodeError(t, w).Code != exception.CodeBadRequest {
				t.Fatalf("101-rune PATCH code: got %d %s, want %s", w.Code, w.Body.String(), exception.CodeBadRequest)
			}
			profile := getCustomerProfile(t, e, auth)
			if profile.DisplayName != "" {
				t.Fatalf("101-rune PATCH changed stored profile: got %+v, want original empty name", profile)
			}
		})
	}
}

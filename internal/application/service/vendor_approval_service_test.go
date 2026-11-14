package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	serviceinterface "github.com/tahedig/too-good-to-go/backend/internal/application/service/interface"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/enum"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/exception"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/model"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/repository"
	"github.com/tahedig/too-good-to-go/backend/internal/infrastructure/repository/memory"
)

const adminID = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"

// validMessage is a reason message within the 20-500 character limit.
const validMessage = "نشانی تحویل قابل تأیید نبود. لطفاً نشانی را اصلاح کنید."

func storeID(n int) string { return fmt.Sprintf("00000000-0000-4000-8000-%012d", n) }

func newVendorService(stores ...model.VendorStore) (serviceinterface.VendorApprovalService, *memory.VendorApprovalRepository) {
	repo := memory.NewVendorApprovalRepository()
	for _, s := range stores {
		repo.Add(s)
	}
	return NewVendorApprovalService(repo, discard), repo
}

func storeWith(n int, status enum.StoreStatus) model.VendorStore {
	return model.VendorStore{
		ID: storeID(n), StoreName: fmt.Sprintf("Store %d", n), OwnerName: "Owner", Category: "نانوایی",
		Address: "خیابان ولیعصر", Status: status, SubmittedAt: time.Date(2026, 10, 1, 0, 0, n, 0, time.UTC),
	}
}

func reasonFor(a enum.StoreAction) serviceinterface.VendorDecisionInput {
	if !a.NeedsReason() {
		return serviceinterface.VendorDecisionInput{}
	}
	return serviceinterface.VendorDecisionInput{ReasonCode: a.ReasonCodes()[0], Message: validMessage}
}

// Every action from every status: only the four transitions of the
// contract succeed, everything else is 409 and changes nothing.
func TestDecideTransitionMatrix(t *testing.T) {
	allowed := map[enum.StoreAction][2]enum.StoreStatus{
		enum.StoreActionApprove:    {enum.StoreStatusPending, enum.StoreStatusApproved},
		enum.StoreActionReject:     {enum.StoreStatusPending, enum.StoreStatusRejected},
		enum.StoreActionSuspend:    {enum.StoreStatusApproved, enum.StoreStatusSuspended},
		enum.StoreActionReactivate: {enum.StoreStatusSuspended, enum.StoreStatusApproved},
	}
	for action, want := range allowed {
		for _, from := range enum.StoreStatuses {
			svc, repo := newVendorService(storeWith(1, from))
			got, err := svc.Decide(context.Background(), adminID, storeID(1), action, reasonFor(action))
			if from == want[0] {
				if err != nil || got.Status != want[1] || got.StatusUpdatedAt == nil {
					t.Errorf("%s from %s: got %+v, %v", action, from, got, err)
				}
				if h := repo.History(); len(h) != 1 || h[0].AdminUserID != adminID || h[0].Action != action {
					t.Errorf("%s from %s: history %+v", action, from, h)
				}
				continue
			}
			if !isCode(err, exception.CodeConflict) {
				t.Errorf("%s from %s: got %v, want 409", action, from, err)
			}
			if s, _ := repo.Get(context.Background(), storeID(1)); s.Status != from || len(repo.History()) != 0 {
				t.Errorf("%s from %s: a refused decision changed the store", action, from)
			}
		}
	}
}

func TestDecideRecordsAndClearsDecision(t *testing.T) {
	svc, _ := newVendorService(storeWith(1, enum.StoreStatusApproved), storeWith(2, enum.StoreStatusPending))
	ctx := context.Background()

	s, err := svc.Decide(ctx, adminID, storeID(1), enum.StoreActionSuspend,
		serviceinterface.VendorDecisionInput{ReasonCode: "missed_pickups", Message: "  " + validMessage + "  "})
	if err != nil || s.Decision == nil || s.Decision.Action != enum.StoreActionSuspend ||
		s.Decision.ReasonCode != "missed_pickups" || s.Decision.Message != validMessage {
		t.Fatalf("suspend: %+v err=%v", s, err)
	}
	s, err = svc.Decide(ctx, adminID, storeID(1), enum.StoreActionReactivate, serviceinterface.VendorDecisionInput{})
	if err != nil || s.Status != enum.StoreStatusApproved || s.Decision != nil {
		t.Fatalf("reactivate must clear the decision: %+v err=%v", s, err)
	}
	// approve ignores any reason sent along
	s, err = svc.Decide(ctx, adminID, storeID(2), enum.StoreActionApprove,
		serviceinterface.VendorDecisionInput{ReasonCode: "junk", Message: "x"})
	if err != nil || s.Decision != nil {
		t.Fatalf("approve: %+v err=%v", s, err)
	}
}

func TestDecideValidatesReason(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		action enum.StoreAction
		in     serviceinterface.VendorDecisionInput
		fields []string
	}{
		{enum.StoreActionReject, serviceinterface.VendorDecisionInput{}, []string{"reason_code", "message"}},
		{enum.StoreActionReject, serviceinterface.VendorDecisionInput{ReasonCode: "missed_pickups", Message: validMessage}, []string{"reason_code"}},
		{enum.StoreActionSuspend, serviceinterface.VendorDecisionInput{ReasonCode: "unverifiable_address", Message: validMessage}, []string{"reason_code"}},
		{enum.StoreActionReject, serviceinterface.VendorDecisionInput{ReasonCode: "other", Message: "کوتاه"}, []string{"message"}},
		// 19 Persian letters plus padding: still too short after trimming
		{enum.StoreActionReject, serviceinterface.VendorDecisionInput{ReasonCode: "other", Message: "   " + strings.Repeat("ن", 19) + "   "}, []string{"message"}},
		{enum.StoreActionReject, serviceinterface.VendorDecisionInput{ReasonCode: "other", Message: strings.Repeat("ن", 501)}, []string{"message"}},
	}
	for i, tc := range cases {
		from, _, _ := tc.action.Transition()
		svc, repo := newVendorService(storeWith(1, from))
		_, err := svc.Decide(ctx, adminID, storeID(1), tc.action, tc.in)
		var appErr exception.AppError
		if !isCode(err, exception.CodeBadRequest) || !errors.As(err, &appErr) {
			t.Errorf("case %d: got %v, want 400", i, err)
			continue
		}
		got := fmt.Sprint(appErr.Details())
		for _, f := range tc.fields {
			if !strings.Contains(got, f) {
				t.Errorf("case %d: details %s miss %s", i, got, f)
			}
		}
		if len(repo.History()) != 0 {
			t.Errorf("case %d: an invalid decision was recorded", i)
		}
	}

	// Exactly 20 and 500 characters are accepted.
	for _, n := range []int{20, 500} {
		svc, _ := newVendorService(storeWith(1, enum.StoreStatusPending))
		if _, err := svc.Decide(ctx, adminID, storeID(1), enum.StoreActionReject,
			serviceinterface.VendorDecisionInput{ReasonCode: "other", Message: strings.Repeat("ن", n)}); err != nil {
			t.Errorf("%d characters: %v", n, err)
		}
	}
}

// Checks run in the contract's order: 404, then the body, then the transition.
func TestDecideCheckOrder(t *testing.T) {
	ctx := context.Background()
	svc, _ := newVendorService(storeWith(1, enum.StoreStatusRejected))

	if _, err := svc.Decide(ctx, adminID, storeID(9), enum.StoreActionReject, serviceinterface.VendorDecisionInput{}); !isCode(err, exception.CodeNotFound) {
		t.Errorf("missing store with a bad body: got %v, want 404", err)
	}
	if _, err := svc.Decide(ctx, adminID, storeID(1), enum.StoreActionReject, serviceinterface.VendorDecisionInput{}); !isCode(err, exception.CodeBadRequest) {
		t.Errorf("bad body on a store in the wrong status: got %v, want 400", err)
	}
	if _, err := svc.Decide(ctx, adminID, storeID(1), enum.StoreActionReject, reasonFor(enum.StoreActionReject)); !isCode(err, exception.CodeConflict) {
		t.Errorf("valid body on a rejected store: got %v, want 409", err)
	}
	if _, err := svc.Decide(ctx, "", storeID(1), enum.StoreActionApprove, serviceinterface.VendorDecisionInput{}); !isCode(err, exception.CodeUnauthorized) {
		t.Errorf("no admin: got %v, want 401", err)
	}
}

// racingRepo reports that another admin changed the store between the
// service's read and its conditional update.
type racingRepo struct {
	*memory.VendorApprovalRepository
}

func (racingRepo) Transition(context.Context, model.StoreTransition) (*model.VendorStore, error) {
	return nil, repository.ErrStoreStatusChanged
}

func TestDecideConcurrentChangeIs409(t *testing.T) {
	repo := memory.NewVendorApprovalRepository()
	repo.Add(storeWith(1, enum.StoreStatusPending))
	svc := NewVendorApprovalService(racingRepo{repo}, discard)
	_, err := svc.Decide(context.Background(), adminID, storeID(1), enum.StoreActionApprove, serviceinterface.VendorDecisionInput{})
	if !isCode(err, exception.CodeConflict) {
		t.Fatalf("got %v, want 409", err)
	}
}

func TestListVendors(t *testing.T) {
	stores := []model.VendorStore{
		storeWith(1, enum.StoreStatusPending), storeWith(2, enum.StoreStatusApproved),
		storeWith(3, enum.StoreStatusApproved), storeWith(4, enum.StoreStatusRejected),
		storeWith(5, enum.StoreStatusSuspended),
	}
	stores[2].StoreName = "نانوایی سنتی"
	svc, _ := newVendorService(stores...)
	ctx := context.Background()

	res, err := svc.ListVendors(ctx, serviceinterface.VendorListInput{})
	if err != nil || res.Total != 5 || len(res.Items) != 5 || res.Page != 1 || res.PageSize != 20 ||
		res.Items[0].ID != storeID(5) {
		t.Fatalf("default list: %+v err=%v", res, err)
	}
	wantCounts := map[enum.StoreStatus]int{"pending": 1, "approved": 2, "rejected": 1, "suspended": 1}

	res, _ = svc.ListVendors(ctx, serviceinterface.VendorListInput{Status: "approved,suspended", PageSize: "2", Page: "2"})
	if res.Total != 3 || len(res.Items) != 1 || res.Items[0].ID != storeID(2) || fmt.Sprint(res.Counts) != fmt.Sprint(wantCounts) {
		t.Fatalf("filtered page 2: %+v", res)
	}
	res, _ = svc.ListVendors(ctx, serviceinterface.VendorListInput{Search: "  سنتی "})
	if res.Total != 1 || res.Items[0].ID != storeID(3) || fmt.Sprint(res.Counts) != fmt.Sprint(wantCounts) {
		t.Fatalf("search: %+v", res)
	}
	res, _ = svc.ListVendors(ctx, serviceinterface.VendorListInput{Page: "9"})
	if res.Total != 5 || len(res.Items) != 0 || res.Items == nil {
		t.Fatalf("page past the end must be an empty list: %+v", res)
	}

	for _, in := range []serviceinterface.VendorListInput{
		{Status: "active"}, {Page: "0"}, {Page: "x"}, {Page: "100001"}, {PageSize: "101"}, {PageSize: "0"},
		{Search: strings.Repeat("ن", 101)},
	} {
		if _, err := svc.ListVendors(ctx, in); !isCode(err, exception.CodeBadRequest) {
			t.Errorf("%+v: got %v, want 400", in, err)
		}
	}
}

func TestGetVendorNotFound(t *testing.T) {
	svc, _ := newVendorService()
	if _, err := svc.GetVendor(context.Background(), storeID(1)); !isCode(err, exception.CodeNotFound) {
		t.Fatalf("got %v, want 404", err)
	}
}

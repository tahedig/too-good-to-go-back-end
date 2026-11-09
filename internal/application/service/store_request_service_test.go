package service

import (
	"context"
	"errors"
	"testing"

	"github.com/tahedig/too-good-to-go/backend/internal/domain/model"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/repository"
)

type storeRequestRepositoryStub struct {
	requests  []*model.StoreRequest
	request   *model.StoreRequest
	err       error
	createErr error
	created   *model.StoreRequest
	userID    int64
	publicID  string
}

var _ repository.StoreRequestRepository = (*storeRequestRepositoryStub)(nil)

func (r *storeRequestRepositoryStub) ListByUser(
	_ context.Context,
	userID int64,
) ([]*model.StoreRequest, error) {
	r.userID = userID
	return r.requests, r.err
}

func (r *storeRequestRepositoryStub) GetByPublicIDAndUser(
	_ context.Context,
	publicID string,
	userID int64,
) (*model.StoreRequest, error) {
	r.publicID = publicID
	r.userID = userID
	return r.request, r.err
}

func (r *storeRequestRepositoryStub) Create(
	_ context.Context,
	req *model.StoreRequest,
) error {
	r.created = req
	return r.createErr
}

func TestStoreRequestServiceList(t *testing.T) {
	expected := []*model.StoreRequest{
		{PublicID: "req-1", StoreName: "نان صبحگاهی - شعبه شریعتی"},
	}

	repo := &storeRequestRepositoryStub{requests: expected}
	svc := NewStoreRequestService(repo)

	requests, err := svc.ListStoreRequests(context.Background(), 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(requests) != 1 || requests[0].PublicID != "req-1" {
		t.Fatalf("unexpected requests: %+v", requests)
	}

	if repo.userID != 42 {
		t.Fatalf("expected user ID 42, got %d", repo.userID)
	}
}

func TestStoreRequestServiceGet(t *testing.T) {
	expected := &model.StoreRequest{
		PublicID:  "req-1",
		StoreName: "نان صبحگاهی - شعبه شریعتی",
	}

	repo := &storeRequestRepositoryStub{request: expected}
	svc := NewStoreRequestService(repo)

	req, err := svc.GetStoreRequest(context.Background(), 42, "req-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if req != expected {
		t.Fatal("expected matching store request")
	}
}

func TestStoreRequestServiceGetNotFound(t *testing.T) {
	repo := &storeRequestRepositoryStub{}
	svc := NewStoreRequestService(repo)

	req, err := svc.GetStoreRequest(context.Background(), 42, "missing-req")
	if err == nil {
		t.Fatal("expected not found error")
	}
	if req != nil {
		t.Fatalf("expected nil request, got %+v", req)
	}
}

func TestStoreRequestServiceCreate(t *testing.T) {
	repo := &storeRequestRepositoryStub{}
	svc := NewStoreRequestService(repo)

	catID := int16(1)
	input := &model.StoreRequest{
		StoreName:  "  نان صبحگاهی - شعبه شریعتی  ",
		CategoryID: &catID,
		OwnerName:  "  آیدا کریمی  ",
		Address:    "  تهران، خیابان شریعتی  ",
	}

	created, err := svc.CreateStoreRequest(context.Background(), 42, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if created.StoreName != "نان صبحگاهی - شعبه شریعتی" ||
		created.Address != "تهران، خیابان شریعتی" ||
		created.OwnerName != "آیدا کریمی" {
		t.Fatalf("fields not trimmed/set properly: %+v", created)
	}

	if created.Status != "pending_review" || created.StatusFa != "در انتظار بررسی" {
		t.Fatalf("expected pending_review status, got: %s / %s", created.Status, created.StatusFa)
	}

	if created.VendorID != 42 {
		t.Fatalf("expected vendor ID 42, got %d", created.VendorID)
	}

	if created.PublicID == "" {
		t.Fatal("expected public ID to be generated")
	}

	if repo.created != input {
		t.Fatal("expected repository Create to be called")
	}
}

func TestStoreRequestServiceCreateValidation(t *testing.T) {
	tests := []struct {
		name  string
		input *model.StoreRequest
	}{
		{
			name:  "nil input",
			input: nil,
		},
		{
			name: "missing store name",
			input: &model.StoreRequest{
				Address: "Some address",
			},
		},
		{
			name: "missing address",
			input: &model.StoreRequest{
				StoreName: "Some store",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &storeRequestRepositoryStub{}
			svc := NewStoreRequestService(repo)

			_, err := svc.CreateStoreRequest(context.Background(), 42, tt.input)
			if err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestStoreRequestServiceRepositoryErrors(t *testing.T) {
	dbErr := errors.New("db error")

	t.Run("list error", func(t *testing.T) {
		repo := &storeRequestRepositoryStub{err: dbErr}
		svc := NewStoreRequestService(repo)

		_, err := svc.ListStoreRequests(context.Background(), 42)
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("create error", func(t *testing.T) {
		repo := &storeRequestRepositoryStub{createErr: dbErr}
		svc := NewStoreRequestService(repo)

		_, err := svc.CreateStoreRequest(context.Background(), 42, &model.StoreRequest{
			StoreName: "Store",
			Address:   "Address",
		})
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

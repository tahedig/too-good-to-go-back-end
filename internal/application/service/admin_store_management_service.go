package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"math"
	"strings"
	"time"

	serviceinterface "github.com/tahedig/too-good-to-go/backend/internal/application/service/interface"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/enum"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/exception"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/model"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/repository"
	"github.com/tahedig/too-good-to-go/backend/pkg/security"
)

const (
	adminStoreDefaultLimit = 20
	adminStoreMaxLimit     = 50
)

type adminStoreManagementService struct {
	repo repository.AdminStoreManagementRepository
}

func NewAdminStoreManagementService(repo repository.AdminStoreManagementRepository) serviceinterface.AdminStoreManagementService {
	return &adminStoreManagementService{repo: repo}
}

func (s *adminStoreManagementService) ListStores(
	ctx context.Context,
	in serviceinterface.AdminStoreListInput,
) (*serviceinterface.AdminStoreListPage, error) {
	var fields []exception.FieldError

	limit := adminStoreDefaultLimit
	if in.Limit != nil {
		limit = *in.Limit
	}
	if limit < 1 || limit > adminStoreMaxLimit {
		fields = append(fields, exception.FieldError{Field: "limit", Message: "must be between 1 and 50"})
	}

	sort := in.Sort
	if sort == "" {
		sort = "newest"
	}
	if sort != "newest" && sort != "oldest" {
		fields = append(fields, exception.FieldError{Field: "sort", Message: "must be newest or oldest"})
	}

	if in.Status != "" && !enum.StoreStatus(in.Status).Valid() {
		fields = append(fields, exception.FieldError{Field: "status", Message: "must be a valid store status"})
	}
	if in.CategoryID != nil && (*in.CategoryID < 1 || *in.CategoryID > math.MaxInt16) {
		fields = append(fields, exception.FieldError{Field: "category_id", Message: "must be a positive category ID"})
	}

	var cursor *repository.AdminStoreCursor
	if in.Cursor != "" {
		decoded, ok := decodeAdminStoreCursor(in.Cursor)
		if !ok || decoded.Sort != sort {
			fields = append(fields, exception.FieldError{Field: "cursor", Message: "must be a valid cursor for this sort"})
		} else {
			cursor = &repository.AdminStoreCursor{CreatedAt: decoded.CreatedAt, ID: decoded.ID}
		}
	}
	if len(fields) > 0 {
		return nil, exception.NewBadRequest("invalid store query", fields)
	}

	var categoryID *int16
	if in.CategoryID != nil {
		value := int16(*in.CategoryID)
		categoryID = &value
	}
	filter := repository.AdminStoreFilter{
		Search:         strings.TrimSpace(in.Search),
		Status:         in.Status,
		CategoryID:     categoryID,
		City:           strings.TrimSpace(in.City),
		AcceptsOrders:  in.AcceptsOrders,
		IncludeDeleted: in.IncludeDeleted != nil && *in.IncludeDeleted,
		Sort:           sort,
		Cursor:         cursor,
		Limit:          limit + 1,
	}
	page, err := s.repo.ListAdmin(ctx, filter)
	if err != nil {
		return nil, exception.NewInternalServer(err)
	}

	result := &serviceinterface.AdminStoreListPage{
		Items: page.Items,
		Total: page.Total,
	}
	if len(result.Items) > limit {
		result.HasMore = true
		result.Items = result.Items[:limit]
		last := result.Items[len(result.Items)-1]
		result.NextCursor = encodeAdminStoreCursor(last, sort)
	}
	return result, nil
}

func (s *adminStoreManagementService) GetManagedStore(ctx context.Context, storeID string) (*model.Store, error) {
	if !security.IsID(storeID) {
		return nil, badAdminStoreID()
	}
	store, err := s.repo.GetByPublicID(ctx, storeID)
	if errors.Is(err, repository.ErrStoreNotFound) || (err == nil && store == nil) {
		return nil, exception.NewNotFound(msgAdminStoreNotFound)
	}
	if err != nil {
		return nil, exception.NewInternalServer(err)
	}
	return store, nil
}

func (s *adminStoreManagementService) UpdateManagedStore(
	ctx context.Context,
	storeID string,
	updated *model.AdminStoreUpdate,
) (*model.Store, error) {
	if updated == nil {
		return nil, exception.NewBadRequest("store update data is required", nil)
	}
	if !security.IsID(storeID) {
		return nil, badAdminStoreID()
	}

	store, err := s.repo.GetByPublicID(ctx, storeID)
	if errors.Is(err, repository.ErrStoreNotFound) || (err == nil && store == nil) {
		return nil, exception.NewNotFound(msgAdminStoreNotFound)
	}
	if err != nil {
		return nil, exception.NewInternalServer(err)
	}

	if updated.Name != nil {
		store.Name = strings.TrimSpace(*updated.Name)
	}
	if updated.Biography != nil {
		store.Biography = strings.TrimSpace(*updated.Biography)
	}
	if updated.PickupInstructions != nil {
		store.PickupInstructions = strings.TrimSpace(*updated.PickupInstructions)
	}
	if updated.Phone != nil {
		phone := strings.TrimSpace(*updated.Phone)
		store.Phone = &phone
	}
	if updated.City != nil {
		store.City = strings.TrimSpace(*updated.City)
	}
	if updated.District != nil {
		district := strings.TrimSpace(*updated.District)
		store.District = &district
	}
	if updated.AddressLine != nil {
		store.AddressLine = strings.TrimSpace(*updated.AddressLine)
	}
	if updated.CategoryID != nil {
		active, err := s.repo.CategoryIsActive(ctx, *updated.CategoryID)
		if err != nil {
			return nil, exception.NewInternalServer(err)
		}
		if !active {
			return nil, exception.NewBadRequest("invalid store information", []exception.FieldError{{
				Field: "category_id", Message: "must reference an active category",
			}})
		}
		store.CategoryID = updated.CategoryID
	}
	if updated.Location != nil {
		store.Location = *updated.Location
	}
	if updated.LogoURL != nil {
		store.LogoURL = updated.LogoURL
	}
	if updated.CoverImageURL != nil {
		store.CoverImageURL = updated.CoverImageURL
	}
	if updated.WorkingHours != nil {
		store.WorkingHours = *updated.WorkingHours
	}
	if updated.Images != nil {
		store.Images = *updated.Images
	}
	if updated.AcceptsOrders != nil {
		store.AcceptsOrders = *updated.AcceptsOrders
	}

	if err := validateAdminStore(store, updated.Location != nil); err != nil {
		return nil, err
	}
	if err := s.repo.Update(ctx, store); err != nil {
		if errors.Is(err, repository.ErrStoreNotFound) {
			return nil, exception.NewNotFound(msgAdminStoreNotFound)
		}
		return nil, exception.NewInternalServer(err)
	}
	store, err = s.repo.GetByPublicID(ctx, storeID)
	if errors.Is(err, repository.ErrStoreNotFound) || (err == nil && store == nil) {
		return nil, exception.NewNotFound(msgAdminStoreNotFound)
	}
	if err != nil {
		return nil, exception.NewInternalServer(err)
	}
	return store, nil
}

func validateAdminStore(store *model.Store, locationChanged bool) error {
	var fields []exception.FieldError
	for name, value := range map[string]string{
		"name":         store.Name,
		"biography":    store.Biography,
		"city":         store.City,
		"address_line": store.AddressLine,
	} {
		if strings.TrimSpace(value) == "" {
			fields = append(fields, exception.FieldError{Field: name, Message: "must not be empty"})
		}
	}
	if locationChanged && (math.IsNaN(store.Location.Latitude) || math.IsInf(store.Location.Latitude, 0) ||
		math.IsNaN(store.Location.Longitude) || math.IsInf(store.Location.Longitude, 0) ||
		store.Location.Latitude < -90 || store.Location.Latitude > 90 ||
		store.Location.Longitude < -180 || store.Location.Longitude > 180) {
		fields = append(fields, exception.FieldError{Field: "location", Message: "latitude or longitude is out of range"})
	}
	seenDays := make(map[int]struct{}, len(store.WorkingHours))
	for i := range store.WorkingHours {
		hour := &store.WorkingHours[i]
		if hour.DayOfWeek < 0 || hour.DayOfWeek > 6 {
			fields = append(fields, exception.FieldError{Field: "working_hours", Message: "day_of_week must be between 0 and 6"})
			break
		}
		if _, exists := seenDays[hour.DayOfWeek]; exists {
			fields = append(fields, exception.FieldError{Field: "working_hours", Message: "each day_of_week may appear only once"})
			break
		}
		seenDays[hour.DayOfWeek] = struct{}{}
		if hour.IsClosed {
			hour.OpenTime = ""
			hour.CloseTime = ""
			continue
		}
		if _, err := time.Parse("15:04", hour.OpenTime); err != nil {
			fields = append(fields, exception.FieldError{Field: "working_hours", Message: "open_time must use HH:MM"})
			break
		}
		if _, err := time.Parse("15:04", hour.CloseTime); err != nil {
			fields = append(fields, exception.FieldError{Field: "working_hours", Message: "close_time must use HH:MM"})
			break
		}
	}
	for i := range store.Images {
		image := &store.Images[i]
		image.URL = strings.TrimSpace(image.URL)
		if strings.TrimSpace(image.URL) == "" || image.SortOrder < math.MinInt16 || image.SortOrder > math.MaxInt16 {
			fields = append(fields, exception.FieldError{Field: "images", Message: "image URL and sort order must be valid"})
			break
		}
	}
	if len(fields) > 0 {
		return exception.NewBadRequest("invalid store information", fields)
	}
	return nil
}

type adminStoreCursorPayload struct {
	Sort      string    `json:"s"`
	CreatedAt time.Time `json:"t"`
	ID        int64     `json:"i"`
}

func encodeAdminStoreCursor(store *model.Store, sort string) *string {
	payload, _ := json.Marshal(adminStoreCursorPayload{Sort: sort, CreatedAt: store.CreatedAt.UTC(), ID: store.ID})
	cursor := base64.RawURLEncoding.EncodeToString(payload)
	return &cursor
}

func decodeAdminStoreCursor(cursor string) (adminStoreCursorPayload, bool) {
	raw, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return adminStoreCursorPayload{}, false
	}
	var decoded adminStoreCursorPayload
	if err := json.Unmarshal(raw, &decoded); err != nil || decoded.ID <= 0 || decoded.CreatedAt.IsZero() ||
		(decoded.Sort != "newest" && decoded.Sort != "oldest") {
		return adminStoreCursorPayload{}, false
	}
	return decoded, true
}

func badAdminStoreID() error {
	return exception.NewBadRequest("invalid store ID", []exception.FieldError{{
		Field: "id", Message: "must be a valid UUID",
	}})
}

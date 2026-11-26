package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/tahedig/too-good-to-go/backend/internal/application/dto"
	serviceinterface "github.com/tahedig/too-good-to-go/backend/internal/application/service/interface"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/enum"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/exception"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/model"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/repository"
	"github.com/tahedig/too-good-to-go/backend/pkg/security"
)

const (
	maxReservationQuantity        = 50
	maxReservationIdempotencyLen  = 128
	reservationCancellationCutoff = 2 * time.Hour
)

type reservationService struct {
	repo repository.ReservationRepository
	now  func() time.Time
}

// NewReservationService creates a ReservationService for TEAM-49 and TEAM-51.
func NewReservationService(repo repository.ReservationRepository) serviceinterface.ReservationService {
	return &reservationService{repo: repo, now: time.Now}
}

func (s *reservationService) CreateReservation(
	ctx context.Context,
	userPublicID string,
	in serviceinterface.CreateReservationInput,
) (*serviceinterface.CreateReservationOutput, error) {
	if strings.TrimSpace(userPublicID) == "" {
		return nil, exception.NewUnauthorized("authentication required")
	}

	offerID := strings.ToLower(strings.TrimSpace(in.OfferID))
	idemKey := strings.TrimSpace(in.IdempotencyKey)

	var fields []exception.FieldError
	if !security.IsID(offerID) {
		fields = append(fields, exception.FieldError{Field: "offer_id", Message: "must be a valid UUID"})
	}
	if in.Quantity < 1 || in.Quantity > maxReservationQuantity {
		fields = append(fields, exception.FieldError{
			Field:   "quantity",
			Message: fmt.Sprintf("must be between 1 and %d", maxReservationQuantity),
		})
	}
	if in.TermsAccepted != nil && !*in.TermsAccepted {
		fields = append(fields, exception.FieldError{
			Field:   "terms_accepted",
			Message: "pickup terms must be accepted",
		})
	}
	if len(idemKey) > maxReservationIdempotencyLen {
		fields = append(fields, exception.FieldError{
			Field:   "idempotency_key",
			Message: fmt.Sprintf("must be at most %d characters", maxReservationIdempotencyLen),
		})
	}
	if len(fields) > 0 {
		return nil, exception.NewBadRequest("invalid reservation checkout request", fields)
	}

	payloadHash := ""
	if idemKey != "" {
		payloadHash = hashReservationPayload(offerID, in.Quantity)
	}

	res, err := s.repo.Create(ctx, repository.CreateReservationParams{
		UserPublicID:           strings.ToLower(strings.TrimSpace(userPublicID)),
		OfferPublicID:          offerID,
		Quantity:               in.Quantity,
		IdempotencyKey:         idemKey,
		IdempotencyPayloadHash: payloadHash,
	})
	if err != nil {
		return nil, mapReservationError(err)
	}

	return &serviceinterface.CreateReservationOutput{
		Reservation: toReservationDTO(res.Reservation, s.now()),
		Replayed:    res.Replayed,
	}, nil
}

func (s *reservationService) GetReservation(
	ctx context.Context,
	userPublicID, reservationPublicID string,
) (*dto.ReservationResponse, error) {
	if strings.TrimSpace(userPublicID) == "" {
		return nil, exception.NewUnauthorized("authentication required")
	}
	id := strings.ToLower(strings.TrimSpace(reservationPublicID))
	if !security.IsID(id) {
		return nil, exception.NewBadRequest("invalid reservation id", []exception.FieldError{
			{Field: "id", Message: "must be a valid UUID"},
		})
	}

	res, err := s.repo.GetByPublicIDAndUser(ctx, id, strings.ToLower(strings.TrimSpace(userPublicID)))
	if err != nil {
		return nil, mapReservationError(err)
	}
	return toReservationDTO(res, s.now()), nil
}

func mapReservationError(err error) error {
	switch {
	case errors.Is(err, repository.ErrUserNotFound):
		return exception.NewUnauthorized("authenticated customer not found")
	case errors.Is(err, repository.ErrOfferNotFound):
		return exception.NewNotFound("offer not found")
	case errors.Is(err, repository.ErrOfferUnavailable):
		return exception.NewConflict("offer is not available for reservation")
	case errors.Is(err, repository.ErrInsufficientStock):
		return exception.NewConflict("requested quantity is not available")
	case errors.Is(err, repository.ErrExceedsMaxPerUser):
		return exception.NewBadRequest("requested quantity exceeds maximum allowed per customer", []exception.FieldError{
			{Field: "quantity", Message: "exceeds maximum allowed per customer"},
		})
	case errors.Is(err, repository.ErrIdempotencyConflict):
		return exception.NewConflict("idempotency key was already used with different reservation parameters")
	case errors.Is(err, repository.ErrReservationNotFound):
		return exception.NewNotFound("reservation not found")
	default:
		return exception.NewInternalServer(err)
	}
}

func hashReservationPayload(offerID string, quantity int) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s:%d", offerID, quantity)))
	return hex.EncodeToString(sum[:])
}

func toReservationDTO(r *model.Reservation, now time.Time) *dto.ReservationResponse {
	ref := r.Reference
	if ref == "" {
		ref = model.ReservationReference(r.ID)
	}
	currency := r.Currency
	if currency == "" {
		currency = "تومان"
	}

	storeDTO := dto.ReservationStoreResponse{
		ID:                 r.Store.ID,
		Name:               r.Store.Name,
		PickupAddress:      joinReservationAddress(r.Store.City, r.Store.AddressLine),
		PickupInstructions: r.Store.PickupInstructions,
	}
	if r.Store.Latitude != nil && r.Store.Longitude != nil {
		storeDTO.Location = &dto.StoreLocationResponse{
			Latitude:  *r.Store.Latitude,
			Longitude: *r.Store.Longitude,
		}
	}

	resp := &dto.ReservationResponse{
		ID:          r.ID,
		Reference:   ref,
		PickupCode:  r.PickupCode,
		Status:      string(r.Status),
		StatusLabel: reservationStatusLabel(r.Status),
		Group:       string(r.Status.Group()),
		Store:       storeDTO,
		Offer: dto.ReservationOfferResponse{
			ID:                r.Offer.ID,
			Title:             r.Offer.Title,
			OfferType:         r.Offer.OfferType,
			ImageURL:          r.Offer.ImageURL,
			CategoryName:      r.Offer.CategoryName,
			Quantity:          r.Offer.Quantity,
			UnitPrice:         r.Offer.UnitPrice,
			UnitOriginalPrice: r.Offer.UnitOriginalPrice,
		},
		ItemsTotal:  r.ItemsTotal,
		ServiceFee:  r.ServiceFee,
		TotalAmount: r.TotalAmount,
		Currency:    currency,
		PickupStart: r.PickupStart,
		PickupEnd:   r.PickupEnd,
		CreatedAt:   r.CreatedAt,
	}
	if r.PickupStart != nil && r.PickupEnd != nil {
		resp.PickupWindow = pickupWindowText(*r.PickupStart, *r.PickupEnd, now)
	}
	if r.PickupStart != nil {
		dl := r.PickupStart.UTC().Add(-reservationCancellationCutoff)
		resp.CancellationDeadline = &dl
	}
	return resp
}

func joinReservationAddress(city, line string) string {
	city = strings.TrimSpace(city)
	line = strings.TrimSpace(line)
	switch {
	case city == "":
		return line
	case line == "":
		return city
	default:
		return city + "، " + line
	}
}

func reservationStatusLabel(s enum.OrderStatus) string {
	switch s {
	case enum.OrderStatusPendingPayment:
		return "در انتظار پرداخت"
	case enum.OrderStatusPaid:
		return "رزرو تأیید شده"
	case enum.OrderStatusReady:
		return "آماده تحویل"
	case enum.OrderStatusCompleted:
		return "تحویل داده شده"
	case enum.OrderStatusCancelled:
		return "لغو شده"
	case enum.OrderStatusExpired:
		return "منقضی شده"
	case enum.OrderStatusNoShow:
		return "عدم مراجعه"
	default:
		return string(s)
	}
}

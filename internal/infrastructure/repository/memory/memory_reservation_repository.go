package memory

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/tahedig/too-good-to-go/backend/internal/domain/enum"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/model"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/repository"
	"github.com/tahedig/too-good-to-go/backend/pkg/security"
)

// ReservableOffer is an in-memory offer fixture for development without a
// database and for unit/router concurrency tests.
type ReservableOffer struct {
	PublicID          string
	Title             string
	OfferType         string
	ImageURL          *string
	CategoryName      string
	OriginalPrice     int64
	DiscountPrice     int64
	ServiceFee        int64
	QuantityAvailable int
	MaxPerUser        int
	PickupStart       time.Time
	PickupEnd         time.Time
	Active            bool
	Store             model.ReservationStore
}

type idempotencyEntry struct {
	payloadHash   string
	reservationID string
}

type storedReservation struct {
	userID      string
	reservation model.Reservation
}

// ReservationRepository is a thread-safe in-memory implementation of
// repository.ReservationRepository. When linked with an OrderRepository,
// created reservations are also recorded there so GET /orders reflects them.
type ReservationRepository struct {
	mu           sync.Mutex
	orders       *OrderRepository
	offers       map[string]*ReservableOffer
	reservations map[string]storedReservation
	idempotency  map[string]idempotencyEntry // key: userID + "|" + idempotencyKey
	nextCode     int
	now          func() time.Time
}

func NewReservationRepository(orders *OrderRepository) *ReservationRepository {
	return &ReservationRepository{
		orders:       orders,
		offers:       make(map[string]*ReservableOffer),
		reservations: make(map[string]storedReservation),
		idempotency:  make(map[string]idempotencyEntry),
		now:          time.Now,
	}
}

var _ repository.ReservationRepository = (*ReservationRepository)(nil)

// AddOffer registers or replaces an offer available for reservation.
func (r *ReservationRepository) AddOffer(offer ReservableOffer) {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := offer
	if cp.OfferType == "" {
		cp.OfferType = "surprise_bag"
	}
	if cp.MaxPerUser <= 0 {
		cp.MaxPerUser = 2
	}
	r.offers[cp.PublicID] = &cp
}

// OfferStock returns the remaining available quantity for an offer.
func (r *ReservationRepository) OfferStock(offerPublicID string) (int, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	o, ok := r.offers[offerPublicID]
	if !ok {
		return 0, false
	}
	return o.QuantityAvailable, true
}

func (r *ReservationRepository) Create(
	_ context.Context,
	params repository.CreateReservationParams,
) (*repository.CreateReservationResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if params.IdempotencyKey != "" {
		idemKey := params.UserPublicID + "|" + params.IdempotencyKey
		if entry, ok := r.idempotency[idemKey]; ok {
			if entry.payloadHash != "" && entry.payloadHash != params.IdempotencyPayloadHash {
				return nil, repository.ErrIdempotencyConflict
			}
			stored := r.reservations[entry.reservationID]
			cp := cloneReservation(stored.reservation)
			return &repository.CreateReservationResult{Reservation: &cp, Replayed: true}, nil
		}
	}

	offer, ok := r.offers[params.OfferPublicID]
	if !ok {
		return nil, repository.ErrOfferNotFound
	}
	now := r.now().UTC()
	if !offer.Active || !offer.PickupEnd.After(now) {
		return nil, repository.ErrOfferUnavailable
	}
	if offer.MaxPerUser > 0 && params.Quantity > offer.MaxPerUser {
		return nil, repository.ErrExceedsMaxPerUser
	}
	if offer.QuantityAvailable < params.Quantity {
		return nil, repository.ErrInsufficientStock
	}

	// Atomic decrement under mutex.
	offer.QuantityAvailable -= params.Quantity
	if offer.QuantityAvailable == 0 {
		offer.Active = false
	}

	id, err := security.NewID()
	if err != nil {
		return nil, err
	}
	r.nextCode++
	pickupCode := fmt.Sprintf("%04d", r.nextCode%10000)

	itemsTotal := offer.DiscountPrice * int64(params.Quantity)
	totalAmount := itemsTotal + offer.ServiceFee
	ps, pe := offer.PickupStart.UTC(), offer.PickupEnd.UTC()
	res := model.Reservation{
		ID:         id,
		Reference:  model.ReservationReference(id),
		PickupCode: pickupCode,
		Status:     enum.OrderStatusPaid,
		Store:      cloneReservationStore(offer.Store),
		Offer: model.ReservationOffer{
			ID:                offer.PublicID,
			Title:             offer.Title,
			OfferType:         offer.OfferType,
			ImageURL:          offer.ImageURL,
			CategoryName:      offer.CategoryName,
			Quantity:          params.Quantity,
			UnitPrice:         offer.DiscountPrice,
			UnitOriginalPrice: offer.OriginalPrice,
		},
		ItemsTotal:  itemsTotal,
		ServiceFee:  offer.ServiceFee,
		TotalAmount: totalAmount,
		Currency:    "تومان",
		PickupStart: &ps,
		PickupEnd:   &pe,
		CreatedAt:   now,
	}

	r.reservations[id] = storedReservation{userID: params.UserPublicID, reservation: res}
	if params.IdempotencyKey != "" {
		r.idempotency[params.UserPublicID+"|"+params.IdempotencyKey] = idempotencyEntry{
			payloadHash:   params.IdempotencyPayloadHash,
			reservationID: id,
		}
	}
	if r.orders != nil {
		r.orders.Add(params.UserPublicID, model.Order{
			ID:     res.ID,
			Status: res.Status,
			Store: model.OrderStore{
				ID:          res.Store.ID,
				Name:        res.Store.Name,
				City:        res.Store.City,
				AddressLine: res.Store.AddressLine,
			},
			Items: []model.OrderItem{{
				Title:     res.Offer.Title,
				OfferType: res.Offer.OfferType,
				Quantity:  res.Offer.Quantity,
				UnitPrice: res.Offer.UnitPrice,
			}},
			ItemsTotal:  res.ItemsTotal,
			ServiceFee:  res.ServiceFee,
			TotalAmount: res.TotalAmount,
			PickupStart: res.PickupStart,
			PickupEnd:   res.PickupEnd,
			CreatedAt:   res.CreatedAt,
		})
	}

	cp := cloneReservation(res)
	return &repository.CreateReservationResult{Reservation: &cp, Replayed: false}, nil
}

func (r *ReservationRepository) GetByPublicIDAndUser(
	_ context.Context,
	reservationPublicID, userPublicID string,
) (*model.Reservation, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	stored, ok := r.reservations[reservationPublicID]
	if !ok || stored.userID != userPublicID {
		return nil, repository.ErrReservationNotFound
	}
	cp := cloneReservation(stored.reservation)
	return &cp, nil
}

func cloneReservation(r model.Reservation) model.Reservation {
	cp := r
	cp.Store = cloneReservationStore(r.Store)
	if r.PickupStart != nil {
		t := *r.PickupStart
		cp.PickupStart = &t
	}
	if r.PickupEnd != nil {
		t := *r.PickupEnd
		cp.PickupEnd = &t
	}
	return cp
}

func cloneReservationStore(s model.ReservationStore) model.ReservationStore {
	cp := s
	if s.Latitude != nil {
		v := *s.Latitude
		cp.Latitude = &v
	}
	if s.Longitude != nil {
		v := *s.Longitude
		cp.Longitude = &v
	}
	return cp
}

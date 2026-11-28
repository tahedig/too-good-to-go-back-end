package memory

import (
	"context"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/tahedig/too-good-to-go/backend/internal/domain/enum"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/model"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/repository"
)

const (
	defaultPlanName       = "پایه · رایگان"
	defaultCommissionRate = 15.0
)

// VendorDashboardOffer represents an in-memory offer owned by a store for
// vendor dashboard calculations and tests.
type VendorDashboardOffer struct {
	ID                string
	StoreID           string
	Title             string
	ImageURL          *string
	Status            string // draft, active, sold_out, expired, cancelled
	QuantityTotal     int
	QuantityAvailable int
	PickupStart       time.Time
	PickupEnd         time.Time
}

// VendorDashboardRecord represents an in-memory reservation belonging to a store.
type VendorDashboardRecord struct {
	ID            string
	StoreID       string
	Status        enum.OrderStatus
	PaymentStatus string
	PickupCode    string
	CustomerName  string
	OfferID       string
	OfferImageURL *string
	Items         []model.OrderItem
	ItemsTotal    int64
	TotalAmount   int64
	VendorPayout  int64
	PickupStart   *time.Time
	PickupEnd     *time.Time
	CompletedAt   *time.Time
	CreatedAt     time.Time
}

type vendorStoreRecord struct {
	vendorUserID string
	store        model.VendorDashboardStore
}

// VendorDashboardRepository stores vendor dashboard fixtures in memory with
// the same scoping and aggregation rules as PostgreSQL.
type VendorDashboardRepository struct {
	mu              sync.Mutex
	stores          map[string]vendorStoreRecord
	offers          []VendorDashboardOffer
	reservations    []VendorDashboardRecord
	commissionRates map[string]float64
}

func NewVendorDashboardRepository() *VendorDashboardRepository {
	return &VendorDashboardRepository{
		stores:          make(map[string]vendorStoreRecord),
		commissionRates: make(map[string]float64),
	}
}

var _ repository.VendorDashboardRepository = (*VendorDashboardRepository)(nil)

func (r *VendorDashboardRepository) AddStore(vendorUserID string, s model.VendorDashboardStore) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s.ID = strings.ToLower(s.ID)
	s.CreatedAt = s.CreatedAt.UTC()
	r.stores[s.ID] = vendorStoreRecord{vendorUserID: strings.ToLower(vendorUserID), store: s}
}

func (r *VendorDashboardRepository) AddOffer(o VendorDashboardOffer) {
	r.mu.Lock()
	defer r.mu.Unlock()
	o.StoreID = strings.ToLower(o.StoreID)
	o.PickupStart = o.PickupStart.UTC()
	o.PickupEnd = o.PickupEnd.UTC()
	r.offers = append(r.offers, o)
}

func (r *VendorDashboardRepository) AddReservation(rec VendorDashboardRecord) {
	r.mu.Lock()
	defer r.mu.Unlock()
	rec.StoreID = strings.ToLower(rec.StoreID)
	rec.CreatedAt = rec.CreatedAt.UTC()
	r.reservations = append(r.reservations, rec)
}

func (r *VendorDashboardRepository) SetCommissionRate(vendorUserID string, rate float64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.commissionRates[strings.ToLower(vendorUserID)] = rate
}

func (r *VendorDashboardRepository) GetDashboard(_ context.Context, q repository.VendorDashboardQuery) (*model.VendorDashboard, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := q.Now.UTC()
	if now.IsZero() {
		now = time.Now().UTC()
	}
	vendorID := strings.ToLower(q.VendorUserID)
	filterStoreID := strings.ToLower(q.StoreID)

	owned := make([]model.VendorDashboardStore, 0)
	ownedByID := make(map[string]model.VendorDashboardStore)
	for _, rec := range r.stores {
		if rec.vendorUserID != vendorID {
			continue
		}
		owned = append(owned, rec.store)
		ownedByID[rec.store.ID] = rec.store
	}
	sort.Slice(owned, func(i, j int) bool {
		pi, pj := storeDisplayPriority(owned[i].Status), storeDisplayPriority(owned[j].Status)
		if pi != pj {
			return pi < pj
		}
		if !owned[i].CreatedAt.Equal(owned[j].CreatedAt) {
			return owned[i].CreatedAt.Before(owned[j].CreatedAt)
		}
		return owned[i].ID < owned[j].ID
	})

	scoped := make(map[string]model.VendorDashboardStore)
	var activeStore *model.VendorDashboardStore
	if filterStoreID != "" {
		st, ok := ownedByID[filterStoreID]
		if !ok {
			return nil, repository.ErrStoreNotFound
		}
		scoped[st.ID] = st
		cp := st
		activeStore = &cp
	} else {
		for _, st := range owned {
			scoped[st.ID] = st
		}
		if len(owned) > 0 {
			cp := owned[0]
			activeStore = &cp
		}
	}

	pendingBanners := make([]model.VendorPendingStoreBanner, 0)
	for _, st := range owned {
		if st.Status == enum.StoreStatusPending {
			pendingBanners = append(pendingBanners, model.VendorPendingStoreBanner{
				ID:          st.ID,
				StoreName:   st.Name,
				City:        st.City,
				AddressLine: st.AddressLine,
				Status:      st.Status,
				CreatedAt:   st.CreatedAt,
			})
		}
	}

	rate := defaultCommissionRate
	if custom, ok := r.commissionRates[vendorID]; ok {
		rate = custom
	}
	periodStart, periodEnd, remainingDays := planPeriodWindow(now)

	dash := &model.VendorDashboard{
		ActiveStore:          activeStore,
		Stores:               owned,
		PendingStoreRequests: pendingBanners,
		Financial: model.VendorDashboardFinancial{
			Plan: model.VendorDashboardPlan{
				Name:           defaultPlanName,
				CommissionRate: rate,
				PeriodStart:    periodStart,
				PeriodEnd:      periodEnd,
				RemainingDays:  remainingDays,
			},
		},
		RecentReservations: []model.VendorDashboardReservation{},
	}

	if len(scoped) == 0 {
		return dash, nil
	}

	offersByID := make(map[string]VendorDashboardOffer, len(r.offers))
	for _, o := range r.offers {
		offersByID[o.ID] = o
		st, ok := scoped[o.StoreID]
		if !ok || st.Status != enum.StoreStatusApproved {
			continue
		}
		if o.Status == "active" && o.PickupEnd.After(now) {
			if o.QuantityAvailable > 0 {
				dash.KPIs.ActiveOffers++
			}
			dash.KPIs.AvailablePackages += o.QuantityAvailable
		}
	}

	dayStart, dayEnd := tehranDayBounds(now)
	var (
		scopedRes []VendorDashboardRecord
		nearest   *VendorDashboardRecord
	)
	for _, rec := range r.reservations {
		if _, ok := scoped[rec.StoreID]; !ok {
			continue
		}
		scopedRes = append(scopedRes, rec)
		qty := totalItemQty(rec.Items)

		switch rec.Status {
		case enum.OrderStatusPendingPayment:
			dash.ReservationCounts.PendingPayment++
		case enum.OrderStatusPaid:
			dash.ReservationCounts.Paid++
		case enum.OrderStatusReady:
			dash.ReservationCounts.Ready++
		case enum.OrderStatusCompleted:
			dash.ReservationCounts.Completed++
		case enum.OrderStatusCancelled:
			dash.ReservationCounts.Cancelled++
		case enum.OrderStatusExpired:
			dash.ReservationCounts.Expired++
		case enum.OrderStatusNoShow:
			dash.ReservationCounts.NoShow++
		}
		dash.ReservationCounts.Total++

		isActive := rec.Status == enum.OrderStatusPendingPayment || rec.Status == enum.OrderStatusPaid || rec.Status == enum.OrderStatusReady
		if isActive {
			dash.ReservationCounts.Active++
			dash.KPIs.ReservedPackages += qty
			if nearest == nil || isEarlierNearest(rec, *nearest, now) {
				cp := rec
				nearest = &cp
			}
		}

		switch rec.Status {
		case enum.OrderStatusCompleted:
			dash.Financial.ReceivedPayout += rec.VendorPayout
		case enum.OrderStatusPaid, enum.OrderStatusReady:
			dash.Financial.PendingPayout += rec.VendorPayout
		}

		if !rec.CreatedAt.Before(dayStart) && rec.CreatedAt.Before(dayEnd) {
			dash.Today.ReservationsCount++
			if rec.Status != enum.OrderStatusCancelled && rec.Status != enum.OrderStatusExpired {
				dash.Today.PackagesReserved += qty
			}
			if rec.Status == enum.OrderStatusPaid || rec.Status == enum.OrderStatusReady || rec.Status == enum.OrderStatusCompleted {
				dash.Today.Revenue += rec.VendorPayout
			}
		}
		if rec.Status == enum.OrderStatusCompleted {
			completedAt := rec.CreatedAt
			if rec.CompletedAt != nil {
				completedAt = rec.CompletedAt.UTC()
			}
			if !completedAt.Before(dayStart) && completedAt.Before(dayEnd) {
				dash.Today.CompletedCount++
			}
		}
	}

	dash.KPIs.ReceivedRevenue = dash.Financial.ReceivedPayout
	dash.Financial.NetPeriodRevenue = dash.Financial.ReceivedPayout + dash.Financial.PendingPayout
	dash.Financial.AvailableBalance = dash.Financial.ReceivedPayout

	if nearest != nil {
		st := scoped[nearest.StoreID]
		title := ""
		if len(nearest.Items) > 0 {
			title = nearest.Items[0].Title
		}
		img := nearest.OfferImageURL
		var avail, total int
		if o, ok := offersByID[nearest.OfferID]; ok {
			avail, total = o.QuantityAvailable, o.QuantityTotal
			if img == nil {
				img = o.ImageURL
			}
			if title == "" {
				title = o.Title
			}
		}
		reserved := max(total-avail, 0)
		dash.NearestReservation = &model.VendorNearestReservation{
			ID:                     nearest.ID,
			Status:                 nearest.Status,
			PickupCode:             nearest.PickupCode,
			CustomerName:           nearest.CustomerName,
			StoreID:                st.ID,
			StoreName:              st.Name,
			OfferTitle:             title,
			OfferImageURL:          img,
			Quantity:               totalItemQty(nearest.Items),
			TotalAmount:            nearest.TotalAmount,
			PickupStart:            nearest.PickupStart,
			PickupEnd:              nearest.PickupEnd,
			OfferQuantityAvailable: avail,
			OfferQuantityReserved:  reserved,
			OfferQuantityTotal:     total,
			CreatedAt:              nearest.CreatedAt,
		}
	}

	sort.Slice(scopedRes, func(i, j int) bool {
		return newerThan(scopedRes[i].CreatedAt, scopedRes[i].ID, scopedRes[j].CreatedAt, scopedRes[j].ID)
	})
	limit := q.RecentLimit
	if limit <= 0 || limit > len(scopedRes) {
		limit = len(scopedRes)
	}
	for _, rec := range scopedRes[:limit] {
		st := scoped[rec.StoreID]
		items := make([]model.OrderItem, len(rec.Items))
		copy(items, rec.Items)
		dash.RecentReservations = append(dash.RecentReservations, model.VendorDashboardReservation{
			ID:            rec.ID,
			Status:        rec.Status,
			PaymentStatus: rec.PaymentStatus,
			PickupCode:    rec.PickupCode,
			CustomerName:  rec.CustomerName,
			StoreID:       st.ID,
			StoreName:     st.Name,
			Items:         items,
			ItemsTotal:    rec.ItemsTotal,
			TotalAmount:   rec.TotalAmount,
			VendorPayout:  rec.VendorPayout,
			PickupStart:   rec.PickupStart,
			PickupEnd:     rec.PickupEnd,
			CreatedAt:     rec.CreatedAt,
		})
	}

	return dash, nil
}

func storeDisplayPriority(s enum.StoreStatus) int {
	switch s {
	case enum.StoreStatusApproved:
		return 0
	case enum.StoreStatusPending:
		return 1
	default:
		return 2
	}
}

func totalItemQty(items []model.OrderItem) int {
	sum := 0
	for _, it := range items {
		sum += it.Quantity
	}
	return sum
}

func tehranDayBounds(now time.Time) (time.Time, time.Time) {
	loc, err := time.LoadLocation("Asia/Tehran")
	if err != nil {
		loc = time.FixedZone("IRST", 3*3600+1800)
	}
	local := now.In(loc)
	start := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, loc).UTC()
	return start, start.Add(24 * time.Hour)
}

func planPeriodWindow(now time.Time) (time.Time, time.Time, int) {
	u := now.UTC()
	start := time.Date(u.Year(), u.Month(), 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, 0)
	rem := int(math.Ceil(end.Sub(u).Hours() / 24.0))
	if rem < 0 {
		rem = 0
	}
	return start, end, rem
}

func isEarlierNearest(a, b VendorDashboardRecord, now time.Time) bool {
	aUpcoming := a.PickupEnd != nil && !a.PickupEnd.Before(now)
	bUpcoming := b.PickupEnd != nil && !b.PickupEnd.Before(now)
	if aUpcoming != bUpcoming {
		return aUpcoming
	}
	if a.PickupStart != nil && b.PickupStart != nil && !a.PickupStart.Equal(*b.PickupStart) {
		return a.PickupStart.Before(*b.PickupStart)
	}
	if (a.PickupStart != nil) != (b.PickupStart != nil) {
		return a.PickupStart != nil
	}
	if !a.CreatedAt.Equal(b.CreatedAt) {
		return a.CreatedAt.Before(b.CreatedAt)
	}
	return a.ID < b.ID
}

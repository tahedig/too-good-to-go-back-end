package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/tahedig/too-good-to-go/backend/internal/application/dto"
	serviceinterface "github.com/tahedig/too-good-to-go/backend/internal/application/service/interface"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/exception"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/model"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/repository"
)

type publicStoreService struct {
	storeRepo repository.StoreRepository
	offerRepo repository.OfferRepository
}

// NewPublicStoreService creates a PublicStoreService.
func NewPublicStoreService(
	storeRepo repository.StoreRepository,
	offerRepo repository.OfferRepository,
) serviceinterface.PublicStoreService {
	return &publicStoreService{
		storeRepo: storeRepo,
		offerRepo: offerRepo,
	}
}

func (s *publicStoreService) GetPublicStore(ctx context.Context, publicID string) (*dto.PublicStoreResponse, error) {
	if strings.TrimSpace(publicID) == "" {
		return nil, exception.NewNotFound("store not found")
	}

	store, err := s.storeRepo.GetPublicByPublicID(ctx, publicID)
	if err != nil {
		return nil, exception.NewInternalServer(err)
	}

	// Unknown or unavailable (unapproved / inactive / deleted) stores return 404 without leaking private fields.
	if store == nil || !store.IsEligibleForPublic() {
		return nil, exception.NewNotFound("store not found")
	}

	var offers []*model.Offer
	if s.offerRepo != nil {
		offers, err = s.offerRepo.ListAvailableByStoreID(ctx, store.ID)
		if err != nil {
			return nil, exception.NewInternalServer(err)
		}
	}

	publicOffers := make([]dto.PublicOfferResponse, 0)
	totalAvailableQty := 0
	var firstPickupWindow string

	for _, o := range offers {
		if o != nil && o.IsAvailable() {
			totalAvailableQty += o.QuantityAvailable

			pickupWindow := o.PickupWindowText
			if pickupWindow == "" && !o.PickupStart.IsZero() && !o.PickupEnd.IsZero() {
				startStr := toPersianDigits(o.PickupStart.Format("15:04"))
				endStr := toPersianDigits(o.PickupEnd.Format("15:04"))
				pickupWindow = fmt.Sprintf("امروز، %s-%s", startStr, endStr)
			}
			if firstPickupWindow == "" {
				firstPickupWindow = pickupWindow
			}

			var distanceText string
			if o.DistanceKm != nil {
				distanceText = fmt.Sprintf("%s کیلومتر", toPersianDigits(fmt.Sprintf("%.1f", *o.DistanceKm)))
			}

			badgeText := fmt.Sprintf("%s بسته موجود", toPersianDigits(fmt.Sprintf("%d", o.QuantityAvailable)))

			currency := o.Currency
			if currency == "" {
				currency = "تومان"
			}

			publicOffers = append(publicOffers, dto.PublicOfferResponse{
				PublicID:          o.PublicID,
				Title:             o.Title,
				CategoryName:      store.CategoryName,
				DistanceText:      distanceText,
				ImageURL:          o.ImageURL,
				OriginalPrice:     o.OriginalPrice,
				Price:             o.Price,
				Currency:          currency,
				QuantityAvailable: o.QuantityAvailable,
				AvailabilityBadge: badgeText,
				PickupWindow:      pickupWindow,
			})
		}
	}

	var pickupSummary string
	if totalAvailableQty > 0 {
		qtyFa := toPersianDigits(fmt.Sprintf("%d", totalAvailableQty))
		if firstPickupWindow != "" {
			pickupSummary = fmt.Sprintf("%s موجود · تحویل %s", qtyFa, firstPickupWindow)
		} else {
			pickupSummary = fmt.Sprintf("%s موجود", qtyFa)
		}
	}

	isOpen := store.IsActive && store.AcceptsOrders
	statusBadge := "باز است"
	if !isOpen {
		statusBadge = "بسته است"
	}

	headerSubtitle := store.CategoryName
	districtOrCity := store.District
	if districtOrCity != nil && *districtOrCity != "" {
		headerSubtitle = fmt.Sprintf("%s · %s", store.CategoryName, *districtOrCity)
	} else if store.City != "" {
		headerSubtitle = fmt.Sprintf("%s · %s", store.CategoryName, store.City)
	}

	var coverImage *string
	if store.CoverImageURL != nil {
		coverImage = store.CoverImageURL
	} else {
		for _, img := range store.Images {
			if img.IsPrimary {
				coverImage = &img.URL
				break
			}
		}
		if coverImage == nil && len(store.Images) > 0 {
			coverImage = &store.Images[0].URL
		}
	}

	districtStr := ""
	if store.District != nil {
		districtStr = *store.District
	}

	scheduleItems := buildPublicWorkingSchedule(store.WorkingHours)

	return &dto.PublicStoreResponse{
		PublicID:             store.PublicID,
		Name:                 store.Name,
		CategoryName:         store.CategoryName,
		City:                 store.City,
		District:             districtStr,
		HeaderSubtitle:       headerSubtitle,
		IsOpen:               isOpen,
		StatusBadge:          statusBadge,
		Biography:            store.Biography,
		PickupAddress:        store.AddressLine,
		PickupInstructions:   store.PickupInstructions,
		PickupSummary:        pickupSummary,
		Location: dto.StoreLocationResponse{
			Latitude:  store.Location.Latitude,
			Longitude: store.Location.Longitude,
		},
		CoverImageURL:        coverImage,
		WorkingHoursSchedule: scheduleItems,
		Offers:               publicOffers,
	}, nil
}

func buildPublicWorkingSchedule(hours []model.WorkingHour) []dto.PublicWorkingScheduleItem {
	if len(hours) == 0 {
		return []dto.PublicWorkingScheduleItem{}
	}

	var items []dto.PublicWorkingScheduleItem
	n := len(hours)
	i := 0

	for i < n {
		start := hours[i]
		j := i + 1

		for j < n && hours[j].IsClosed == start.IsClosed &&
			hours[j].OpenTime == start.OpenTime &&
			hours[j].CloseTime == start.CloseTime {
			j++
		}

		startFa := start.DayNameFa
		if startFa == "" {
			startFa = defaultDayNameFa(start.DayOfWeek)
		}

		var daysLabel string
		if j-1 == i {
			daysLabel = startFa
		} else {
			last := hours[j-1]
			lastFa := last.DayNameFa
			if lastFa == "" {
				lastFa = defaultDayNameFa(last.DayOfWeek)
			}
			daysLabel = fmt.Sprintf("%s تا %s", startFa, lastFa)
		}

		var hoursLabel string
		if start.IsClosed {
			hoursLabel = "تعطیل"
		} else {
			openFa := toPersianDigits(start.OpenTime)
			closeFa := toPersianDigits(start.CloseTime)
			hoursLabel = fmt.Sprintf("%s-%s", openFa, closeFa)
		}

		items = append(items, dto.PublicWorkingScheduleItem{
			Days:     daysLabel,
			Hours:    hoursLabel,
			IsClosed: start.IsClosed,
		})

		i = j
	}

	return items
}

func defaultDayNameFa(dayOfWeek int) string {
	switch dayOfWeek {
	case 0:
		return "شنبه"
	case 1:
		return "یکشنبه"
	case 2:
		return "دوشنبه"
	case 3:
		return "سه‌شنبه"
	case 4:
		return "چهارشنبه"
	case 5:
		return "پنج‌شنبه"
	case 6:
		return "جمعه"
	default:
		return ""
	}
}

func toPersianDigits(s string) string {
	r := strings.NewReplacer(
		"0", "۰",
		"1", "۱",
		"2", "۲",
		"3", "۳",
		"4", "۴",
		"5", "۵",
		"6", "۶",
		"7", "۷",
		"8", "۸",
		"9", "۹",
	)
	return r.Replace(s)
}


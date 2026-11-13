package service

import (
	"context"
	"encoding/base64"
	"slices"
	"strconv"
	"strings"
	"time"

	serviceinterface "github.com/tahedig/too-good-to-go/backend/internal/application/service/interface"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/enum"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/exception"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/repository"
	"github.com/tahedig/too-good-to-go/backend/pkg/security"
)

const (
	DefaultOrderPageSize = 20
	MaxOrderPageSize     = 50
)

type orderService struct {
	orders repository.OrderRepository
}

func NewOrderService(orders repository.OrderRepository) serviceinterface.OrderService {
	return &orderService{orders: orders}
}

func (s *orderService) ListMyOrders(ctx context.Context, userID string, in serviceinterface.ListOrdersInput) (*serviceinterface.OrderPage, error) {
	if userID == "" {
		return nil, exception.NewUnauthorized("authentication required")
	}
	q, err := parseListOrdersInput(in)
	if err != nil {
		return nil, err
	}
	q.UserID = userID

	// One extra row tells whether another page exists.
	pageSize := q.Limit
	q.Limit++
	orders, err := s.orders.ListByUser(ctx, q)
	if err != nil {
		return nil, exception.NewInternalServer(err)
	}

	page := &serviceinterface.OrderPage{Orders: orders}
	if len(orders) > pageSize {
		page.Orders = orders[:pageSize]
		last := page.Orders[pageSize-1]
		page.NextCursor = encodeOrderCursor(repository.OrderCursor{CreatedAt: last.CreatedAt, ID: last.ID})
	}
	return page, nil
}

func parseListOrdersInput(in serviceinterface.ListOrdersInput) (repository.OrderListQuery, error) {
	var (
		q      = repository.OrderListQuery{Limit: DefaultOrderPageSize}
		fields []exception.FieldError
	)

	if in.Limit != "" {
		n, err := strconv.Atoi(in.Limit)
		if err != nil || n < 1 || n > MaxOrderPageSize {
			fields = append(fields, exception.FieldError{Field: "limit",
				Message: "must be a number from 1 to " + strconv.Itoa(MaxOrderPageSize)})
		}
		q.Limit = n
	}

	switch group := enum.OrderGroup(in.Group); group {
	case "", "all":
	case enum.OrderGroupUpcoming, enum.OrderGroupPast:
		q.Statuses = group.Statuses()
	default:
		fields = append(fields, exception.FieldError{Field: "group", Message: "must be all, upcoming or past"})
	}

	if in.Status != "" {
		var wanted []enum.OrderStatus
		for _, raw := range strings.Split(in.Status, ",") {
			status := enum.OrderStatus(strings.TrimSpace(raw))
			if !status.Valid() {
				fields = append(fields, exception.FieldError{Field: "status",
					Message: "must be a comma-separated list of: pending_payment, paid, ready, completed, cancelled, expired, no_show"})
				break
			}
			if !slices.Contains(wanted, status) {
				wanted = append(wanted, status)
			}
		}
		if q.Statuses != nil {
			// Both given: only statuses in the group and in the list. The
			// result may be empty, which matches no orders.
			wanted = slices.DeleteFunc(wanted, func(s enum.OrderStatus) bool { return !slices.Contains(q.Statuses, s) })
			if wanted == nil {
				wanted = []enum.OrderStatus{}
			}
		}
		q.Statuses = wanted
	}

	if in.Cursor != "" {
		cursor, ok := decodeOrderCursor(in.Cursor)
		if !ok {
			fields = append(fields, exception.FieldError{Field: "cursor", Message: "is not a cursor returned by this endpoint"})
		}
		q.After = cursor
	}

	if len(fields) > 0 {
		return q, exception.NewBadRequest("invalid order list query", fields)
	}
	return q, nil
}

// The cursor is opaque to clients: base64url of "<created_at RFC3339Nano>|<order public ID>".
func encodeOrderCursor(c repository.OrderCursor) string {
	return base64.RawURLEncoding.EncodeToString([]byte(c.CreatedAt.UTC().Format(time.RFC3339Nano) + "|" + c.ID))
}

func decodeOrderCursor(s string) (*repository.OrderCursor, bool) {
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return nil, false
	}
	ts, id, ok := strings.Cut(string(raw), "|")
	if !ok || !security.IsID(id) {
		return nil, false
	}
	t, err := time.Parse(time.RFC3339Nano, ts)
	if err != nil {
		return nil, false
	}
	return &repository.OrderCursor{CreatedAt: t, ID: strings.ToLower(id)}, true
}

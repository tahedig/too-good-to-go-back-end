package exception

import "errors"

var (
	ErrOrderNotFound          = NewNotFound("سفارش یافت نشد")
	ErrStoreNotFound          = NewNotFound("فروشگاه فعالی برای این فروشنده یافت نشد")
	ErrInvalidPickupCode      = NewBadRequest("کد تحویل نامعتبر است", nil)
	ErrOrderAlreadyDelivered  = NewConflict("سفارش قبلاً تحویل داده شده است")
	ErrOrderCannotBeDelivered = NewBadRequest("وضعیت سفارش اجازه تحویل را نمی‌دهد", nil)
	ErrInvalidOrderTransition = NewConflict("تغییر وضعیت سفارش از وضعیت فعلی مجاز نیست")
)

// IsDomainOrderError checks if an error is one of the domain order errors.
func IsDomainOrderError(err error) bool {
	return errors.Is(err, ErrOrderNotFound) ||
		errors.Is(err, ErrStoreNotFound) ||
		errors.Is(err, ErrInvalidPickupCode) ||
		errors.Is(err, ErrOrderAlreadyDelivered) ||
		errors.Is(err, ErrOrderCannotBeDelivered) ||
		errors.Is(err, ErrInvalidOrderTransition)
}

package subscription

import (
	"errors"
	"fmt"
)

var (
	ErrSubjectRequired     = errors.New("subscription subject is required")
	ErrGrantIDRequired     = errors.New("subscription grant id is required")
	ErrQuotaInvalid        = errors.New("subscription quota is invalid")
	ErrPlanNotFound        = errors.New("subscription plan not found")
	ErrGrantNotFound       = errors.New("subscription grant not found")
	ErrReservationNotFound = errors.New("subscription reservation not found")
)

type ErrLimitExceededError struct {
	Quota   QuotaKey
	Current int
	Limit   int
}

func (e ErrLimitExceededError) Error() string {
	return fmt.Sprintf("quota exceeded for %s: %d/%d", e.Quota.String(), e.Current, e.Limit)
}

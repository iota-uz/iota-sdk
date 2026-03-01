package subscription

import "fmt"

var (
	ErrTierNotConfigured = fmt.Errorf("subscription tier is not configured")
)

type ErrLimitExceeded struct {
	EntityType string
	Current    int
	Limit      int
}

func (e ErrLimitExceeded) Error() string {
	return fmt.Sprintf("limit exceeded for %s: %d/%d", e.EntityType, e.Current, e.Limit)
}

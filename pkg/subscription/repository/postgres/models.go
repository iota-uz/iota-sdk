package postgres

import "time"

type entitlementModel struct {
	TenantID              string
	PlanID                string
	StripeSubscriptionID  *string
	StripeCustomerID      *string
	Features              []byte
	EntityLimits          []byte
	SeatLimit             *int
	CurrentSeats          int
	InGracePeriod         bool
	GracePeriodEndsAt     *time.Time
	LastSyncedAt          *time.Time
	StripeSubscriptionEnd *time.Time
	CreatedAt             time.Time
	UpdatedAt             time.Time
}

type planModel struct {
	PlanID       string
	ParentPlanID *string
	Name         string
	Description  string
	PriceCents   int64
	Interval     string
	Features     []byte
	EntityLimits []byte
	SeatLimit    *int
	DisplayOrder int
}

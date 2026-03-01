package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

var ErrEntitlementNotFound = errors.New("subscription entitlement not found")

type Entitlement struct {
	TenantID              uuid.UUID
	PlanID                string
	StripeSubscriptionID  *string
	StripeCustomerID      *string
	Features              []string
	EntityLimits          map[string]int
	SeatLimit             *int
	CurrentSeats          int
	InGracePeriod         bool
	GracePeriodEndsAt     *time.Time
	LastSyncedAt          *time.Time
	StripeSubscriptionEnd *time.Time
	CreatedAt             time.Time
	UpdatedAt             time.Time
}

type Plan struct {
	PlanID       string
	DisplayName  string
	Description  string
	PriceCents   int64
	Interval     string
	Features     []string
	EntityLimits map[string]int
	SeatLimit    *int
	DisplayOrder int
}

type Repository interface {
	GetEntitlement(ctx context.Context, tenantID uuid.UUID) (*Entitlement, error)
	UpsertEntitlement(ctx context.Context, entitlement *Entitlement) error
	SetStripeReferences(ctx context.Context, tenantID uuid.UUID, customerID, subscriptionID *string) error
	FindTenantByStripeCustomer(ctx context.Context, customerID string) (uuid.UUID, error)
	FindTenantByStripeSubscription(ctx context.Context, subscriptionID string) (uuid.UUID, error)
	SetGracePeriod(ctx context.Context, tenantID uuid.UUID, inGrace bool, endsAt *time.Time) error
	SetPlan(ctx context.Context, tenantID uuid.UUID, planID string) error
	TouchSyncedAt(ctx context.Context, tenantID uuid.UUID, syncedAt time.Time) error

	GetEntityCounts(ctx context.Context, tenantID uuid.UUID) (map[string]int, error)
	GetEntityCount(ctx context.Context, tenantID uuid.UUID, entityType string) (int, error)
	SetEntityCount(ctx context.Context, tenantID uuid.UUID, entityType string, count int) error
	IncrementEntityCount(ctx context.Context, tenantID uuid.UUID, entityType string) error
	IncrementEntityCountIfBelow(ctx context.Context, tenantID uuid.UUID, entityType string, maxCount int) (bool, error)
	DecrementEntityCount(ctx context.Context, tenantID uuid.UUID, entityType string) error

	AddSeatIfBelow(ctx context.Context, tenantID uuid.UUID, maxCount int) (bool, error)
	IncrementSeat(ctx context.Context, tenantID uuid.UUID) error
	DecrementSeat(ctx context.Context, tenantID uuid.UUID) error

	UpsertPlans(ctx context.Context, plans []Plan) error
}

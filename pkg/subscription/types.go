package subscription

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

type Scope string

const (
	ScopeGlobal  Scope = "global"
	ScopeTenant  Scope = "tenant"
	ScopeUser    Scope = "user"
	ScopeTeam    Scope = "team"
	ScopeProject Scope = "project"
)

type SubjectRef struct {
	Scope Scope
	ID    uuid.UUID
}

type Subject struct {
	Scope   Scope
	ID      uuid.UUID
	Parents []SubjectRef
}

func (s Subject) Ref() SubjectRef {
	return SubjectRef{
		Scope: s.Scope,
		ID:    s.ID,
	}
}

type FeatureKey string

type Window string

const (
	WindowNone  Window = "none"
	WindowHour  Window = "hour"
	WindowDay   Window = "day"
	WindowWeek  Window = "week"
	WindowMonth Window = "month"
)

type QuotaKey struct {
	Resource  string
	Dimension string
	Window    Window
}

func (q QuotaKey) String() string {
	return fmt.Sprintf("%s|%s|%s", q.Resource, q.Dimension, q.Window)
}

type GrantKind string

const (
	GrantKindDefault  GrantKind = "default"
	GrantKindPlan     GrantKind = "plan"
	GrantKindPromo    GrantKind = "promo"
	GrantKindAddOn    GrantKind = "addon"
	GrantKindOverride GrantKind = "override"
	GrantKindDeny     GrantKind = "deny"
)

type GrantEffect string

const (
	GrantEffectAllow GrantEffect = "allow"
	GrantEffectDeny  GrantEffect = "deny"
)

type QuotaMode string

const (
	QuotaModeAbsolute QuotaMode = "absolute"
	QuotaModeAdditive QuotaMode = "additive"
)

type QuotaRule struct {
	Effect GrantEffect
	Limit  int
	Mode   QuotaMode
}

type Grant struct {
	ID        string
	Kind      GrantKind
	Subject   SubjectRef
	PlanID    string
	Features  map[FeatureKey]GrantEffect
	Quotas    map[string]QuotaRule
	Priority  int
	Version   int
	Metadata  map[string]string
	ExpiresAt *time.Time
	CreatedAt time.Time
	UpdatedAt time.Time
}

type DecisionSource struct {
	GrantID  string
	Kind     GrantKind
	Priority int
	Reason   string
}

type Decision struct {
	Allowed bool
	Subject SubjectRef
	Feature FeatureKey
	PlanID  string
	Reason  string
	Version string
	Sources []DecisionSource
}

type LimitDecision struct {
	Allowed   bool
	Subject   SubjectRef
	Quota     QuotaKey
	Current   int
	Limit     int
	Remaining int
	PlanID    string
	Reason    string
	Version   string
	Sources   []DecisionSource
}

type ReservationStatus string

const (
	ReservationPending   ReservationStatus = "pending"
	ReservationCommitted ReservationStatus = "committed"
	ReservationReleased  ReservationStatus = "released"
	ReservationExpired   ReservationStatus = "expired"
)

type Reservation struct {
	ID          string
	Token       string
	Subject     SubjectRef
	Quota       QuotaKey
	Amount      int
	Status      ReservationStatus
	ReservedAt  time.Time
	ExpiresAt   time.Time
	CommittedAt *time.Time
	ReleasedAt  *time.Time
}

type PlanInfo struct {
	ID          string
	DisplayName string
	Description string
}

type PlanDefinition struct {
	PlanID       string
	DisplayName  string
	Description  string
	PriceCents   int64
	Interval     string
	Features     []string
	EntityLimits map[string]int
	SeatLimit    *int
	DisplayOrder int
	ParentPlanID string
}

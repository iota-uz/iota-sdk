package subscription

import (
	"fmt"
	"strings"
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

// SubjectRef is a compact identifier for grant ownership and usage keys.
// Use SubjectRef in storage/indexing and Subject for runtime policy evaluation.
type SubjectRef struct {
	Scope Scope
	ID    uuid.UUID
}

// Subject carries full evaluation context for policy checks.
// It includes the concrete subject and optional parent references that inherit
// grants (for example, user -> team -> tenant -> global).
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
	WindowNone Window = "none"
)

type QuotaKey struct {
	Resource  string
	Dimension string
	Window    Window
}

// NewQuotaKey constructs and validates a quota key.
func NewQuotaKey(resource, dimension string, window Window) (QuotaKey, error) {
	return validateQuota(QuotaKey{
		Resource:  strings.TrimSpace(resource),
		Dimension: strings.TrimSpace(dimension),
		Window:    window,
	})
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
	ID       string
	Kind     GrantKind
	Subject  SubjectRef
	PlanID   string
	Features map[FeatureKey]GrantEffect
	// Quotas maps a quota key string to a rule. The key is either the full
	// QuotaKey.String() value ("resource|dimension|window") for dimension-aware
	// lookups, or just the resource name for dimension-less quotas. See
	// matchQuotaRule in subscription.go for the lookup priority.
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

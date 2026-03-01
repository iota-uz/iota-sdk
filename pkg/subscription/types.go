package subscription

import "time"

type LimitResult struct {
	Allowed    bool
	Current    int
	Limit      int
	Percentage float64
	Message    string
}

type Limit struct {
	EntityType  string
	Current     int
	Max         int
	IsUnlimited bool
	Percentage  float64
}

type TierInfo struct {
	Tier        string
	DisplayName string
	Features    []string
	Limits      map[string]int
	SeatLimit   *int
	ExpiresAt   *time.Time
	InGrace     bool
	GraceEndsAt *time.Time
}

type TierDefinition struct {
	Tier         string
	DisplayName  string
	Description  string
	PriceCents   int64
	Interval     string
	Features     []string
	EntityLimits map[string]int
	SeatLimit    *int
	DisplayOrder int
	ParentTier   string
}

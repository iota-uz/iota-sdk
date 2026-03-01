package testing

import "github.com/iota-uz/iota-sdk/pkg/subscription"

func DefaultTiers() []subscription.TierDefinition {
	basicSeatLimit := 5
	return []subscription.TierDefinition{
		{
			Tier:         "FREE",
			DisplayName:  "Free",
			PriceCents:   0,
			Interval:     "month",
			Features:     []string{"core_access"},
			EntityLimits: map[string]int{"drivers": 50, "loads": 5000},
			DisplayOrder: 0,
		},
		{
			Tier:         "BASIC",
			DisplayName:  "Basic",
			PriceCents:   2900,
			Interval:     "month",
			Features:     []string{"core_access", "reporting"},
			EntityLimits: map[string]int{"drivers": 200, "loads": 15000},
			SeatLimit:    &basicSeatLimit,
			DisplayOrder: 1,
			ParentTier:   "FREE",
		},
		{
			Tier:         "PRO",
			DisplayName:  "Pro",
			PriceCents:   9900,
			Interval:     "month",
			Features:     []string{"shyona_access"},
			EntityLimits: map[string]int{"drivers": -1, "loads": -1},
			DisplayOrder: 2,
			ParentTier:   "BASIC",
		},
	}
}

func DefaultConfig() subscription.Config {
	return subscription.Config{
		GracePeriodDays:       7,
		LimitWarningThreshold: 0.8,
		DefaultTier:           "FREE",
		Tiers:                 DefaultTiers(),
	}
}

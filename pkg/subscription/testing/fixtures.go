// Package testing provides fixtures and mocks for subscription package tests.
package testing

import "github.com/iota-uz/iota-sdk/pkg/subscription"

func DefaultPlans() []subscription.PlanDefinition {
	basicSeatLimit := 5
	return []subscription.PlanDefinition{
		{
			PlanID:       "FREE",
			DisplayName:  "Free",
			PriceCents:   0,
			Interval:     "month",
			Features:     []string{"core_access"},
			EntityLimits: map[string]int{"drivers": 50, "loads": 5000},
			DisplayOrder: 0,
		},
		{
			PlanID:       "BASIC",
			DisplayName:  "Basic",
			PriceCents:   2900,
			Interval:     "month",
			Features:     []string{"core_access", "reporting"},
			EntityLimits: map[string]int{"drivers": 200, "loads": 15000},
			SeatLimit:    &basicSeatLimit,
			DisplayOrder: 1,
			ParentPlanID: "FREE",
		},
		{
			PlanID:       "PRO",
			DisplayName:  "Pro",
			PriceCents:   9900,
			Interval:     "month",
			Features:     []string{"shyona_access"},
			EntityLimits: map[string]int{"drivers": -1, "loads": -1},
			DisplayOrder: 2,
			ParentPlanID: "BASIC",
		},
	}
}

func DefaultConfig() subscription.Config {
	return subscription.Config{
		DefaultPlan: "FREE",
		Plans:       DefaultPlans(),
	}
}

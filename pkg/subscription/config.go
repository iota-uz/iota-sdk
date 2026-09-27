// Package subscription provides policy evaluation, limits, and plan entitlements.
package subscription

import "time"

type Config struct {
	DefaultPlan    string
	Plans          []PlanDefinition
	ReservationTTL time.Duration
}

func (c Config) normalized() Config {
	out := c
	if out.DefaultPlan == "" {
		out.DefaultPlan = "FREE"
	}
	if out.ReservationTTL <= 0 {
		out.ReservationTTL = 15 * time.Minute
	}
	return out
}

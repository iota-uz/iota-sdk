package viewmodels

import (
	"fmt"
	"sort"

	"github.com/iota-uz/iota-sdk/pkg/subscription"
)

type PlanViewModel struct {
	Tier         string
	DisplayName  string
	Description  string
	PriceLabel   string
	Features     []string
	EntityLimits map[string]int
	SeatLabel    string
	DisplayOrder int
}

func FromTierDefinitions(tiers []subscription.TierDefinition) []PlanViewModel {
	plans := make([]PlanViewModel, 0, len(tiers))
	for _, tier := range tiers {
		features := append([]string{}, tier.Features...)
		sort.Strings(features)
		priceLabel := "$0"
		if tier.PriceCents > 0 {
			priceLabel = fmt.Sprintf("$%.2f/%s", float64(tier.PriceCents)/100.0, tier.Interval)
		}
		seatLabel := "Unlimited"
		if tier.SeatLimit != nil {
			seatLabel = fmt.Sprintf("%d", *tier.SeatLimit)
		}

		limits := make(map[string]int, len(tier.EntityLimits))
		for key, value := range tier.EntityLimits {
			limits[key] = value
		}

		plans = append(plans, PlanViewModel{
			Tier:         tier.Tier,
			DisplayName:  tier.DisplayName,
			Description:  tier.Description,
			PriceLabel:   priceLabel,
			Features:     features,
			EntityLimits: limits,
			SeatLabel:    seatLabel,
			DisplayOrder: tier.DisplayOrder,
		})
	}

	sort.SliceStable(plans, func(i, j int) bool {
		if plans[i].DisplayOrder == plans[j].DisplayOrder {
			return plans[i].Tier < plans[j].Tier
		}
		return plans[i].DisplayOrder < plans[j].DisplayOrder
	})
	return plans
}

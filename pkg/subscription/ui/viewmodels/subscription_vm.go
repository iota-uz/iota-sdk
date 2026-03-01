package viewmodels

import (
	"fmt"
	"sort"

	"github.com/iota-uz/iota-sdk/pkg/subscription"
)

type PlanViewModel struct {
	PlanID       string
	DisplayName  string
	Description  string
	PriceLabel   string
	Features     []string
	EntityLimits map[string]int
	SeatLabel    string
	DisplayOrder int
}

func FromPlanDefinitions(definitions []subscription.PlanDefinition) []PlanViewModel {
	plans := make([]PlanViewModel, 0, len(definitions))
	for _, definition := range definitions {
		features := append([]string{}, definition.Features...)
		sort.Strings(features)
		priceLabel := "$0"
		if definition.PriceCents > 0 {
			if definition.Interval == "" {
				priceLabel = fmt.Sprintf("$%.2f", float64(definition.PriceCents)/100.0)
			} else {
				priceLabel = fmt.Sprintf("$%.2f/%s", float64(definition.PriceCents)/100.0, definition.Interval)
			}
		}
		seatLabel := "Unlimited"
		if definition.SeatLimit != nil && *definition.SeatLimit >= 0 {
			seatLabel = fmt.Sprintf("%d", *definition.SeatLimit)
		}

		limits := make(map[string]int, len(definition.EntityLimits))
		for key, value := range definition.EntityLimits {
			limits[key] = value
		}

		plans = append(plans, PlanViewModel{
			PlanID:       definition.PlanID,
			DisplayName:  definition.DisplayName,
			Description:  definition.Description,
			PriceLabel:   priceLabel,
			Features:     features,
			EntityLimits: limits,
			SeatLabel:    seatLabel,
			DisplayOrder: definition.DisplayOrder,
		})
	}

	sort.SliceStable(plans, func(i, j int) bool {
		if plans[i].DisplayOrder == plans[j].DisplayOrder {
			return plans[i].PlanID < plans[j].PlanID
		}
		return plans[i].DisplayOrder < plans[j].DisplayOrder
	})
	return plans
}

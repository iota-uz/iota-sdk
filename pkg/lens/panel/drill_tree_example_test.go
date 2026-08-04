package panel_test

import (
	"fmt"

	"github.com/iota-uz/iota-sdk/pkg/lens/action"
	"github.com/iota-uz/iota-sdk/pkg/lens/panel"
)

func ExampleDrillTree() {
	navigate := action.Navigate("/policies?year=2025&quarter=1")
	tree := panel.DrillTree{Branches: []panel.DrillBranch{{
		TriggerKey: "earned",
		Label:      "Earned premium",
		Children: []panel.DrillNode{{
			Key:   "year:2025",
			Label: "2025",
			Value: 125_000,
			Children: []panel.DrillNode{{
				Key:    "quarter:2025:q1",
				Label:  "Q1",
				Value:  125_000,
				Action: &navigate,
			}},
		}},
	}}}

	chart := panel.Pie("premium", "Premium", "premium_breakdown").
		LabelField("segment").
		ValueField("amount").
		IDField("segment_key").
		DrillTree(tree).
		Build()

	fmt.Println(chart.Fields.ID, chart.DrillTree.Branches[0].TriggerKey)
	// Output: segment_key earned
}

package panel_test

import (
	"fmt"

	"github.com/iota-uz/iota-sdk/pkg/lens/action"
	"github.com/iota-uz/iota-sdk/pkg/lens/panel"
)

// ExampleMetricFlow demonstrates the three metric primitives composed inside
// nested tabs: one stable parent metric with one active decomposition lens at
// a time. Switching a tab never changes the parent, period, or perimeter —
// only which composition is on screen.
func ExampleMetricFlow() {
	openStage := action.OpenDrawer("/metrics/flow/result")

	flow := panel.MetricFlow("flow", "Result formula", "flow_dataset",
		panel.FlowStage{Key: "opening", Label: "Opening balance", Role: panel.FlowRoleInput},
		panel.FlowStage{
			Key: "movement", Label: "Movement", Role: panel.FlowRoleAdd,
			Availability: panel.AvailabilityConfigRequired,
		},
		panel.FlowStage{
			Key: "adjustment", Label: "Adjustment", Role: panel.FlowRoleSubtract,
			Confidence: panel.ConfidenceRequiresReconciliation,
		},
		panel.FlowStage{
			Key: "closing", Label: "Closing balance", Role: panel.FlowRoleResult, Caption: "period end",
			Action: &openStage,
		},
	).FlowReconcile(0.5).Terminal().Build()

	hierarchy := panel.MetricHierarchy("hierarchy", "Decomposition", "hierarchy_dataset",
		panel.HierarchyRow{Key: "total", Label: "Total"},
		panel.HierarchyRow{Key: "segment-a", Label: "Segment A", Parent: "total"},
		panel.HierarchyRow{Key: "segment-b", Label: "Segment B", Parent: "total"},
		panel.HierarchyRow{Key: "unallocated", Label: "Unallocated", Parent: "total", Unallocated: true},
	).HierarchyReconcile(0.5).Terminal().Build()

	relationship := panel.MetricRelationship("relationship", "Related metric", "relationship_dataset", panel.RelationshipSpec{
		Source: panel.RelationshipEnd{Key: "deferred-amount", Label: "Deferred amount"},
		Target: panel.RelationshipEnd{Key: "closing-balance", Label: "Closing balance"},
		Type:   panel.RelationshipAssociation,
	}).Terminal().Build()

	movement := panel.Tabs("movement", "Movement", flow, relationship).Build()
	stock := panel.Tabs("stock", "Stock", hierarchy).Build()
	composition := panel.Tabs("composition", "Composition", stock, movement).Build()

	fmt.Println(composition.Kind, len(composition.Children))
	fmt.Println(stock.Children[0].Kind, stock.Children[0].ID)
	fmt.Println(movement.Children[0].Kind, movement.Children[0].ID)
	fmt.Println(movement.Children[1].Kind, movement.Children[1].ID)
	// Output:
	// tabs 2
	// metric_hierarchy hierarchy
	// metric_flow flow
	// metric_relationship relationship
}

package spec

import (
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/lens/action"
	"github.com/iota-uz/iota-sdk/pkg/lens/panel"
	"github.com/stretchr/testify/require"
)

func TestFinalizeInteractionContractMarksOnlyNonActionableLeaves(t *testing.T) {
	t.Parallel()
	actionSpec := &action.Spec{}
	document := Document{
		Explorers: []ExplorerSpec{{HostPanelID: "explorer"}},
		Rows: []RowSpec{{Panels: []PanelSpec{
			{ID: "plain"},
			{ID: "already-terminal", Terminal: true},
			{ID: "panel-action", Action: actionSpec},
			{ID: "drill", DrillTree: &panel.DrillTree{}},
			{ID: "column-action", Columns: []TableColumnSpec{{Action: actionSpec}}},
			{ID: "flow-action", FlowStages: []panel.FlowStage{{Action: actionSpec}}},
			{ID: "hierarchy-action", HierarchyRows: []panel.HierarchyRow{{Action: actionSpec}}},
			{ID: "relationship-action", Relationship: &panel.RelationshipSpec{Source: panel.RelationshipEnd{Action: actionSpec}}},
			{ID: "explorer"},
			{ID: "container", Children: []PanelSpec{{ID: "nested-leaf"}, {ID: "nested-action", Action: actionSpec}}},
		}}},
	}

	finalized := FinalizeInteractionContract(document)
	panels := finalized.Rows[0].Panels
	require.True(t, panels[0].Terminal)
	require.True(t, panels[1].Terminal)
	for index := 2; index <= 8; index++ {
		require.Falsef(t, panels[index].Terminal, "panel %s must remain actionable", panels[index].ID)
	}
	require.False(t, panels[9].Terminal)
	require.True(t, panels[9].Children[0].Terminal)
	require.False(t, panels[9].Children[1].Terminal)
}

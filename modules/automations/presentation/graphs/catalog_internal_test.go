package graphs

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/automation/graph"
)

func TestEditorCatalogGroupsActionsByModuleAndSendsTheirRules(t *testing.T) {
	view := catalog.View{Catalog: graph.Catalog{
		Events: map[string]graph.EventSpec{
			"core.user.created":  {Title: "User created", Group: "core.users"},
			"core.group.created": {Title: "Group created", Group: "core.groups"},
			"crm.client.created": {Title: "Client created", Group: "crm.clients"},
		},
		Nodes: map[string]graph.NodeSpec{
			"action": {Title: "Action", Behavior: graph.BehaviorCall, Dispatch: &graph.Dispatch{Param: "action", Prefix: "action."}},
			"action.core.user.block": {
				Title: "Block user", Behavior: graph.BehaviorCall, Group: "users",
				Templates: []string{"args.reason"}, Values: []string{"args.user_id"},
			},
			"action.crm.chat.send_message": {Title: "Send chat message", Behavior: graph.BehaviorCall, Group: "crm"},
		},
	}}
	out := editorCatalog(context.Background(), view, nil, nil)
	nodes := map[string]CatalogNode{}
	for _, n := range out.Nodes {
		nodes[n.Type] = n
	}
	// A registered action sits under its module, not under one "Action" heading.
	assert.Equal(t, groupFallback("core"), nodes["action.core.user.block"].Group)
	assert.Equal(t, groupFallback("crm"), nodes["action.crm.chat.send_message"].Group)
	// Its CEL args are CEL to the editor; the front type says which param names the action.
	assert.Contains(t, nodes["action.core.user.block"].Expressions, ExpressionParam{Param: "args.user_id", Type: ExprDyn})
	assert.Equal(t, []string{"args.reason"}, nodes["action.core.user.block"].Templates)
	require.NotNil(t, nodes["action"].Dispatch)
	assert.Equal(t, CatalogDispatch{Param: "action", Prefix: "action."}, *nodes["action"].Dispatch)
	assert.Nil(t, nodes["action.core.user.block"].Dispatch)
	// Events are grouped by module (core.users and core.groups together).
	groups := map[string]string{}
	for _, e := range out.Events {
		groups[e.Key] = e.Group
	}
	assert.Equal(t, groups["core.user.created"], groups["core.group.created"])
	assert.Equal(t, groupFallback("core"), groups["core.user.created"])
	assert.Equal(t, groupFallback("crm"), groups["crm.client.created"])
}

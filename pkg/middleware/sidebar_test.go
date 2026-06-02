package middleware

import (
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func keysOf(items []types.NavigationItem) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, item.Key)
	}
	return out
}

func TestSplitPinnedItems(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		items        []types.NavigationItem
		wantPinned   []string
		wantUnpinned []string
	}{
		{
			name: "top-level pinned extracted and removed from unpinned",
			items: []types.NavigationItem{
				{Key: "a", Pinned: true},
				{Key: "b"},
				{Key: "c", Pinned: true},
			},
			wantPinned:   []string{"a", "c"},
			wantUnpinned: []string{"b"},
		},
		{
			name: "pinned child hoisted, parent kept with remaining children",
			items: []types.NavigationItem{
				{Key: "parent", Children: []types.NavigationItem{
					{Key: "child-pinned", Pinned: true},
					{Key: "child-plain"},
				}},
			},
			wantPinned:   []string{"child-pinned"},
			wantUnpinned: []string{"parent"},
		},
		{
			name: "parent pruned when all children pinned",
			items: []types.NavigationItem{
				{Key: "parent", Children: []types.NavigationItem{
					{Key: "c1", Pinned: true},
					{Key: "c2", Pinned: true},
				}},
			},
			wantPinned:   []string{"c1", "c2"},
			wantUnpinned: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			pinned, unpinned := splitPinnedItems(tt.items)
			assert.Equal(t, tt.wantPinned, keysOf(pinned))
			assert.Equal(t, tt.wantUnpinned, keysOf(unpinned))
		})
	}
}

func TestSplitPinnedItemsRemainingChildren(t *testing.T) {
	t.Parallel()

	_, unpinned := splitPinnedItems([]types.NavigationItem{
		{Key: "parent", Children: []types.NavigationItem{
			{Key: "child-pinned", Pinned: true},
			{Key: "child-plain"},
		}},
	})

	require.Len(t, unpinned, 1)
	require.Equal(t, []string{"child-plain"}, keysOf(unpinned[0].Children))
}

func TestGetEnabledNavItems(t *testing.T) {
	t.Parallel()

	t.Run("child inherits parent workspace when empty", func(t *testing.T) {
		t.Parallel()
		out := getEnabledNavItems([]types.NavigationItem{
			{Key: "parent", Workspace: "erp", Children: []types.NavigationItem{
				{Key: "c1", Href: "/c1"},
				{Key: "c2", Href: "/c2", Workspace: "crm"},
			}},
		})
		require.Len(t, out, 1)
		// Two children => parent retained, children carry inherited/own workspace.
		require.Equal(t, "parent", out[0].Key)
		require.Equal(t, "erp", out[0].Children[0].Workspace)
		require.Equal(t, "crm", out[0].Children[1].Workspace)
	})

	t.Run("single child collapses to the child", func(t *testing.T) {
		t.Parallel()
		out := getEnabledNavItems([]types.NavigationItem{
			{Key: "parent", Workspace: "erp", Children: []types.NavigationItem{
				{Key: "only", Href: "/only"},
			}},
		})
		require.Equal(t, []string{"only"}, keysOf(out))
		require.Equal(t, "erp", out[0].Workspace)
	})

	t.Run("single-child parent collapses to grandchild leaf", func(t *testing.T) {
		t.Parallel()
		// Parent has one child group; that group has one leaf. Both single-child
		// levels collapse so the grandchild is hoisted to the top, parent dropped.
		out := getEnabledNavItems([]types.NavigationItem{
			{Key: "parent", Workspace: "erp", Children: []types.NavigationItem{
				{Key: "group", Children: []types.NavigationItem{
					{Key: "leaf-deep", Href: "/deep"},
				}},
			}},
			{Key: "leaf", Href: "/leaf"},
		})
		require.Equal(t, []string{"leaf-deep", "leaf"}, keysOf(out))
	})

	t.Run("single-child group chain collapses to grandchild leaf", func(t *testing.T) {
		t.Parallel()
		// parent -> g1 (group) -> g1a (leaf, empty Children). Both single-child
		// levels collapse, hoisting g1a to the top.
		out := getEnabledNavItems([]types.NavigationItem{
			{Key: "parent", Workspace: "erp", Children: []types.NavigationItem{
				{Key: "g1", Children: []types.NavigationItem{
					{Key: "g1a", Children: []types.NavigationItem{}},
				}},
			}},
			{Key: "leaf", Href: "/leaf"},
		})
		// g1a is a leaf (empty Children) -> g1 collapses to g1a -> parent collapses to g1a.
		require.Equal(t, []string{"g1a", "leaf"}, keysOf(out))
	})
}

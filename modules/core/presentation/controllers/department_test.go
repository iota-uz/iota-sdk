package controllers

import (
	"testing"

	"github.com/iota-uz/iota-sdk/modules/core/presentation/viewmodels"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBuildParentOptions_ExcludesSubtree verifies the cycle guard: when editing
// a department, that department and its descendants must be removed from the
// parent-select options so the user cannot reparent it under its own subtree.
func TestBuildParentOptions_ExcludesSubtree(t *testing.T) {
	t.Parallel()
	vms := []*viewmodels.Department{
		{ID: "root", Name: "Root"},
		{ID: "child", Name: "Child"},
		{ID: "grandchild", Name: "Grandchild"},
		{ID: "sibling", Name: "Sibling"},
	}
	// Editing "child": exclude child + its descendant grandchild.
	excluded := map[string]struct{}{"child": {}, "grandchild": {}}

	opts := buildParentOptions(vms, excluded)

	require.Len(t, opts, 2)
	got := make(map[string]struct{}, len(opts))
	for _, o := range opts {
		got[o.ID] = struct{}{}
	}
	assert.Contains(t, got, "root")
	assert.Contains(t, got, "sibling")
	assert.NotContains(t, got, "child")
	assert.NotContains(t, got, "grandchild")
}

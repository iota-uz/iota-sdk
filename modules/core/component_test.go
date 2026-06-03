package core_test

import (
	"testing"

	"github.com/iota-uz/iota-sdk/modules/core"
	"github.com/iota-uz/iota-sdk/pkg/composition"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestComponentSkipAdminNavItems(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		opts             *core.ModuleOptions
		wantNavItems     bool
		wantAdminQuickLk bool
	}{
		{
			name:             "zero value preserves admin nav items",
			opts:             &core.ModuleOptions{},
			wantNavItems:     true,
			wantAdminQuickLk: true,
		},
		{
			name:             "SkipAdminNavItems suppresses nav items but keeps quick links",
			opts:             &core.ModuleOptions{SkipAdminNavItems: true},
			wantNavItems:     false,
			wantAdminQuickLk: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			component := core.NewComponent(tt.opts)
			contributions, err := composition.InspectStatic(component)
			require.NoError(t, err)

			if tt.wantNavItems {
				assert.NotEmpty(t, contributions.NavItems, "expected admin nav items to be contributed")
			} else {
				assert.Empty(t, contributions.NavItems, "expected no nav items when SkipAdminNavItems is true")
			}

			// Quick-link / spotlight registration is unaffected by SkipAdminNavItems:
			// self-service quick links plus admin quick links should still register.
			require.NotEmpty(t, contributions.QuickLinks)
			if tt.wantAdminQuickLk {
				assert.GreaterOrEqual(t, len(contributions.QuickLinks), 3,
					"admin quick links should still register regardless of SkipAdminNavItems")
			}
		})
	}
}

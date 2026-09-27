package ops

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegistry_ContainsExpectedOperations_Scenarios(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		operation string
	}{
		{name: "seed main is registered", operation: "seed.main"},
		{name: "seed superadmin is registered", operation: "seed.superadmin"},
		{name: "seed e2e is registered", operation: "seed.e2e"},
		{name: "db e2e create is registered", operation: "db.e2e.create"},
		{name: "db e2e drop is registered", operation: "db.e2e.drop"},
		{name: "db e2e reset is registered", operation: "db.e2e.reset"},
		{name: "db e2e migrate is registered", operation: "db.e2e.migrate"},
	}

	registry := Registry()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec, ok := registry[tt.operation]
			require.True(t, ok, "missing operation %q", tt.operation)
			assert.Equal(t, tt.operation, spec.Name)
			require.NotEmpty(t, spec.Steps)
			for _, step := range spec.Steps {
				assert.NotNil(t, step.Handler, "operation %q step %q has nil handler", tt.operation, step.ID)
			}
		})
	}
}

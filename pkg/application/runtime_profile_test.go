package application

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeCompositionProfile_Scenarios(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    CompositionProfile
		expected CompositionProfile
	}{
		{
			name:     "defaults to server",
			input:    "",
			expected: CompositionProfileServer,
		},
		{
			name:     "honors bootstrap",
			input:    CompositionProfileBootstrap,
			expected: CompositionProfileBootstrap,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := normalizeCompositionProfile(tt.input)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, got)
		})
	}
}

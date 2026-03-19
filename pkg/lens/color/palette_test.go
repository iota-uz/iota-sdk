package color

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCanonicalProductKey_AliasesRemainStable(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "online kasko alias", input: "ONLINE_KASKO", want: "KASKO"},
		{name: "web constructor alias", input: "WEB_CONSTRUCTOR", want: "EURO_KASKO"},
		{name: "numeric osago alias", input: "3", want: "OSAGO"},
		{name: "trims and normalizes", input: " osgor ", want: "OSGOR"},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, CanonicalProductKey(tc.input))
		})
	}
}

func TestSemantic_KnownPalettesAndFallbacksRemainDeterministic(t *testing.T) {
	t.Parallel()

	require.Equal(t, "#7C3AED", Semantic(ScopeProduct, "OSAGO"))
	require.Equal(t, "#DC2626", Semantic(ScopeProduct, "ONLINE_KASKO"))
	require.Equal(t, "#0F766E", Semantic(ScopeProduct, "WEB_CONSTRUCTOR"))
	require.Equal(t, "#10B981", Semantic(ScopePaymentMethod, "payme"))
	require.Equal(t, Semantic(ScopeAgency, "Alpha"), Semantic(ScopeAgency, "Alpha"))
	require.NotEqual(t, Semantic(ScopeAgency, "AB"), Semantic(ScopeAgency, "BA"))
}

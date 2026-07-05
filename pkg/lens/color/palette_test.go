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
	alphaColor := Semantic(ScopeAgency, "Alpha")
	require.Equal(t, alphaColor, Semantic(ScopeAgency, "Alpha"))
	require.NotEqual(t, Semantic(ScopeAgency, "AB"), Semantic(ScopeAgency, "BA"))
}

func TestCategorical_ReturnsPaletteFromStartAndCycles(t *testing.T) {
	t.Parallel()

	require.Nil(t, Categorical(0))
	require.Nil(t, Categorical(-1))

	want := []string{
		"#2563EB", "#0D9488", "#D97706", "#7C3AED", "#DC2626",
		"#0284C7", "#DB2777", "#65A30D", "#9333EA", "#64748B",
	}
	require.Equal(t, want, Categorical(10))
	require.Equal(t, want[:3], Categorical(3))

	cycled := Categorical(12)
	require.Len(t, cycled, 12)
	require.Equal(t, want, cycled[:10])
	require.Equal(t, want[0], cycled[10])
	require.Equal(t, want[1], cycled[11])
}

func TestSequence_IsScopeIndependent(t *testing.T) {
	t.Parallel()

	// The FNV scope-hash offset is gone: every scope yields the same
	// sequence, identical to Categorical.
	require.Equal(t, Categorical(5), Sequence("PRODUCT", 5))
	require.Equal(t, Sequence("PRODUCT", 5), Sequence("REGION", 5))
	require.Equal(t, Sequence("", 5), Sequence("ANYTHING", 5))
	require.Nil(t, Sequence("PRODUCT", 0))
}

func TestNeutralAndAccentTokens(t *testing.T) {
	t.Parallel()

	require.Equal(t, "#94A3B8", Neutral)
	require.Equal(t, "#2563EB", Accent())
}

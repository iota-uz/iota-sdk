package color

import (
	"testing"

	"github.com/stretchr/testify/require"
)

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

func TestNeutralAndAccentTokens(t *testing.T) {
	t.Parallel()

	require.Equal(t, "#94A3B8", Neutral)
	require.Equal(t, "#2563EB", Accent())
}

func TestSemanticCompatibilityPreservesProductAliases(t *testing.T) {
	t.Parallel()

	require.Equal(t, "OSAGO", CanonicalProductKey("3"))
	require.Equal(t, "#7C3AED", Semantic(ScopeProduct, "3"))
	require.Equal(t, []string{"#7C3AED", "#2563EB"}, Palette(ScopeProduct, []string{"OSAGO", "TRAVEL"}))
}

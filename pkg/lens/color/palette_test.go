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

	cases := []struct {
		name      string
		key       string
		canonical string
		color     string
	}{
		{name: "OSAGO legacy ID", key: "3", canonical: "OSAGO", color: "#4338CA"},
		{name: "TRAVEL legacy ID", key: "17", canonical: "TRAVEL", color: "#15803D"},
		{name: "OPO legacy ID", key: "144", canonical: "OPO", color: "#DC2626"},
		{name: "SMR legacy ID", key: "334", canonical: "SMR", color: "#A16207"},
		{name: "EURO KASKO legacy ID", key: "347", canonical: "EURO_KASKO", color: "#F97316"},
		{name: "KASKO legacy ID", key: "349", canonical: "KASKO", color: "#F97316"},
		{name: "OSGOR legacy ID", key: "4002", canonical: "OSGOR", color: "#0369A1"},
		{name: "OSGOP legacy ID", key: "4003", canonical: "OSGOP", color: "#7C3AED"},
		{name: "online KASKO alias", key: "ONLINE_KASKO", canonical: "KASKO", color: "#F97316"},
		{name: "web constructor alias", key: "WEB_CONSTRUCTOR", canonical: "EURO_KASKO", color: "#F97316"},
		{name: "electronic OSGOR alias", key: "EOSGOR", canonical: "OSGOR", color: "#0369A1"},
		{name: "electronic OSGOP alias", key: "EOSGOP", canonical: "OSGOP", color: "#7C3AED"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			require.Equal(t, tc.canonical, CanonicalProductKey(tc.key))
			require.Equal(t, tc.color, Semantic(ScopeProduct, tc.key))
		})
	}

	require.Equal(t, []string{"#4338CA", "#15803D"}, Palette(ScopeProduct, []string{"OSAGO", "TRAVEL"}))
}

package color

import (
	"math"
	"strconv"
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

func TestCategoricalKeyUsesGenericPalette(t *testing.T) {
	t.Parallel()
	// This would be falsely green if a domain map returned a color that also
	// happened to belong to the generic palette.
	color := CategoricalKey("PRODUCT", "OSAGO")
	require.Contains(t, Series(), color)
	require.Equal(t, color, CategoricalKey("PRODUCT", "OSAGO"))
	require.NotEqual(t, "#4338CA", color)
	require.Equal(t, Accent(), CategoricalKey("any", ""))
}

func TestCategoriesHoldTwelveUniqueColoursWithoutSemanticOnes(t *testing.T) {
	t.Parallel()

	categories := Categories()
	require.Len(t, categories, 12)
	require.Len(t, uniqueColours(categories), 12)
	for _, semantic := range []string{Positive, Negative, Neutral} {
		require.NotContains(t, categories, semantic)
	}
	require.Equal(t, "#16A34A", Positive)
	require.Equal(t, "#DC2626", Negative)

	categories[0] = "mutated"
	require.NotEqual(t, "mutated", Categories()[0], "Categories must hand out a copy")
}

func TestCategoryAtCycles(t *testing.T) {
	t.Parallel()

	categories := Categories()
	require.Equal(t, categories[0], CategoryAt(0))
	require.Equal(t, categories[11], CategoryAt(11))
	require.Equal(t, categories[0], CategoryAt(12))
	require.Equal(t, categories[3], CategoryAt(27))
	require.Equal(t, categories[0], CategoryAt(-5))
}

// TestCategoriesStayDistinguishable keeps every pair of entity colours, and
// every entity colour against the semantic ones, a clearly visible step apart
// in CIELAB, so a palette edit cannot quietly reintroduce lookalikes.
func TestCategoriesStayDistinguishable(t *testing.T) {
	t.Parallel()

	const minimum = 18.0
	all := append(Categories(), Positive, Negative, Neutral)
	for i := range all {
		for j := i + 1; j < len(all); j++ {
			distance := labDistance(t, all[i], all[j])
			require.GreaterOrEqualf(t, distance, minimum, "%s and %s are too close (%.1f)", all[i], all[j], distance)
		}
	}
}

func uniqueColours(colours []string) map[string]struct{} {
	unique := make(map[string]struct{}, len(colours))
	for _, colour := range colours {
		unique[colour] = struct{}{}
	}
	return unique
}

func labDistance(t *testing.T, left, right string) float64 {
	t.Helper()
	l1, a1, b1 := lab(t, left)
	l2, a2, b2 := lab(t, right)
	return math.Sqrt((l1-l2)*(l1-l2) + (a1-a2)*(a1-a2) + (b1-b2)*(b1-b2))
}

func lab(t *testing.T, hex string) (float64, float64, float64) {
	t.Helper()
	require.Len(t, hex, 7)
	channel := func(offset int) float64 {
		value, err := strconv.ParseUint(hex[offset:offset+2], 16, 8)
		require.NoError(t, err)
		c := float64(value) / 255
		if c <= 0.04045 {
			return c / 12.92
		}
		return math.Pow((c+0.055)/1.055, 2.4)
	}
	r, g, bl := channel(1), channel(3), channel(5)
	x := (0.4124*r + 0.3576*g + 0.1805*bl) / 0.95047
	y := 0.2126*r + 0.7152*g + 0.0722*bl
	z := (0.0193*r + 0.1192*g + 0.9505*bl) / 1.08883
	f := func(v float64) float64 {
		if v > 0.008856 {
			return math.Cbrt(v)
		}
		return 7.787*v + 16.0/116
	}
	fx, fy, fz := f(x), f(y), f(z)
	return 116*fy - 16, 500 * (fx - fy), 200 * (fy - fz)
}

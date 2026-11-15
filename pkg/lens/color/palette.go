// Package color provides the canonical fallback palette values for Lens charts.
package color

import (
	"slices"
)

// genericPalette is the Lens design system v2 categorical palette, and the only
// place it is written down. cmd/lens-typegen reads it through Series and emits
// web/lens/src/contract/palette.ts, which is what the React runtime actually
// paints with, so a colour changes here and is regenerated — never mirrored.
// The lead hex intentionally matches the runtime's --lens-accent-500 token in
// web/lens/src/styles.css, which is chrome rather than series colour.
var genericPalette = []string{
	"#2563EB",
	"#0D9488",
	"#D97706",
	"#7C3AED",
	"#DC2626",
	"#0284C7",
	"#DB2777",
	"#65A30D",
	"#9333EA",
	"#64748B",
}

// Neutral is reserved for "Others" buckets so aggregated remainders read as
// de-emphasized rather than as another category. It is generated into the
// runtime as PALETTE_NEUTRAL and equals the --lens-text-faint token both themes
// declare in web/lens/src/styles.css: a remainder is grey in either theme.
const Neutral = "#94A3B8"

// Semantic colours carry a meaning, so they never appear in Categories and a
// chart that colours entities must not hand them out as ordinary categories.
const (
	// Positive is "good": income, profit, growth, a met target. Green 600 reads
	// on both a white and a dark card.
	Positive = "#16A34A"
	// Negative is "bad": expense, loss, decline, a missed target. It is the red
	// of the generic palette, so a chart that mixes the two stays consistent.
	Negative = "#DC2626"
)

// categoryPalette is the palette for colouring entities from a company's own
// reference data (expense categories, accounts, counterparties, projects).
// It has twelve colours that stay apart on a white and on a dark card: no red
// and no grey, which mean "bad" and "other" (Negative, Neutral), no green,
// which means "good" (Positive), and no purple next to violet. Neighbours in
// the order differ in hue, so the first colours used are the most distinct.
// TestCategoriesStayDistinguishable holds the minimum perceptual distance.
var categoryPalette = []string{
	"#2563EB", // blue
	"#D97706", // amber
	"#0D9488", // teal
	"#DB2777", // pink
	"#7C3AED", // violet
	"#65A30D", // lime
	"#0284C7", // sky
	"#EA580C", // orange
	"#C026D3", // fuchsia
	"#92400E", // brown
	"#06B6D4", // cyan
	"#A78BFA", // light violet
}

// Categories returns the entity palette in its declared order. Use it where
// each category of a company's own reference data owns one colour; the
// semantic colours (Positive, Negative, Neutral) stay out of it.
func Categories() []string {
	return slices.Clone(categoryPalette)
}

// CategoryAt returns the colour of the entity at index, cycling through
// Categories. A negative index wraps to the start.
func CategoryAt(index int) string {
	if index < 0 {
		index = 0
	}
	return categoryPalette[index%len(categoryPalette)]
}

// Series returns the categorical palette in its declared order. It exists so
// cmd/lens-typegen can hand the values to the React runtime without a second
// copy of the literal; callers that only need colours should use Categorical.
func Series() []string {
	return slices.Clone(genericPalette)
}

// Accent returns the primary Lens accent color: the palette's own lead, which
// also matches the runtime's --lens-accent-500 token in web/lens/src/styles.css.
func Accent() string { return genericPalette[0] }

// Categorical returns the first n categorical palette colors, cycling through
// the palette when n exceeds its length. Every caller gets the same sequence
// starting at index 0, so all dashboards share one palette.
func Categorical(n int) []string {
	if n <= 0 {
		return nil
	}
	colors := make([]string, n)
	for i := 0; i < n; i++ {
		colors[i] = genericPalette[i%len(genericPalette)]
	}
	return colors
}

// CategoricalKey chooses a stable generic color for a category within a scale.
// Domain-specific aliases and colors belong to the consuming application.
func CategoricalKey(scale, key string) string {
	if key == "" {
		return genericPalette[0]
	}
	return genericPalette[stableIndex(scale+":"+key, len(genericPalette))]
}

func stableIndex(key string, size int) int {
	hash := uint64(14695981039346656037)
	for _, ch := range key {
		hash ^= uint64(ch)
		hash *= 1099511628211
	}
	return int(hash % uint64(size))
}

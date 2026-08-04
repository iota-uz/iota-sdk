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

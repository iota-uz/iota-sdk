// Package color provides the canonical fallback palette values for Lens charts.
package color

import (
	"slices"
	"strings"
)

// Deprecated compatibility scopes kept for existing server-side Lens specs.
// New runtime palettes are categorical; these constants preserve stable colors
// for consumers that already persist or compare semantic categories.
const (
	ScopeProduct        = "PRODUCT"
	ScopePaymentMethod  = "PAYMENT_METHOD"
	ScopeContractSource = "CONTRACT_SOURCE"
	ScopeAgency         = "AGENCY"
	ScopeRegion         = "REGION"
	ScopeGender         = "GENDER"
	ScopeVehicleType    = "VEHICLE_TYPE"
	ScopeDamageType     = "DAMAGE_TYPE"
	ScopeDecision       = "DECISION"
	ScopeClaimSource    = "CLAIM_SOURCE"
)

var productPalette = map[string]string{
	"OSAGO":      "#7C3AED",
	"TRAVEL":     "#2563EB",
	"KASKO":      "#DC2626",
	"EURO_KASKO": "#0F766E",
	"OSGOR":      "#D97706",
	"OSGOP":      "#DB2777",
	"SMR":        "#EA580C",
	"OPO":        "#16A34A",
}

var paymentMethodPalette = map[string]string{
	"CLICK":  "#2563EB",
	"PAYME":  "#10B981",
	"OCTO":   "#F97316",
	"STRIPE": "#7C3AED",
	"CASH":   "#475569",
}

var productAliases = map[string]string{
	"3":               "OSAGO",
	"17":              "TRAVEL",
	"144":             "OPO",
	"334":             "SMR",
	"347":             "EURO_KASKO",
	"349":             "KASKO",
	"4002":            "OSGOR",
	"4003":            "OSGOP",
	"ONLINE_KASKO":    "KASKO",
	"WEB_CONSTRUCTOR": "EURO_KASKO",
	"EOSGOR":          "OSGOR",
	"EOSGOP":          "OSGOP",
}

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

// Semantic returns a stable color for a scoped category.
// Deprecated: new callers should use Categorical for presentation-only colors.
func Semantic(scope, key string) string {
	scope = normalizeToken(scope)
	key = canonicalKey(scope, key)
	switch scope {
	case ScopeProduct:
		if color, ok := productPalette[key]; ok {
			return color
		}
	case ScopePaymentMethod:
		if color, ok := paymentMethodPalette[key]; ok {
			return color
		}
	}
	if key == "" {
		return genericPalette[0]
	}
	return genericPalette[stableIndex(scope+":"+key, len(genericPalette))]
}

// Palette maps each scoped category to its stable semantic color.
// Deprecated: new callers should use Categorical for presentation-only colors.
func Palette(scope string, keys []string) []string {
	colors := make([]string, 0, len(keys))
	for _, key := range keys {
		colors = append(colors, Semantic(scope, key))
	}
	return colors
}

// CanonicalProductKey returns the stable product identifier used by Semantic.
// Deprecated: consumers should normalize their own domain identifiers.
func CanonicalProductKey(key string) string {
	normalized := normalizeToken(key)
	if alias, ok := productAliases[normalized]; ok {
		return alias
	}
	return normalized
}

func normalizeToken(value string) string {
	value = strings.ToUpper(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, "-", "_")
	return strings.ReplaceAll(value, " ", "_")
}

func canonicalKey(scope, key string) string {
	if scope == ScopeProduct {
		return CanonicalProductKey(key)
	}
	return normalizeToken(key)
}

func stableIndex(key string, size int) int {
	hash := uint64(14695981039346656037)
	for _, ch := range key {
		hash ^= uint64(ch)
		hash *= 1099511628211
	}
	return int(hash % uint64(size))
}

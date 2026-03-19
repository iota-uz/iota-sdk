// Package color provides stable semantic and fallback palettes for Lens charts.
package color

import (
	"strings"
)

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

var genericPalette = []string{
	"#2563EB",
	"#DC2626",
	"#16A34A",
	"#D97706",
	"#7C3AED",
	"#0F766E",
	"#DB2777",
	"#0891B2",
	"#CA8A04",
	"#9333EA",
	"#EA580C",
	"#4F46E5",
	"#BE123C",
	"#0284C7",
	"#15803D",
	"#7C2D12",
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

func Palette(scope string, keys []string) []string {
	colors := make([]string, 0, len(keys))
	for _, key := range keys {
		colors = append(colors, Semantic(scope, key))
	}
	return colors
}

func Sequence(scope string, size int) []string {
	if size <= 0 {
		return nil
	}
	scope = normalizeToken(scope)
	if scope == "" {
		scope = "DEFAULT"
	}
	offset := stableIndex(scope, len(genericPalette))
	colors := make([]string, size)
	for i := 0; i < size; i++ {
		colors[i] = genericPalette[(offset+i)%len(genericPalette)]
	}
	return colors
}

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
	value = strings.ReplaceAll(value, " ", "_")
	return value
}

func canonicalKey(scope, key string) string {
	switch normalizeToken(scope) {
	case ScopeProduct:
		return CanonicalProductKey(key)
	default:
		return normalizeToken(key)
	}
}

func stableIndex(key string, size int) int {
	if size <= 0 {
		return 0
	}
	hash := uint64(14695981039346656037)
	for _, ch := range key {
		hash ^= uint64(ch)
		hash *= 1099511628211
	}
	return int(hash % uint64(size))
}

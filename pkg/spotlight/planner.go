// Package spotlight provides query planning and text normalization helpers for
// Spotlight search, including lookup detection and exact-term expansion.
package spotlight

import (
	"strings"
	"unicode"
)

func planRequest(req SearchRequest) SearchRequest {
	planned := req
	planned.Query = strings.TrimSpace(req.Query)
	if planned.Mode != "" {
		return planned
	}

	if IsHowQuery(planned.Query) {
		planned.Mode = QueryModeHelp
		planned.PreferredDomains = []ResultDomain{ResultDomainKnowledge}
		return planned
	}

	if strings.HasPrefix(planned.Query, "/") {
		planned.Mode = QueryModeNavigate
		planned.PreferredDomains = []ResultDomain{ResultDomainNavigate}
		return planned
	}

	exactTerms := ExpandExactTerms(planned.Query)
	if isLookupQuery(planned.Query, exactTerms) {
		planned.Mode = QueryModeLookup
		planned.ExactTerms = exactTerms
		planned.PreferredDomains = []ResultDomain{ResultDomainLookup, ResultDomainNavigate}
		return planned
	}

	planned.Mode = QueryModeExplore
	if len(exactTerms) > 0 {
		planned.ExactTerms = exactTerms
	}
	return planned
}

func BuildSearchText(values ...string) string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	return strings.Join(out, "\n")
}

func ExpandExactTerms(values ...string) []string {
	seen := make(map[string]struct{}, len(values)*4)
	out := make([]string, 0, len(values)*4)
	for _, value := range values {
		for _, candidate := range splitExactTermCandidates(value) {
			variants := []string{
				candidate,
				strings.ToLower(candidate),
				strings.ToUpper(candidate),
			}
			if digits := onlyDigits(candidate); digits != "" && shouldAddDigitsVariant(candidate, digits) {
				variants = append(variants, digits)
			}
			if compact := onlyAlphaNumericUpper(candidate); compact != "" {
				variants = append(variants, compact)
			}
			for _, variant := range variants {
				if variant == "" {
					continue
				}
				if _, exists := seen[variant]; exists {
					continue
				}
				seen[variant] = struct{}{}
				out = append(out, variant)
			}
		}
	}
	return out
}

func splitExactTermCandidates(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}

	parts := strings.FieldsFunc(value, func(r rune) bool {
		return r == '\n' || r == '\r'
	})

	out := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		out = append(out, trimmed)
	}
	if len(out) > 0 {
		return out
	}
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	return []string{trimmed}
}

func isLookupQuery(query string, exactTerms []string) bool {
	trimmed := strings.TrimSpace(query)
	if trimmed == "" {
		return false
	}
	lower := strings.ToLower(trimmed)
	if strings.Contains(lower, "@") {
		return true
	}
	digits := onlyDigits(trimmed)
	if len(digits) >= 5 {
		return true
	}
	compact := onlyAlphaNumericUpper(trimmed)
	hasLetter := false
	hasDigit := false
	for _, r := range compact {
		if unicode.IsLetter(r) {
			hasLetter = true
		}
		if unicode.IsDigit(r) {
			hasDigit = true
		}
	}
	if hasLetter && hasDigit && len(compact) >= 4 {
		return true
	}
	return len(exactTerms) >= 2 && len(trimmed) >= 6
}

func onlyDigits(value string) string {
	var b strings.Builder
	b.Grow(len(value))
	for _, r := range value {
		if unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func onlyAlphaNumericUpper(value string) string {
	var b strings.Builder
	b.Grow(len(value))
	for _, r := range value {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(unicode.ToUpper(r))
		}
	}
	return b.String()
}

func shouldAddDigitsVariant(value, digits string) bool {
	if digits == "" {
		return false
	}
	hasLetter := false
	for _, r := range value {
		if unicode.IsLetter(r) {
			hasLetter = true
			break
		}
	}
	return !hasLetter
}

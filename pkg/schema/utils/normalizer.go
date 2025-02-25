package utils

import (
	"strings"
	"unicode"
)

// Normalizer handles SQL and schema normalization
type Normalizer struct {
	options NormalizerOptions
}

type NormalizerOptions struct {
	CaseInsensitive bool
	TrimSpaces      bool
	SortElements    bool
}

// NormalizeSQL normalizes SQL text for consistent comparison
func (n *Normalizer) NormalizeSQL(sql string) string {
	if n.options.TrimSpaces {
		sql = n.normalizeWhitespace(sql)
	}

	if n.options.CaseInsensitive {
		sql = strings.ToLower(sql)
	}

	return sql
}

func (n *Normalizer) normalizeWhitespace(sql string) string {
	// Remove extra whitespace while preserving necessary spacing
	var result strings.Builder
	var lastChar rune

	for _, char := range sql {
		if unicode.IsSpace(char) {
			if !unicode.IsSpace(lastChar) {
				result.WriteRune(' ')
			}
		} else {
			result.WriteRune(char)
		}
		lastChar = char
	}

	return strings.TrimSpace(result.String())
}

// New creates a new SQL normalizer
func New(opts NormalizerOptions) *Normalizer {
	return &Normalizer{
		options: opts,
	}
}

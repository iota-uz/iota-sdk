package codecs

import (
	"strings"
	"unicode"
)

// normalizeWhitespace collapses consecutive whitespace into single spaces.
func normalizeWhitespace(s string) string {
	var result strings.Builder
	prevWasSpace := false
	for _, r := range s {
		if unicode.IsSpace(r) {
			if !prevWasSpace {
				result.WriteRune(' ')
				prevWasSpace = true
			}
		} else {
			result.WriteRune(r)
			prevWasSpace = false
		}
	}
	return strings.TrimSpace(result.String())
}

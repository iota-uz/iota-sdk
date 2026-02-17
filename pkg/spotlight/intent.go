package spotlight

import (
	"regexp"
	"strings"
)

var howWordPattern = regexp.MustCompile(`\bhow\b`)

func IsHowQuery(query string) bool {
	trimmed := strings.TrimSpace(strings.ToLower(query))
	if trimmed == "" {
		return false
	}
	return howWordPattern.MatchString(trimmed)
}

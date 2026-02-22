package services

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

var (
	titleMarkdownPattern = regexp.MustCompile(`[*_~\[\]()]`)
	titleWhitespace      = regexp.MustCompile(`\s+`)
)

func cleanSessionTitle(title string) string {
	if !utf8.ValidString(title) {
		title = strings.ToValidUTF8(title, "�")
	}

	title = strings.TrimSpace(title)
	title = strings.Trim(title, "\"'`")
	title = titleMarkdownPattern.ReplaceAllString(title, "")
	title = titleWhitespace.ReplaceAllString(title, " ")
	return truncateSessionTitle(title)
}

func extractFallbackSessionTitle(message string) string {
	if !utf8.ValidString(message) {
		message = strings.ToValidUTF8(message, "�")
	}

	message = strings.TrimSpace(message)
	if message == "" {
		return ""
	}

	lower := strings.ToLower(message)
	prefixes := []string{
		"can you ", "could you ", "please ", "i need ", "i want ",
		"show me ", "tell me ", "give me ", "help me ",
	}
	for _, prefix := range prefixes {
		if strings.HasPrefix(lower, prefix) {
			message = message[len(prefix):]
			break
		}
	}

	runes := []rune(message)
	if len(runes) > 0 {
		runes[0] = unicode.ToUpper(runes[0])
		message = string(runes)
	}

	return truncateSessionTitle(message)
}

func isValidSessionTitle(title string) bool {
	if title == "" {
		return false
	}
	length := utf8.RuneCountInString(title)
	if length < minTitleLength || length > maxTitleLength {
		return false
	}

	lower := strings.ToLower(title)
	invalidPrefixes := []string{"here is", "here's", "the title", "title:", "as an ai"}
	for _, prefix := range invalidPrefixes {
		if strings.HasPrefix(lower, prefix) {
			return false
		}
	}
	return true
}

var titleEllipsis = []rune("...")

func truncateSessionTitle(title string) string {
	runes := []rune(title)
	if len(runes) <= maxTitleLength {
		return title
	}

	if maxTitleLength <= len(titleEllipsis) {
		return string(runes[:maxTitleLength])
	}

	return string(runes[:maxTitleLength-len(titleEllipsis)]) + string(titleEllipsis)
}

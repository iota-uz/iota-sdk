package services

import (
	"regexp"
	"strings"
)

var (
	titleMarkdownPattern = regexp.MustCompile(`[*_~\[\]()]`)
	titleWhitespace      = regexp.MustCompile(`\s+`)
)

func cleanSessionTitle(title string) string {
	title = strings.TrimSpace(title)
	title = strings.Trim(title, "\"'`")
	title = titleMarkdownPattern.ReplaceAllString(title, "")
	title = titleWhitespace.ReplaceAllString(title, " ")
	if len(title) > maxTitleLength {
		title = title[:maxTitleLength-3] + "..."
	}
	return title
}

func extractFallbackSessionTitle(message string) string {
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

	if len(message) > 0 {
		message = strings.ToUpper(string(message[0])) + message[1:]
	}
	if len(message) > maxTitleLength {
		message = message[:maxTitleLength-3] + "..."
	}
	return message
}

func isValidSessionTitle(title string) bool {
	if title == "" {
		return false
	}
	length := len(title)
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

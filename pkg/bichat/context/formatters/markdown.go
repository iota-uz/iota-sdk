package formatters

import "strings"

// EscapeMarkdownCell escapes a value for use in a markdown table cell.
// It replaces carriage returns, newlines, and pipe characters to prevent
// table formatting issues, trims whitespace, and truncates to maxWidth.
// If maxWidth <= 0, no truncation is applied.
func EscapeMarkdownCell(s string, maxWidth int) string {
	s = strings.ReplaceAll(s, "\r\n", "\\n")
	s = strings.ReplaceAll(s, "\r", "\\n")
	s = strings.ReplaceAll(s, "\n", "\\n")
	s = strings.ReplaceAll(s, "|", "\\|")
	s = strings.TrimSpace(s)
	if maxWidth > 0 && len(s) > maxWidth {
		return s[:maxWidth-3] + "..."
	}
	return s
}

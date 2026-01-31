package controllers

// isValidRedirectURL checks if the redirect URL is safe
// Returns true only if the URL is empty or a relative path starting with single "/"
func isValidRedirectURL(next string) bool {
	if next == "" {
		return true
	}
	// Must start with single "/" but not "//" (to prevent protocol-relative URLs)
	// Also reject absolute URLs
	if len(next) < 2 {
		return next == "/"
	}
	return next[0] == '/' && next[1] != '/'
}

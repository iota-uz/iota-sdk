package security

import (
	"net/url"
	"strings"
)

// IsValidRedirect validates that a redirect URL is safe to use.
// It only allows relative paths starting with "/" to prevent open redirect attacks.
//
// Valid examples:
//   - "/"
//   - "/dashboard"
//   - "/users/123"
//   - "/path/with/query?foo=bar"
//   - "" (empty string, caller should handle with default)
//
// Invalid examples:
//   - "http://evil.com"
//   - "https://evil.com"
//   - "//evil.com" (protocol-relative URL)
//   - "javascript:alert(1)"
//   - "\@evil.com" (URL encoding bypass)
func IsValidRedirect(redirectURL string) bool {
	// Empty string is technically valid - caller should handle default
	if redirectURL == "" {
		return true
	}

	// Trim whitespace to prevent bypass attempts
	redirectURL = strings.TrimSpace(redirectURL)

	// Must start with "/" to be a relative path
	if !strings.HasPrefix(redirectURL, "/") {
		return false
	}

	// Prevent protocol-relative URLs and backslash bypass (//evil.com or /\evil.com)
	if strings.HasPrefix(redirectURL, "//") || (len(redirectURL) > 1 && redirectURL[0] == '/' && redirectURL[1] == '\\') {
		return false
	}

	// Parse to detect schemes and other attacks
	parsed, err := url.Parse(redirectURL)
	if err != nil {
		return false
	}

	// Reject if scheme is present (http, https, javascript, data, etc.)
	if parsed.Scheme != "" {
		return false
	}

	// Reject if host is present (absolute URL)
	if parsed.Host != "" {
		return false
	}

	// Additional check: path must start with "/"
	if parsed.Path != "" && !strings.HasPrefix(parsed.Path, "/") {
		return false
	}

	return true
}

// GetValidatedRedirect extracts and validates the redirect URL from a query parameter.
// Returns the validated URL or "/" as a safe fallback.
func GetValidatedRedirect(redirectURL string) string {
	if IsValidRedirect(redirectURL) && redirectURL != "" {
		return redirectURL
	}
	return "/"
}

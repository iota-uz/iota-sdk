package security

import (
	"testing"
)

func TestIsValidRedirect(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		// Valid cases
		{
			name:     "root path",
			input:    "/",
			expected: true,
		},
		{
			name:     "simple path",
			input:    "/dashboard",
			expected: true,
		},
		{
			name:     "nested path",
			input:    "/users/123",
			expected: true,
		},
		{
			name:     "path with query",
			input:    "/path?foo=bar",
			expected: true,
		},
		{
			name:     "path with multiple query params",
			input:    "/path?foo=bar&baz=qux",
			expected: true,
		},
		{
			name:     "path with fragment",
			input:    "/path#section",
			expected: true,
		},
		{
			name:     "path with query and fragment",
			input:    "/path?foo=bar#section",
			expected: true,
		},
		{
			name:     "empty string",
			input:    "",
			expected: true,
		},
		{
			name:     "deep nested path",
			input:    "/module/submodule/action/123",
			expected: true,
		},

		// Invalid cases - absolute URLs
		{
			name:     "http absolute URL",
			input:    "http://evil.com",
			expected: false,
		},
		{
			name:     "https absolute URL",
			input:    "https://evil.com",
			expected: false,
		},
		{
			name:     "http with path",
			input:    "http://evil.com/path",
			expected: false,
		},
		{
			name:     "https with path",
			input:    "https://evil.com/path",
			expected: false,
		},

		// Invalid cases - protocol-relative URLs
		{
			name:     "protocol-relative URL",
			input:    "//evil.com",
			expected: false,
		},
		{
			name:     "protocol-relative with path",
			input:    "//evil.com/path",
			expected: false,
		},

		// Invalid cases - javascript and data URIs
		{
			name:     "javascript URI",
			input:    "javascript:alert(1)",
			expected: false,
		},
		{
			name:     "data URI",
			input:    "data:text/html,<script>alert(1)</script>",
			expected: false,
		},

		// Invalid cases - bypass attempts
		{
			name:     "backslash bypass attempt",
			input:    "\\@evil.com",
			expected: false,
		},
		{
			name:     "whitespace prefix",
			input:    " /path",
			expected: true, // trimmed
		},
		{
			name:     "whitespace suffix",
			input:    "/path ",
			expected: true, // trimmed
		},
		{
			name:     "relative path without leading slash",
			input:    "path",
			expected: false,
		},
		{
			name:     "parent directory traversal",
			input:    "/../etc/passwd",
			expected: true, // this is still a valid relative path, but application should handle traversal
		},

		// Invalid cases - URL encoding attacks
		{
			name:     "URL encoded http",
			input:    "%68%74%74%70%3a%2f%2f%65%76%69%6c%2e%63%6f%6d",
			expected: false,
		},
		{
			name:     "URL encoded protocol-relative",
			input:    "%2f%2fevil.com",
			expected: false,
		},

		// Edge cases
		{
			name:     "single dot",
			input:    "/.",
			expected: true,
		},
		{
			name:     "double dot",
			input:    "/..",
			expected: true,
		},
		{
			name:     "only whitespace",
			input:    "   ",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := IsValidRedirect(tt.input)
			if result != tt.expected {
				t.Errorf("IsValidRedirect(%q) = %v, expected %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestGetValidatedRedirect(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "valid path returns same",
			input:    "/dashboard",
			expected: "/dashboard",
		},
		{
			name:     "invalid URL returns fallback",
			input:    "http://evil.com",
			expected: "/",
		},
		{
			name:     "empty string returns fallback",
			input:    "",
			expected: "/",
		},
		{
			name:     "protocol-relative returns fallback",
			input:    "//evil.com",
			expected: "/",
		},
		{
			name:     "javascript URI returns fallback",
			input:    "javascript:alert(1)",
			expected: "/",
		},
		{
			name:     "valid path with query",
			input:    "/path?foo=bar",
			expected: "/path?foo=bar",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := GetValidatedRedirect(tt.input)
			if result != tt.expected {
				t.Errorf("GetValidatedRedirect(%q) = %q, expected %q", tt.input, result, tt.expected)
			}
		})
	}
}

// Benchmark tests
func BenchmarkIsValidRedirect(b *testing.B) {
	testCases := []string{
		"/",
		"/dashboard",
		"/users/123",
		"http://evil.com",
		"//evil.com",
		"javascript:alert(1)",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, tc := range testCases {
			IsValidRedirect(tc)
		}
	}
}

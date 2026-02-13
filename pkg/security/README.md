# Security Package

This package provides security utilities for the IOTA SDK, focusing on preventing common web vulnerabilities.

## Open Redirect Prevention

The `IsValidRedirect()` and `GetValidatedRedirect()` functions prevent open redirect attacks by validating redirect URLs.

### Usage

```go
import "github.com/iota-uz/iota-sdk/pkg/security"

// In a controller
func (c *LoginController) Post(w http.ResponseWriter, r *http.Request) {
    // Get the redirect URL from query parameter
    redirectURL := r.URL.Query().Get("next")

    // Validate and get safe redirect
    safeRedirect := security.GetValidatedRedirect(redirectURL)

    // Use the validated redirect
    http.Redirect(w, r, safeRedirect, http.StatusFound)
}
```

### Valid Redirect URLs

Only relative paths starting with `/` are considered valid:

- ✅ `/` - root path
- ✅ `/dashboard` - simple path
- ✅ `/users/123` - nested path
- ✅ `/path?foo=bar` - path with query parameters
- ✅ `/path#section` - path with fragment
- ✅ `` (empty string) - treated as valid, but `GetValidatedRedirect()` returns `/`

### Invalid Redirect URLs

All absolute URLs and protocol-relative URLs are rejected:

- ❌ `http://evil.com` - absolute URL with scheme
- ❌ `https://evil.com` - absolute URL with scheme
- ❌ `//evil.com` - protocol-relative URL
- ❌ `javascript:alert(1)` - javascript URI
- ❌ `data:text/html,<script>alert(1)</script>` - data URI
- ❌ `\@evil.com` - backslash bypass attempt
- ❌ `path` - relative path without leading slash

### Security Considerations

1. **Always validate user-provided redirect URLs** - Never trust the `next`, `redirect`, or similar query parameters
2. **Use `GetValidatedRedirect()` for convenience** - It returns a safe fallback (`/`) when validation fails
3. **URL-encode when passing in query strings** - Use `url.QueryEscape()` when including the redirect in another URL
4. **Consider path traversal separately** - While `/../etc/passwd` is technically a valid relative path, your application should handle directory traversal at the filesystem level

### Example: Login Flow with Next Parameter

```go
// When redirecting to login with next parameter
nextURL := security.GetValidatedRedirect(r.URL.Query().Get("next"))
http.Redirect(w, r, fmt.Sprintf("/login?next=%s", url.QueryEscape(nextURL)), http.StatusFound)

// After successful login
redirectURL := security.GetValidatedRedirect(r.URL.Query().Get("next"))
http.Redirect(w, r, redirectURL, http.StatusFound)
```

## Testing

The package includes comprehensive tests covering:
- Valid relative paths
- Invalid absolute URLs
- Protocol-relative URL attacks
- JavaScript and data URI attacks
- URL encoding bypass attempts
- Edge cases (whitespace, empty strings, etc.)

Run tests with:
```bash
go test -v ./pkg/security/
```

## Performance

The validation functions are optimized for minimal overhead:
- String operations are minimized
- URL parsing is only done once per validation
- Benchmark tests ensure consistent performance

package middleware

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	csrf "filippo.io/csrf/gorilla"
	"github.com/gorilla/mux"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/appconfig"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/httpconfig"
)

// CSRFOption configures the CSRF middleware.
type CSRFOption func(*csrfConfig)

type csrfConfig struct {
	exemptPrefixes []string
	trustedOrigins []string
	errorHandler   http.Handler
}

// CSRFExemptPrefixes sets URL path prefixes that are exempt from CSRF validation.
func CSRFExemptPrefixes(prefixes ...string) CSRFOption {
	return func(c *csrfConfig) {
		c.exemptPrefixes = prefixes
	}
}

// CSRFTrustedOrigins sets origins that are allowed to make cross-origin requests.
// gorilla/csrf checks the Origin header on state-changing requests; without this,
// requests from untrusted origins are rejected.
// Values should be scheme-qualified origins (e.g., "http://localhost:3200", "https://erp.eai.uz").
func CSRFTrustedOrigins(origins ...string) CSRFOption {
	return func(c *csrfConfig) {
		c.trustedOrigins = origins
	}
}

// CSRFErrorHandler sets a custom error handler for CSRF failures.
func CSRFErrorHandler(h http.Handler) CSRFOption {
	return func(c *csrfConfig) {
		c.errorHandler = h
	}
}

// CSRFFromConfig creates CSRF middleware pre-configured from httpconfig.Config and appconfig.Config.
// It reads AllowedOrigins and computes Origin from config.
// AllowedOrigins (full URLs) are normalized to scheme-qualified origins for CSRF trust.
// Additional opts (e.g. CSRFExemptPrefixes) are applied on top.
func CSRFFromConfig(cfg *httpconfig.Config, appCfg *appconfig.Config, opts ...CSRFOption) mux.MiddlewareFunc {
	baseOpts := []CSRFOption{}

	defaultScheme := appCfg.Scheme()
	var trustedOrigins []string
	for _, origin := range cfg.AllowedOrigins {
		if normalized, ok := normalizeTrustedOrigin(origin, defaultScheme); ok {
			trustedOrigins = append(trustedOrigins, normalized)
		}
	}
	if normalized, ok := normalizeTrustedOrigin(cfg.Origin(appCfg), defaultScheme); ok {
		trustedOrigins = append(trustedOrigins, normalized)
	}

	if len(trustedOrigins) > 0 {
		baseOpts = append(baseOpts, CSRFTrustedOrigins(trustedOrigins...))
	}

	baseOpts = append(baseOpts, opts...)
	return CSRF(nil, baseOpts...)
}

// CSRF returns middleware that provides CSRF protection using gorilla/csrf.
// authKey must be a 32-byte key for token generation/validation.
//
// The middleware:
//   - Generates CSRF tokens on safe methods (GET, HEAD, OPTIONS, TRACE)
//   - Validates tokens on state-changing methods (POST, PUT, PATCH, DELETE)
//   - Enforces Fetch Metadata and Origin checks to validate requests.
//   - Supports path-based exemptions for API endpoints using Bearer tokens.
func CSRF(authKey []byte, opts ...CSRFOption) mux.MiddlewareFunc {
	cfg := &csrfConfig{}
	for _, opt := range opts {
		opt(cfg)
	}

	csrfOpts := []csrf.Option{}
	if len(cfg.trustedOrigins) > 0 {
		csrfOpts = append(csrfOpts, csrf.TrustedOrigins(cfg.trustedOrigins))
	}
	if cfg.errorHandler != nil {
		csrfOpts = append(csrfOpts, csrf.ErrorHandler(cfg.errorHandler))
	}

	protect := csrf.Protect(authKey, csrfOpts...)

	return func(next http.Handler) http.Handler {
		protected := protect(next)

		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			for _, prefix := range cfg.exemptPrefixes {
				if strings.HasPrefix(r.URL.Path, prefix) {
					next.ServeHTTP(w, r)
					return
				}
			}
			protected.ServeHTTP(w, r)
		})
	}
}

func normalizeTrustedOrigin(origin, defaultScheme string) (string, bool) {
	parsed, err := url.Parse(origin)
	if err != nil {
		return "", false
	}

	if parsed.Host == "" {
		parsed, err = url.Parse(fmt.Sprintf("//%s", origin))
		if err != nil {
			return "", false
		}
		if parsed.Host == "" {
			return "", false
		}
	}

	scheme := parsed.Scheme
	if scheme == "" {
		scheme = defaultScheme
	}

	return fmt.Sprintf("%s://%s", scheme, parsed.Host), true
}

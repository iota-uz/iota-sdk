package middleware

import (
	"context"
	"net/http"
	"net/url"
	"strings"

	csrf "filippo.io/csrf/gorilla"
	"github.com/gorilla/mux"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
)

// CSRFOption configures the CSRF middleware.
type CSRFOption func(*csrfConfig)

type csrfConfig struct {
	secure         bool
	domain         string
	path           string
	cookieName     string
	exemptPrefixes []string
	trustedOrigins []string
	errorHandler   http.Handler
}

// CSRFSecure sets the Secure flag on the CSRF cookie.
func CSRFSecure(secure bool) CSRFOption {
	return func(c *csrfConfig) {
		c.secure = secure
	}
}

// CSRFDomain sets the Domain on the CSRF cookie.
func CSRFDomain(domain string) CSRFOption {
	return func(c *csrfConfig) {
		c.domain = domain
	}
}

// CSRFPath sets the Path on the CSRF cookie.
func CSRFPath(path string) CSRFOption {
	return func(c *csrfConfig) {
		c.path = path
	}
}

// CSRFCookieName sets the name of the CSRF cookie.
func CSRFCookieName(name string) CSRFOption {
	return func(c *csrfConfig) {
		c.cookieName = name
	}
}

// CSRFExemptPrefixes sets URL path prefixes that are exempt from CSRF validation.
func CSRFExemptPrefixes(prefixes ...string) CSRFOption {
	return func(c *csrfConfig) {
		c.exemptPrefixes = prefixes
	}
}

// CSRFTrustedOrigins sets origins that are allowed to make cross-origin requests.
// gorilla/csrf checks the Origin header on POST requests; without this,
// requests from http://localhost:3200 (or any non-HTTPS origin) are rejected.
// Values should be host[:port] (e.g., "localhost:3200", "erp.eai.uz").
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

// CSRFFromConfig creates CSRF middleware pre-configured from Configuration.
// It reads AllowedOrigins, Domain, Origin, and GoAppEnvironment from config.
// AllowedOrigins (full URLs) are parsed to extract host[:port] for CSRF trust.
// Additional opts (e.g. CSRFExemptPrefixes) are applied on top.
func CSRFFromConfig(cfg *configuration.Configuration, opts ...CSRFOption) mux.MiddlewareFunc {
	baseOpts := []CSRFOption{
		CSRFSecure(cfg.GoAppEnvironment == configuration.Production),
		CSRFDomain(cfg.Domain),
	}

	// Collect trusted origins from AllowedOrigins (full URLs → host[:port])
	// and the app's own Origin.
	var trustedOrigins []string
	for _, origin := range cfg.AllowedOrigins {
		if parsed, err := url.Parse(origin); err == nil && parsed.Host != "" {
			trustedOrigins = append(trustedOrigins, parsed.Host)
		}
	}
	if parsed, err := url.Parse(cfg.Origin); err == nil && parsed.Host != "" {
		trustedOrigins = append(trustedOrigins, parsed.Host)
	}
	if len(trustedOrigins) > 0 {
		baseOpts = append(baseOpts, CSRFTrustedOrigins(trustedOrigins...))
	}

	baseOpts = append(baseOpts, opts...)
	return CSRF(nil, baseOpts...)
}

// csrfTokenKey is the context key used to pass the CSRF token to templates.
// This must match the key used in composables.CSRFTokenField() and CSRFMetaTag().
const csrfTokenKey = "gorilla.csrf.Token"

// CSRF returns middleware that provides CSRF protection using gorilla/csrf.
// authKey must be a 32-byte key for token generation/validation.
//
// The middleware:
//   - Generates CSRF tokens on safe methods (GET, HEAD, OPTIONS, TRACE)
//   - Validates tokens on state-changing methods (POST, PUT, PATCH, DELETE)
//   - Injects the token into context for use by CSRFTokenField() and CSRFMetaTag()
//   - Supports path-based exemptions for API endpoints using Bearer tokens
func CSRF(authKey []byte, opts ...CSRFOption) mux.MiddlewareFunc {
	cfg := &csrfConfig{
		secure:     true,
		path:       "/",
		cookieName: "_csrf",
	}
	for _, opt := range opts {
		opt(cfg)
	}

	csrfOpts := []csrf.Option{
		csrf.Secure(cfg.secure),
		csrf.Path(cfg.path),
		csrf.SameSite(csrf.SameSiteLaxMode),
		csrf.HttpOnly(true),
		csrf.CookieName(cfg.cookieName),
	}
	if cfg.domain != "" {
		csrfOpts = append(csrfOpts, csrf.Domain(cfg.domain))
	}
	if len(cfg.trustedOrigins) > 0 {
		csrfOpts = append(csrfOpts, csrf.TrustedOrigins(cfg.trustedOrigins))
	}
	if cfg.errorHandler != nil {
		csrfOpts = append(csrfOpts, csrf.ErrorHandler(cfg.errorHandler))
	}

	protect := csrf.Protect(authKey, csrfOpts...)

	return func(next http.Handler) http.Handler {
		// Wrap next handler to inject token into templ-compatible context key.
		// gorilla/csrf stores the token with an unexported key; we bridge it
		// to the well-known string key that CSRFTokenField()/CSRFMetaTag() read.
		tokenInjector := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := csrf.Token(r)
			ctx := context.WithValue(r.Context(), csrfTokenKey, token)
			next.ServeHTTP(w, r.WithContext(ctx))
		})

		protected := protect(tokenInjector)

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

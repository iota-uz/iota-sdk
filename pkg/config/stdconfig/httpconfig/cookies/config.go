// Package cookies provides typed configuration for cookie key names.
// It is a stdconfig package intended to be registered via config.Register[cookies.Config].
package cookies

// Config holds cookie key name settings.
//
// Env prefix: "http.cookies" (e.g. SID_COOKIE_KEY → http.cookies.sid).
type Config struct {
	// SID is the session-ID cookie key.
	SID string `koanf:"sid" default:"sid"`
	// OAuthState is the OAuth state cookie key.
	OAuthState string `koanf:"oauthstate" default:"oauthState"`
	// Domain optionally enables a shared-domain cookie. Leave empty (the
	// default) for a host-only cookie so the same server can be reached through
	// localhost, a LAN hostname, or another development alias without browsers
	// rejecting the cookie. Maps from HTTP_COOKIES_DOMAIN.
	Domain string `koanf:"domain"`
}

// ConfigPrefix returns the koanf prefix for cookies ("http.cookies").
func (Config) ConfigPrefix() string { return "http.cookies" }

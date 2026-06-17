// Package headers provides typed configuration for HTTP header name settings.
// It is a stdconfig package intended to be registered via config.Register[headers.Config].
package headers

// Config holds HTTP header name settings.
//
// Env prefix: "http.headers" (e.g. REQUEST_ID_HEADER → http.headers.requestid).
type Config struct {
	// RequestID is the header name SDK looks for to propagate request IDs.
	RequestID string `koanf:"requestid" default:"X-Request-ID"`
	// RealIP is the header name SDK uses to extract the real client IP.
	RealIP string `koanf:"realip" default:"X-Real-IP"`
}

// ConfigPrefix returns the koanf prefix for headers ("http.headers").
func (Config) ConfigPrefix() string { return "http.headers" }

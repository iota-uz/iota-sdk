package httpconfig

import (
	"time"

	"github.com/iota-uz/iota-sdk/pkg/configuration"
)

// FromLegacy constructs a Config from the legacy *configuration.Configuration.
// SocketAddress is intentionally not stored — call Config.SocketAddress() to derive it.
func FromLegacy(c *configuration.Configuration) Config {
	sessionDur := c.SessionDuration
	if sessionDur == 0 {
		sessionDur = 720 * time.Hour
	}

	return Config{
		Port:           c.ServerPort,
		Domain:         c.Domain,
		Origin:         c.Origin,
		AllowedOrigins: c.AllowedOrigins,
		Environment:    c.GoAppEnvironment,
		Headers: HeadersConfig{
			RequestID: c.RequestIDHeader,
			RealIP:    c.RealIPHeader,
		},
		Cookies: CookiesConfig{
			SID:        c.SidCookieKey,
			OAuthState: c.OauthStateCookieKey,
		},
		Session: SessionConfig{
			Duration: sessionDur,
		},
		Pagination: PaginationConfig{
			PageSize:    c.PageSize,
			MaxPageSize: c.MaxPageSize,
		},
	}
}

package services

import (
	"testing"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/appconfig"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/httpconfig/cookies"
	"github.com/stretchr/testify/require"
)

func TestAuthFlowSessionCookie_DomainAndSecurity(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		domain      string
		environment string
		wantDomain  string
		wantSecure  bool
	}{
		{
			name:        "host only by default",
			environment: "development",
		},
		{
			name:        "explicit shared domain",
			domain:      ".example.com",
			environment: "production",
			wantDomain:  ".example.com",
			wantSecure:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			service := &AuthFlowService{
				cookiesCfg: &cookies.Config{SID: "granite_sid", Domain: tt.domain},
				appCfg:     &appconfig.Config{Environment: tt.environment},
			}
			cookie := service.sessionCookie("token", time.Now().Add(time.Hour))

			require.Equal(t, "granite_sid", cookie.Name)
			require.Equal(t, tt.wantDomain, cookie.Domain)
			require.Equal(t, tt.wantSecure, cookie.Secure)
		})
	}
}

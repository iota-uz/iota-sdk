package services

import (
	"testing"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/appconfig"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/httpconfig/cookies"
	"github.com/stretchr/testify/require"
)

func TestAuthFlowSessionCookieIsHostOnlyByDefault(t *testing.T) {
	t.Parallel()

	service := &AuthFlowService{
		cookiesCfg: &cookies.Config{SID: "granite_sid"},
		appCfg:     &appconfig.Config{Environment: "development"},
	}
	cookie := service.sessionCookie("token", time.Now().Add(time.Hour))

	require.Equal(t, "granite_sid", cookie.Name)
	require.Empty(t, cookie.Domain)
	require.False(t, cookie.Secure)
}

func TestAuthFlowSessionCookieSupportsExplicitSharedDomain(t *testing.T) {
	t.Parallel()

	service := &AuthFlowService{
		cookiesCfg: &cookies.Config{SID: "granite_sid", Domain: ".example.com"},
		appCfg:     &appconfig.Config{Environment: "production"},
	}
	cookie := service.sessionCookie("token", time.Now().Add(time.Hour))

	require.Equal(t, ".example.com", cookie.Domain)
	require.True(t, cookie.Secure)
}

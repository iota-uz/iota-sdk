package session

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestCreateDTOUsesExplicitExpiryForShortLivedSessions(t *testing.T) {
	t.Parallel()
	expiresAt := time.Date(2026, time.July, 31, 12, 5, 0, 0, time.UTC)
	created := (&CreateDTO{
		Token: "background-token", UserID: 42, TenantID: uuid.New(),
		IP: "127.0.0.1", UserAgent: "background", ExpiresAt: expiresAt,
	}).ToEntity()
	require.Equal(t, expiresAt, created.ExpiresAt())
	require.Equal(t, StatusActive, created.Status())
}

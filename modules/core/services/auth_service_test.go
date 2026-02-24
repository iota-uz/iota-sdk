package services_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/session"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/itf"
)

func persistAuthTestUser(t *testing.T, f *itf.TestEnvironment) {
	t.Helper()

	tx, err := composables.UseTx(f.Ctx)
	require.NoError(t, err)

	u := f.User
	var email, phone string
	if u.Email() != nil {
		email = u.Email().Value()
	}
	if u.Phone() != nil {
		phone = u.Phone().Value()
	}

	query := `
		INSERT INTO users (id, type, first_name, last_name, email, phone, password, tenant_id, ui_language, created_at, updated_at)
		VALUES ($1, 'system', $2, $3, $4, $5, $6, $7, 'en', NOW(), NOW())
		ON CONFLICT (id) DO NOTHING
	`
	_, err = tx.Exec(
		f.Ctx,
		query,
		u.ID(),
		u.FirstName(),
		u.LastName(),
		email,
		phone,
		u.Password(),
		f.Tenant.ID,
	)
	require.NoError(t, err)
}

func TestAuthService_Authorize(t *testing.T) {
	t.Parallel()

	f := setupTestWithPermissions(t)
	require.NotNil(t, f.User)
	persistAuthTestUser(t, f)
	authService := itf.GetService[services.AuthService](f)
	sessionService := itf.GetService[services.SessionService](f)
	sessionRepo := persistence.NewSessionRepository()

	t.Run("Authorize_ExpiredSession_ReturnsErrSessionExpired_AndDeletesSession", func(t *testing.T) {
		token := "expired-session-token"
		expiredSession := session.New(
			token,
			f.User.ID(),
			f.Tenant.ID,
			"127.0.0.1",
			"test-agent",
			session.WithExpiresAt(time.Now().Add(-1*time.Minute)),
		)
		require.NoError(t, sessionRepo.Create(f.Ctx, expiredSession))

		_, err := authService.Authorize(f.Ctx, token)
		require.ErrorIs(t, err, services.ErrSessionExpired)

		_, err = sessionService.GetByToken(f.Ctx, token)
		require.ErrorIs(t, err, persistence.ErrSessionNotFound)
	})

	t.Run("Authorize_ActiveSession_ReturnsSession", func(t *testing.T) {
		token := "active-session-token"
		activeSession := session.New(
			token,
			f.User.ID(),
			f.Tenant.ID,
			"127.0.0.1",
			"test-agent",
			session.WithExpiresAt(time.Now().Add(10*time.Minute)),
		)
		require.NoError(t, sessionRepo.Create(f.Ctx, activeSession))

		sess, err := authService.Authorize(f.Ctx, token)
		require.NoError(t, err)
		assert.Equal(t, token, sess.Token())
	})

	t.Run("Authorize_MissingSession_ReturnsError", func(t *testing.T) {
		_, err := authService.Authorize(f.Ctx, "missing-session-token")
		require.ErrorIs(t, err, persistence.ErrSessionNotFound)
	})
}

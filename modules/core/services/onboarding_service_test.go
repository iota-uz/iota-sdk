package services_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/session"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/core/permissions"
	"github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/itf"
)

func createOnboardingTestUser(t *testing.T, ctx context.Context, build func(user.User) user.User) user.User {
	t.Helper()
	tenantID, err := composables.UseTenantID(ctx)
	require.NoError(t, err)
	email, err := internet.NewEmail("onboarding-" + uuid.NewString() + "@example.test")
	require.NoError(t, err)
	u, err := user.New("", "", email, user.UILanguageEN, user.WithTenantID(tenantID)).SetPassword("Permanent123!")
	require.NoError(t, err)
	if build != nil {
		u = build(u)
	}
	created, err := persistence.NewUserRepository(persistence.NewUploadRepository()).Create(ctx, u)
	require.NoError(t, err)
	return created
}

// persistAdmin stores an administrator whose permissions the privilege policy
// reads back from the database.
func persistAdmin(t *testing.T, f *itf.TestEnvironment, perms ...permission.Permission) context.Context {
	t.Helper()
	permissionRepository := persistence.NewPermissionRepository()
	for _, perm := range perms {
		require.NoError(t, permissionRepository.Save(f.Ctx, perm))
	}
	email, err := internet.NewEmail("admin-" + uuid.NewString() + "@example.test")
	require.NoError(t, err)
	admin, err := persistence.NewUserRepository(persistence.NewUploadRepository()).Create(f.Ctx, user.New(
		"Onboarding", "Admin", email, user.UILanguageEN,
		user.WithTenantID(f.TenantID()),
		user.WithPermissions(perms),
	))
	require.NoError(t, err)
	return composables.WithUser(f.Ctx, admin)
}

func issued(t *testing.T, password string, expiresAt time.Time) func(user.User) user.User {
	t.Helper()
	return func(u user.User) user.User {
		out, err := u.IssueTemporaryPassword(password, expiresAt)
		require.NoError(t, err)
		return out
	}
}

func onboardingSessionToken(t *testing.T, ctx context.Context, sessionService *services.SessionService, u user.User) string {
	t.Helper()
	token := "onboarding-" + uuid.NewString()
	require.NoError(t, sessionService.Create(ctx, &session.CreateDTO{
		Token: token, UserID: u.ID(), TenantID: u.TenantID(),
		Status: session.StatusPendingOnboarding, ExpiresAt: time.Now().Add(time.Hour),
	}))
	return token
}

func createSessions(t *testing.T, ctx context.Context, sessionService *services.SessionService, u user.User, tokens ...string) {
	t.Helper()
	for _, token := range tokens {
		require.NoError(t, sessionService.Create(ctx, &session.CreateDTO{Token: token + uuid.NewString(), UserID: u.ID(), TenantID: u.TenantID()}))
	}
}

func TestUserService_IssueTemporaryPassword(t *testing.T) {
	// Falsely green if the admin could still set a permanent password or old sessions survive.
	t.Parallel()
	f := setupTest(t)
	ctx := persistAdmin(t, f, permissions.UserRead, permissions.UserUpdate)
	userService := itf.GetService[services.UserService](f)
	sessionService := itf.GetService[services.SessionService](f)
	userRepo := persistence.NewUserRepository(persistence.NewUploadRepository())

	target := createOnboardingTestUser(t, ctx, nil)
	createSessions(t, ctx, sessionService, target, "active-a-", "active-b-")

	result, err := userService.IssueTemporaryPassword(ctx, target.ID(), "")
	require.NoError(t, err)
	require.NoError(t, user.ValidatePassword(result.Password), "generated password must satisfy the policy")
	assert.WithinDuration(t, time.Now().Add(user.TemporaryPasswordTTL), result.ExpiresAt, time.Minute)

	stored, err := userRepo.GetByID(f.Ctx, target.ID())
	require.NoError(t, err)
	assert.Equal(t, user.StatusPendingOnboarding, stored.Status())
	assert.True(t, stored.CheckPassword(result.Password))
	assert.False(t, stored.CheckPassword("Permanent123!"))
	assert.WithinDuration(t, result.ExpiresAt, stored.PasswordExpiresAt(), time.Second)
	sessions, err := sessionService.GetByUserID(f.Ctx, target.ID())
	require.NoError(t, err)
	assert.Empty(t, sessions)

	second, err := userService.IssueTemporaryPassword(ctx, target.ID(), "Chosen-temp-1")
	require.NoError(t, err)
	assert.Equal(t, "Chosen-temp-1", second.Password)
	reissued, err := userRepo.GetByID(f.Ctx, target.ID())
	require.NoError(t, err)
	assert.False(t, reissued.CheckPassword(result.Password), "a reissue invalidates the previous temporary password")
	assert.True(t, reissued.CheckPassword("Chosen-temp-1"))
}

func TestUserService_IssueTemporaryPassword_RequiresUserUpdate(t *testing.T) {
	t.Parallel()
	f := setupTestWithPermissions(t, permissions.UserRead)
	userService := itf.GetService[services.UserService](f)
	target := createOnboardingTestUser(t, f.Ctx, nil)

	_, err := userService.IssueTemporaryPassword(f.Ctx, target.ID(), "")
	require.ErrorIs(t, err, composables.ErrForbidden)
}

func TestUserService_CompleteOnboarding(t *testing.T) {
	// Falsely green if completion leaves a partially activated profile or keeps the temporary password usable.
	t.Parallel()
	f := setupTestWithPermissions(t)
	userService := itf.GetService[services.UserService](f)
	sessionService := itf.GetService[services.SessionService](f)
	userRepo := persistence.NewUserRepository(persistence.NewUploadRepository())
	input := services.OnboardingInput{
		FirstName: "Nodira",
		LastName:  "Karimova",
		Language:  user.UILanguageRU,
		Password:  "my own passphrase",
	}

	t.Run("activates the profile and revokes sessions", func(t *testing.T) {
		pending := createOnboardingTestUser(t, f.Ctx, issued(t, "Temporary-1", time.Now().Add(time.Hour)))
		createSessions(t, f.Ctx, sessionService, pending, "other-")
		token := onboardingSessionToken(t, f.Ctx, sessionService, pending)

		_, err := userService.CompleteOnboarding(f.Ctx, token, input)
		require.NoError(t, err)

		stored, err := userRepo.GetByID(f.Ctx, pending.ID())
		require.NoError(t, err)
		assert.Equal(t, user.StatusActive, stored.Status())
		assert.False(t, stored.HasTemporaryPassword())
		assert.Equal(t, "Nodira", stored.FirstName())
		assert.Equal(t, "Karimova", stored.LastName())
		assert.Equal(t, user.UILanguageRU, stored.UILanguage())
		assert.True(t, stored.CheckPassword("my own passphrase"))
		assert.False(t, stored.CheckPassword("Temporary-1"))
		sessions, err := sessionService.GetByUserID(f.Ctx, pending.ID())
		require.NoError(t, err)
		assert.Empty(t, sessions)

		_, err = userService.CompleteOnboarding(f.Ctx, token, input)
		require.ErrorIs(t, err, services.ErrOnboardingSessionInvalid, "a repeated submission must not change the account")
	})

	rejected := []struct {
		name  string
		build func(user.User) user.User
		input services.OnboardingInput
		err   error
	}{
		{
			name:  "reused temporary password",
			build: issued(t, "Temporary-1", time.Now().Add(time.Hour)),
			input: services.OnboardingInput{FirstName: "A", LastName: "B", Language: user.UILanguageEN, Password: "Temporary-1"},
			err:   user.ErrPasswordReusesTemporary,
		},
		{
			name:  "expired temporary password",
			build: issued(t, "Temporary-1", time.Now().Add(-time.Minute)),
			input: input,
			err:   user.ErrTemporaryPasswordExpired,
		},
		{
			name: "blocked user",
			build: func(u user.User) user.User {
				blocker, err := composables.UseUser(persistAdmin(t, f))
				require.NoError(t, err)
				return issued(t, "Temporary-1", time.Now().Add(time.Hour))(u).Block("left the company", blocker.ID(), blocker.TenantID())
			},
			input: input,
			err:   services.ErrUserBlocked,
		},
		{
			name:  "active user",
			build: nil,
			input: input,
			err:   user.ErrNotPendingOnboarding,
		},
		{
			name:  "session revoked by a reissued temporary password",
			build: issued(t, "Temporary-1", time.Now().Add(time.Hour)),
			input: input,
			err:   services.ErrOnboardingSessionInvalid,
		},
	}
	for _, tc := range rejected {
		t.Run(tc.name, func(t *testing.T) {
			target := createOnboardingTestUser(t, f.Ctx, tc.build)
			token := onboardingSessionToken(t, f.Ctx, sessionService, target)
			if tc.err == services.ErrOnboardingSessionInvalid {
				_, err := sessionService.DeleteByUserID(f.Ctx, target.ID())
				require.NoError(t, err)
			}

			_, err := userService.CompleteOnboarding(f.Ctx, token, tc.input)
			require.ErrorIs(t, err, tc.err)

			stored, err := userRepo.GetByID(f.Ctx, target.ID())
			require.NoError(t, err)
			assert.Equal(t, target.Status(), stored.Status(), "a rejected completion must not change the account")
			assert.Equal(t, target.Password(), stored.Password())
			assert.Empty(t, stored.FirstName())
		})
	}
}

func TestAuthService_TemporaryPasswordLogin(t *testing.T) {
	// Falsely green if a pending user can open a full session or unlimited guesses are accepted.
	t.Parallel()
	f := setupTest(t)
	authService := itf.GetService[services.AuthService](f)
	userRepo := persistence.NewUserRepository(persistence.NewUploadRepository())

	t.Run("pending user gets no full session", func(t *testing.T) {
		pending := createOnboardingTestUser(t, f.Ctx, issued(t, "Temporary-1", time.Now().Add(time.Hour)))

		verified, err := authService.VerifyPassword(f.Ctx, pending.Email().Value(), "Temporary-1")
		require.NoError(t, err)
		_, err = authService.CreateSession(f.Ctx, verified)
		require.ErrorIs(t, err, services.ErrOnboardingRequired)
		_, _, err = authService.Authenticate(f.Ctx, pending.Email().Value(), "Temporary-1")
		require.ErrorIs(t, err, services.ErrOnboardingRequired)

		sess, err := authService.CreateOnboardingSession(f.Ctx, verified)
		require.NoError(t, err)
		assert.True(t, sess.IsPendingOnboarding())
		assert.False(t, sess.IsActive())
		assert.False(t, sess.ExpiresAt().After(verified.PasswordExpiresAt()))
	})

	t.Run("expired temporary password", func(t *testing.T) {
		expired := createOnboardingTestUser(t, f.Ctx, issued(t, "Temporary-1", time.Now().Add(-time.Minute)))

		_, err := authService.VerifyPassword(f.Ctx, expired.Email().Value(), "Temporary-1")
		require.ErrorIs(t, err, user.ErrTemporaryPasswordExpired)
	})

	t.Run("attempts are limited", func(t *testing.T) {
		pending := createOnboardingTestUser(t, f.Ctx, issued(t, "Temporary-1", time.Now().Add(time.Hour)))

		for i := 0; i < user.MaxTemporaryPasswordAttempts; i++ {
			_, err := authService.VerifyPassword(f.Ctx, pending.Email().Value(), "wrong-guess")
			require.ErrorIs(t, err, composables.ErrInvalidPassword)
		}
		stored, err := userRepo.GetByID(f.Ctx, pending.ID())
		require.NoError(t, err)
		assert.Equal(t, user.MaxTemporaryPasswordAttempts, stored.FailedPasswordAttempts())

		_, err = authService.VerifyPassword(f.Ctx, pending.Email().Value(), "Temporary-1")
		require.ErrorIs(t, err, user.ErrTemporaryPasswordExhausted)
	})

	t.Run("active users keep their password flow", func(t *testing.T) {
		active := createOnboardingTestUser(t, f.Ctx, nil)

		for i := 0; i < user.MaxTemporaryPasswordAttempts+1; i++ {
			_, err := authService.VerifyPassword(f.Ctx, active.Email().Value(), "wrong-guess")
			require.ErrorIs(t, err, composables.ErrInvalidPassword)
		}
		_, sess, err := authService.Authenticate(f.Ctx, active.Email().Value(), "Permanent123!")
		require.NoError(t, err)
		assert.True(t, sess.IsActive())
	})
}

func TestAuthService_TemporaryPasswordAttemptsHoldUnderConcurrency(t *testing.T) {
	// Falsely green if the limit is read before the comparison: parallel guesses would all be evaluated.
	f := setupTest(t)
	ctx := userCommittedCtx(f)
	authService := itf.GetService[services.AuthService](f)
	userRepo := persistence.NewUserRepository(persistence.NewUploadRepository())
	pending := createOnboardingTestUser(t, ctx, issued(t, "Temporary-1", time.Now().Add(time.Hour)))

	const guesses = 20
	var wg sync.WaitGroup
	for i := 0; i < guesses; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := authService.VerifyPassword(ctx, pending.Email().Value(), "wrong-guess")
			assert.True(t, errors.Is(err, composables.ErrInvalidPassword) || errors.Is(err, user.ErrTemporaryPasswordExhausted), err)
		}()
	}
	wg.Wait()

	stored, err := userRepo.GetByID(ctx, pending.ID())
	require.NoError(t, err)
	assert.Equal(t, user.MaxTemporaryPasswordAttempts, stored.FailedPasswordAttempts())
	_, err = authService.VerifyPassword(ctx, pending.Email().Value(), "Temporary-1")
	require.ErrorIs(t, err, user.ErrTemporaryPasswordExhausted)
}

func TestUserService_UpdateSelfKeepsReissuedTemporaryPassword(t *testing.T) {
	// Falsely green if a profile form built from a stale user snapshot writes the old password hash back.
	t.Parallel()
	f := setupTest(t)
	adminCtx := persistAdmin(t, f, permissions.UserRead, permissions.UserUpdate)
	userService := itf.GetService[services.UserService](f)
	userRepo := persistence.NewUserRepository(persistence.NewUploadRepository())
	stale := createOnboardingTestUser(t, f.Ctx, nil)

	reissued, err := userService.IssueTemporaryPassword(adminCtx, stale.ID(), "")
	require.NoError(t, err)

	selfCtx := composables.WithUser(f.Ctx, stale)
	_, err = userService.UpdateSelf(selfCtx, stale.SetName("Stale", "Form", ""))
	require.NoError(t, err)

	stored, err := userRepo.GetByID(f.Ctx, stale.ID())
	require.NoError(t, err)
	assert.True(t, stored.CheckPassword(reissued.Password))
	assert.False(t, stored.CheckPassword("Permanent123!"))
	assert.True(t, stored.IsPendingOnboarding())
}

func TestAuthService_TemporaryPasswordRacingReissue(t *testing.T) {
	// Falsely green if a password verified before a reissue can still open onboarding afterwards.
	t.Parallel()
	f := setupTest(t)
	adminCtx := persistAdmin(t, f, permissions.UserRead, permissions.UserUpdate)
	authService := itf.GetService[services.AuthService](f)
	userService := itf.GetService[services.UserService](f)
	pending := createOnboardingTestUser(t, f.Ctx, issued(t, "Temporary-1", time.Now().Add(time.Hour)))

	verified, err := authService.VerifyPassword(f.Ctx, pending.Email().Value(), "Temporary-1")
	require.NoError(t, err)
	_, err = userService.IssueTemporaryPassword(adminCtx, pending.ID(), "Temporary-2")
	require.NoError(t, err)

	_, err = authService.CreateOnboardingSession(f.Ctx, verified)
	require.ErrorIs(t, err, composables.ErrInvalidPassword)
	_, err = authService.VerifyPassword(f.Ctx, pending.Email().Value(), "Temporary-1")
	require.Error(t, err)
}

func TestAuthService_ExhaustedTemporaryPasswordIsNotCompared(t *testing.T) {
	// Falsely green if an exhausted account still distinguishes right and wrong guesses.
	t.Parallel()
	f := setupTest(t)
	authService := itf.GetService[services.AuthService](f)
	exhausted := createOnboardingTestUser(t, f.Ctx, func(u user.User) user.User {
		out, err := u.IssueTemporaryPassword("Temporary-1", time.Now().Add(time.Hour))
		require.NoError(t, err)
		return user.New(out.FirstName(), out.LastName(), out.Email(), out.UILanguage(),
			user.WithTenantID(out.TenantID()),
			user.WithPassword(out.Password()),
			user.WithStatus(out.Status()),
			user.WithPasswordExpiresAt(out.PasswordExpiresAt()),
			user.WithFailedPasswordAttempts(user.MaxTemporaryPasswordAttempts),
		)
	})

	_, wrongErr := authService.VerifyPassword(f.Ctx, exhausted.Email().Value(), "wrong-guess")
	_, rightErr := authService.VerifyPassword(f.Ctx, exhausted.Email().Value(), "Temporary-1")
	require.ErrorIs(t, wrongErr, user.ErrTemporaryPasswordExhausted)
	require.ErrorIs(t, rightErr, user.ErrTemporaryPasswordExhausted)
}

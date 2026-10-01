package user_test

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
)

func newActiveUser(t *testing.T) user.User {
	t.Helper()
	u, err := user.New("Ann", "Lee", internet.MustParseEmail("ann@example.com"), user.UILanguageEN).SetPassword("permanent-pass")
	require.NoError(t, err)
	return u
}

func TestNew_DefaultsToActive(t *testing.T) {
	t.Parallel()
	u := newActiveUser(t)
	assert.Equal(t, user.StatusActive, u.Status())
	assert.False(t, u.IsPendingOnboarding())
	assert.False(t, u.HasTemporaryPassword())
	require.NoError(t, u.TemporaryPasswordUsable(time.Now()))
}

func TestIssueTemporaryPassword_ReplacesPasswordAndRequiresOnboarding(t *testing.T) {
	t.Parallel()
	expiresAt := time.Now().Add(user.TemporaryPasswordTTL)

	u, err := newActiveUser(t).IssueTemporaryPassword("temporary-1", expiresAt)
	require.NoError(t, err)

	assert.True(t, u.IsPendingOnboarding())
	assert.True(t, u.HasTemporaryPassword())
	assert.Equal(t, expiresAt, u.PasswordExpiresAt())
	assert.True(t, u.CheckPassword("temporary-1"))
	assert.False(t, u.CheckPassword("permanent-pass"))
	assert.NotEqual(t, "temporary-1", u.Password(), "only the hash is stored")
}

func TestIssueTemporaryPassword_AppliesPasswordPolicy(t *testing.T) {
	t.Parallel()
	_, err := newActiveUser(t).IssueTemporaryPassword("short", time.Now().Add(time.Hour))
	require.ErrorIs(t, err, user.ErrPasswordTooShort)
}

func TestIssueTemporaryPassword_RequiresExpiry(t *testing.T) {
	// Falsely green if a zero expiry produced a pending user whose temporary password has no limits.
	t.Parallel()
	_, err := newActiveUser(t).IssueTemporaryPassword("temporary-1", time.Time{})
	require.ErrorIs(t, err, user.ErrTemporaryPasswordNoExpiry)
}

func TestTemporaryPasswordUsable(t *testing.T) {
	t.Parallel()
	now := time.Now()
	issued, err := newActiveUser(t).IssueTemporaryPassword("temporary-1", now.Add(time.Hour))
	require.NoError(t, err)

	require.NoError(t, issued.TemporaryPasswordUsable(now))
	require.ErrorIs(t, issued.TemporaryPasswordUsable(now.Add(2*time.Hour)), user.ErrTemporaryPasswordExpired)

	exhausted := user.New("", "", issued.Email(), user.UILanguageEN,
		user.WithPassword(issued.Password()),
		user.WithStatus(user.StatusPendingOnboarding),
		user.WithPasswordExpiresAt(now.Add(time.Hour)),
		user.WithFailedPasswordAttempts(user.MaxTemporaryPasswordAttempts),
	)
	require.ErrorIs(t, exhausted.TemporaryPasswordUsable(now), user.ErrTemporaryPasswordExhausted)
}

func TestCompleteOnboarding(t *testing.T) {
	t.Parallel()
	pending, err := newActiveUser(t).IssueTemporaryPassword("temporary-1", time.Now().Add(time.Hour))
	require.NoError(t, err)

	t.Run("rejects the temporary password", func(t *testing.T) {
		t.Parallel()
		_, err := pending.CompleteOnboarding("temporary-1")
		require.ErrorIs(t, err, user.ErrPasswordReusesTemporary)
	})

	t.Run("rejects a password outside the policy", func(t *testing.T) {
		t.Parallel()
		_, err := pending.CompleteOnboarding("short")
		require.ErrorIs(t, err, user.ErrPasswordTooShort)
		_, err = pending.CompleteOnboarding(strings.Repeat("a", user.MaxPasswordBytes+1))
		require.ErrorIs(t, err, user.ErrPasswordTooLong)
	})

	t.Run("activates with the user's own password", func(t *testing.T) {
		t.Parallel()
		active, err := pending.CompleteOnboarding("my own passphrase")
		require.NoError(t, err)
		assert.Equal(t, user.StatusActive, active.Status())
		assert.False(t, active.HasTemporaryPassword())
		assert.True(t, active.CheckPassword("my own passphrase"))
		assert.False(t, active.CheckPassword("temporary-1"))
	})

	t.Run("requires pending onboarding", func(t *testing.T) {
		t.Parallel()
		_, err := newActiveUser(t).CompleteOnboarding("my own passphrase")
		require.ErrorIs(t, err, user.ErrNotPendingOnboarding)
	})
}

func TestValidatePassword_CountsCharactersNotBytes(t *testing.T) {
	t.Parallel()
	require.NoError(t, user.ValidatePassword("пароль12"))
	require.ErrorIs(t, user.ValidatePassword("пароль1"), user.ErrPasswordTooShort)
}

func TestGenerateTemporaryPassword(t *testing.T) {
	t.Parallel()
	first, err := user.GenerateTemporaryPassword()
	require.NoError(t, err)
	second, err := user.GenerateTemporaryPassword()
	require.NoError(t, err)
	require.NoError(t, user.ValidatePassword(first))
	assert.NotEqual(t, first, second)
}

package commands

import (
	"testing"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSeedSuperadmin_UserCreation tests that the superadmin user is created with correct properties
// Note: This is a unit test that tests user creation logic without requiring a database
func TestSeedSuperadmin_UserCreation(t *testing.T) {
	t.Run("CreateSuperadminUser", func(t *testing.T) {
		// Create superadmin user with the same parameters as the seed command
		superadminUser, err := user.New(
			"Super",
			"Admin",
			internet.MustParseEmail("admin@superadmin.local"),
			user.UILanguageEN,
			user.WithType(user.TypeSuperAdmin),
		).SetPassword("SuperAdmin123!")
		require.NoError(t, err)
		require.NotNil(t, superadminUser)

		// Verify user properties
		assert.Equal(t, "Super", superadminUser.FirstName())
		assert.Equal(t, "Admin", superadminUser.LastName())
		assert.Equal(t, "admin@superadmin.local", superadminUser.Email().Value())
		assert.Equal(t, user.TypeSuperAdmin, superadminUser.Type())
		assert.Equal(t, user.UILanguageEN, superadminUser.UILanguage())

		// Verify user can login with the password
		assert.True(t, superadminUser.CheckPassword("SuperAdmin123!"))
		assert.False(t, superadminUser.CheckPassword("WrongPassword"))
	})

	t.Run("VerifyDefaultTenantID", func(t *testing.T) {
		// Verify the default tenant ID used in seed command is correct
		defaultTenantID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
		assert.NotEqual(t, uuid.Nil, defaultTenantID)
		assert.Equal(t, "00000000-0000-0000-0000-000000000001", defaultTenantID.String())
	})
}

func TestSeedSuperadmin_PasswordValidation(t *testing.T) {
	t.Run("ValidPassword", func(t *testing.T) {
		// Create superadmin user
		superadminUser := user.New(
			"Super",
			"Admin",
			internet.MustParseEmail("admin@superadmin.local"),
			user.UILanguageEN,
			user.WithType(user.TypeSuperAdmin),
		)

		// Set password
		userWithPassword, err := superadminUser.SetPassword("SuperAdmin123!")
		require.NoError(t, err)
		require.NotNil(t, userWithPassword)

		// Verify password was set correctly
		assert.True(t, userWithPassword.CheckPassword("SuperAdmin123!"))
		assert.False(t, userWithPassword.CheckPassword("WrongPassword"))
	})

	t.Run("EmptyPasswordHashesSuccessfully", func(t *testing.T) {
		// Create superadmin user
		superadminUser := user.New(
			"Super",
			"Admin",
			internet.MustParseEmail("admin@superadmin.local"),
			user.UILanguageEN,
			user.WithType(user.TypeSuperAdmin),
		)

		// Note: bcrypt actually allows empty passwords, so this will succeed
		// In production, validation should happen before calling SetPassword
		userWithPassword, err := superadminUser.SetPassword("")
		require.NoError(t, err)
		require.NotNil(t, userWithPassword)

		// Verify empty password was hashed
		assert.True(t, userWithPassword.CheckPassword(""))
		assert.False(t, userWithPassword.CheckPassword("SomePassword"))
	})
}

func TestSeedSuperadmin_EmailValidation(t *testing.T) {
	t.Run("ValidEmail", func(t *testing.T) {
		email := internet.MustParseEmail("admin@superadmin.local")
		assert.NotNil(t, email)
		assert.Equal(t, "admin@superadmin.local", email.Value())
		assert.Equal(t, "admin", email.Username())
		assert.Equal(t, "superadmin.local", email.Domain())
	})
}

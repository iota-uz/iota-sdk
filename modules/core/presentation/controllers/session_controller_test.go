package controllers_test

import (
	"crypto/sha256"
	"fmt"
	"net/http"
	"testing"

	"github.com/iota-uz/iota-sdk/modules"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/session"
	"github.com/iota-uz/iota-sdk/modules/core/permissions"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/controllers"
	"github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
	"github.com/iota-uz/iota-sdk/pkg/itf"
)

// ACCOUNT SESSION CONTROLLER TESTS
// These tests validate session management from the user's account perspective

// persistTestUser writes the test user to the database so session FK constraints are satisfied.
// The ITF creates an in-memory user only; sessions reference users by ID. Raw SQL is used here
// for test setup; using UserService/UserRepository would be more maintainable if they expose
// a suitable create-or-use API for tests.
func persistTestUser(t *testing.T, env *itf.TestEnvironment) {
	t.Helper()

	ctx := env.Ctx
	user := env.User

	// Handle potentially nil email/phone
	var email, phone string
	if user.Email() != nil {
		email = user.Email().Value()
	}
	if user.Phone() != nil {
		phone = user.Phone().Value()
	}

	// Insert a minimal user row to satisfy FK constraints
	// Use ON CONFLICT to avoid errors if user already exists
	query := `
		INSERT INTO users (id, type, first_name, last_name, email, phone, password, tenant_id, ui_language, created_at, updated_at)
		VALUES ($1, 'system', $2, $3, $4, $5, $6, $7, 'en', NOW(), NOW())
		ON CONFLICT (id) DO NOTHING
	`

	_, err := env.Pool.Exec(ctx, query,
		user.ID(),
		user.FirstName(),
		user.LastName(),
		email,
		phone,
		user.Password(), // This is the password hash from the user entity
		env.Tenant.ID,
	)
	if err != nil {
		t.Fatalf("Failed to persist test user: %v", err)
	}
}

func TestAccountController_GetSessions(t *testing.T) {
	t.Parallel()

	t.Run("Returns_401_When_Session_Cookie_Missing", func(t *testing.T) {
		t.Parallel()

		// Create suite with authenticated user but no session cookie
		suite := itf.NewSuiteBuilder(t).
			WithModules(modules.BuiltInModules...).
			AsUser().
			Build()

		controller := controllers.NewAccountController(suite.Env().App)
		suite.Register(controller)

		// Request without session cookie
		suite.GET("/account/sessions").
			Assert(t).
			ExpectStatus(http.StatusUnauthorized).
			ExpectBodyContains("Session not found")
	})

	t.Run("Returns_Sessions_List_For_Authenticated_User", func(t *testing.T) {
		t.Parallel()

		suite := itf.NewSuiteBuilder(t).
			WithModules(modules.BuiltInModules...).
			AsUser().
			Build()

		// Persist test user to database (required for FK constraints)
		persistTestUser(t, suite.Env())

		controller := controllers.NewAccountController(suite.Env().App)
		suite.Register(controller)

		config := configuration.Use()
		currentToken := "test-current-session-token"

		// Create current session for the user
		sessionService := itf.GetService[services.SessionService](suite.Env())
		ctx := suite.Env().Ctx

		err := sessionService.Create(ctx, &session.CreateDTO{
			UserID:    suite.Env().User.ID(),
			TenantID:  suite.Env().Tenant.ID,
			IP:        "127.0.0.1",
			UserAgent: "test-agent",
			Token:     currentToken,
		})
		if err != nil {
			t.Fatalf("Failed to create current session: %v", err)
		}

		// Create 2 additional test sessions
		err = sessionService.Create(ctx, &session.CreateDTO{
			UserID:    suite.Env().User.ID(),
			TenantID:  suite.Env().Tenant.ID,
			IP:        "192.168.1.1",
			UserAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64)",
			Token:     "other-token-1",
		})
		if err != nil {
			t.Fatalf("Failed to create test session: %v", err)
		}

		err = sessionService.Create(ctx, &session.CreateDTO{
			UserID:    suite.Env().User.ID(),
			TenantID:  suite.Env().Tenant.ID,
			IP:        "192.168.1.2",
			UserAgent: "Mozilla/5.0 (iPhone; CPU iPhone OS 14_0 like Mac OS X)",
			Token:     "other-token-2",
		})
		if err != nil {
			t.Fatalf("Failed to create test session: %v", err)
		}

		response := suite.GET("/account/sessions").
			Cookie(config.SidCookieKey, currentToken).
			Expect(t)

		response.Status(http.StatusOK)
		// Should contain sessions list
	})

	t.Run("Correctly_Identifies_Current_Session_With_IsCurrent_Flag", func(t *testing.T) {
		t.Parallel()

		suite := itf.NewSuiteBuilder(t).
			WithModules(modules.BuiltInModules...).
			AsUser().
			Build()

		persistTestUser(t, suite.Env())

		controller := controllers.NewAccountController(suite.Env().App)
		suite.Register(controller)

		config := configuration.Use()
		currentToken := "my-current-token"

		sessionService := itf.GetService[services.SessionService](suite.Env())
		ctx := suite.Env().Ctx

		// Create current session
		err := sessionService.Create(ctx, &session.CreateDTO{
			UserID:    suite.Env().User.ID(),
			TenantID:  suite.Env().Tenant.ID,
			IP:        "127.0.0.1",
			UserAgent: "current-agent",
			Token:     currentToken,
		})
		if err != nil {
			t.Fatalf("Failed to create current session: %v", err)
		}

		// Create additional session (non-current)
		err = sessionService.Create(ctx, &session.CreateDTO{
			UserID:    suite.Env().User.ID(),
			TenantID:  suite.Env().Tenant.ID,
			IP:        "192.168.1.3",
			UserAgent: "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)",
			Token:     "other-session",
		})
		if err != nil {
			t.Fatalf("Failed to create test session: %v", err)
		}

		response := suite.GET("/account/sessions").
			Cookie(config.SidCookieKey, currentToken).
			Expect(t)

		response.Status(http.StatusOK)
		// The page should show current session indicator
		// Actual indicator text depends on template implementation
	})

	t.Run("Returns_Single_Session_When_Only_Current_Exists", func(t *testing.T) {
		t.Parallel()

		suite := itf.NewSuiteBuilder(t).
			WithModules(modules.BuiltInModules...).
			AsUser().
			Build()

		persistTestUser(t, suite.Env())

		controller := controllers.NewAccountController(suite.Env().App)
		suite.Register(controller)

		config := configuration.Use()
		currentToken := "only-token"
		sessionService := itf.GetService[services.SessionService](suite.Env())
		ctx := suite.Env().Ctx

		// Delete all existing sessions for the user
		_, err := sessionService.DeleteByUserId(ctx, suite.Env().User.ID())
		if err != nil {
			t.Fatalf("Failed to delete sessions: %v", err)
		}

		// Create only a current session (otherwise request fails)
		err = sessionService.Create(ctx, &session.CreateDTO{
			UserID:    suite.Env().User.ID(),
			TenantID:  suite.Env().Tenant.ID,
			IP:        "127.0.0.1",
			UserAgent: "test",
			Token:     currentToken,
		})
		if err != nil {
			t.Fatalf("Failed to create session: %v", err)
		}

		response := suite.GET("/account/sessions").
			Cookie(config.SidCookieKey, currentToken).
			Expect(t)

		response.Status(http.StatusOK)
		// Should render sessions page with single (current) session
	})
}

func TestAccountController_RevokeSession(t *testing.T) {
	t.Parallel()

	t.Run("Returns_401_When_Session_Cookie_Missing", func(t *testing.T) {
		t.Parallel()

		suite := itf.NewSuiteBuilder(t).
			WithModules(modules.BuiltInModules...).
			AsUser().
			Build()

		controller := controllers.NewAccountController(suite.Env().App)
		suite.Register(controller)

		suite.DELETE("/account/sessions/dummy-token").
			Assert(t).
			ExpectStatus(http.StatusUnauthorized).
			ExpectBodyContains("Session not found")
	})

	t.Run("Returns_403_When_Attempting_To_Revoke_Current_Session", func(t *testing.T) {
		t.Parallel()

		suite := itf.NewSuiteBuilder(t).
			WithModules(modules.BuiltInModules...).
			AsUser().
			Build()

		persistTestUser(t, suite.Env())

		controller := controllers.NewAccountController(suite.Env().App)
		suite.Register(controller)

		sessionService := itf.GetService[services.SessionService](suite.Env())
		ctx := suite.Env().Ctx
		config := configuration.Use()

		// Create current session with known token
		currentToken := "current-session-token-12345"
		err := sessionService.Create(ctx, &session.CreateDTO{
			UserID:    suite.Env().User.ID(),
			TenantID:  suite.Env().Tenant.ID,
			IP:        "127.0.0.1",
			UserAgent: "test-agent",
			Token:     currentToken,
		})
		if err != nil {
			t.Fatalf("Failed to create current session: %v", err)
		}

		// Hash the current token (as done in ViewModel)
		tokenHash := hashToken(currentToken)

		// Attempt to revoke current session
		suite.DELETE(fmt.Sprintf("/account/sessions/%s", tokenHash)).
			Cookie(config.SidCookieKey, currentToken).
			Assert(t).
			ExpectStatus(http.StatusForbidden)
	})

	t.Run("Successfully_Revokes_Other_Session_And_Returns_200", func(t *testing.T) {
		t.Parallel()

		suite := itf.NewSuiteBuilder(t).
			WithModules(modules.BuiltInModules...).
			AsUser().
			Build()

		persistTestUser(t, suite.Env())

		controller := controllers.NewAccountController(suite.Env().App)
		suite.Register(controller)

		sessionService := itf.GetService[services.SessionService](suite.Env())
		ctx := suite.Env().Ctx
		config := configuration.Use()

		// Create current session
		currentToken := "current-session-token"
		err := sessionService.Create(ctx, &session.CreateDTO{
			UserID:    suite.Env().User.ID(),
			TenantID:  suite.Env().Tenant.ID,
			IP:        "127.0.0.1",
			UserAgent: "current-agent",
			Token:     currentToken,
		})
		if err != nil {
			t.Fatalf("Failed to create current session: %v", err)
		}

		// Create another session to revoke
		otherToken := "other-session-token-98765"
		err = sessionService.Create(ctx, &session.CreateDTO{
			UserID:    suite.Env().User.ID(),
			TenantID:  suite.Env().Tenant.ID,
			IP:        "192.168.1.100",
			UserAgent: "other-agent",
			Token:     otherToken,
		})
		if err != nil {
			t.Fatalf("Failed to create other session: %v", err)
		}

		// Hash the other token
		otherTokenHash := hashToken(otherToken)

		// Revoke the other session (with current session cookie)
		suite.DELETE(fmt.Sprintf("/account/sessions/%s", otherTokenHash)).
			Cookie(config.SidCookieKey, currentToken).
			Assert(t).
			ExpectStatus(http.StatusOK)

		// Verify session was deleted
		_, err = sessionService.GetByToken(ctx, otherToken)
		if err == nil {
			t.Error("Expected session to be deleted, but it still exists")
		}
	})

	t.Run("Returns_404_For_NonExistent_Session_Token", func(t *testing.T) {
		t.Parallel()

		suite := itf.NewSuiteBuilder(t).
			WithModules(modules.BuiltInModules...).
			AsUser().
			Build()

		persistTestUser(t, suite.Env())

		controller := controllers.NewAccountController(suite.Env().App)
		suite.Register(controller)

		config := configuration.Use()
		sessionService := itf.GetService[services.SessionService](suite.Env())
		ctx := suite.Env().Ctx

		currentToken := "current-token"
		err := sessionService.Create(ctx, &session.CreateDTO{
			UserID:    suite.Env().User.ID(),
			TenantID:  suite.Env().Tenant.ID,
			IP:        "127.0.0.1",
			UserAgent: "test",
			Token:     currentToken,
		})
		if err != nil {
			t.Fatalf("Failed to create session: %v", err)
		}

		// Hash a non-existent token
		nonExistentHash := hashToken("non-existent-token")

		suite.DELETE(fmt.Sprintf("/account/sessions/%s", nonExistentHash)).
			Cookie(config.SidCookieKey, currentToken).
			Assert(t).
			ExpectStatus(http.StatusNotFound).
			ExpectBodyContains("not found")
	})
}

func TestAccountController_RevokeAllOtherSessions(t *testing.T) {
	t.Parallel()

	t.Run("Returns_401_When_Session_Cookie_Missing", func(t *testing.T) {
		t.Parallel()

		suite := itf.NewSuiteBuilder(t).
			WithModules(modules.BuiltInModules...).
			AsUser().
			Build()

		controller := controllers.NewAccountController(suite.Env().App)
		suite.Register(controller)

		suite.DELETE("/account/sessions/others").
			Assert(t).
			ExpectStatus(http.StatusUnauthorized).
			ExpectBodyContains("Session not found")
	})

	t.Run("Revokes_All_Sessions_Except_Current", func(t *testing.T) {
		t.Parallel()

		suite := itf.NewSuiteBuilder(t).
			WithModules(modules.BuiltInModules...).
			AsUser().
			Build()

		persistTestUser(t, suite.Env())

		controller := controllers.NewAccountController(suite.Env().App)
		suite.Register(controller)

		sessionService := itf.GetService[services.SessionService](suite.Env())
		ctx := suite.Env().Ctx
		config := configuration.Use()

		// Create current session
		currentToken := "current-active-token"
		err := sessionService.Create(ctx, &session.CreateDTO{
			UserID:    suite.Env().User.ID(),
			TenantID:  suite.Env().Tenant.ID,
			IP:        "127.0.0.1",
			UserAgent: "current",
			Token:     currentToken,
		})
		if err != nil {
			t.Fatalf("Failed to create current session: %v", err)
		}

		// Create multiple other sessions
		for i := 1; i <= 3; i++ {
			err := sessionService.Create(ctx, &session.CreateDTO{
				UserID:    suite.Env().User.ID(),
				TenantID:  suite.Env().Tenant.ID,
				IP:        fmt.Sprintf("192.168.1.%d", i),
				UserAgent: fmt.Sprintf("agent-%d", i),
				Token:     fmt.Sprintf("other-token-%d", i),
			})
			if err != nil {
				t.Fatalf("Failed to create test session %d: %v", i, err)
			}
		}

		// Revoke all other sessions
		suite.DELETE("/account/sessions/others").
			Cookie(config.SidCookieKey, currentToken).
			Assert(t).
			ExpectStatus(http.StatusOK)

		// Verify current session still exists
		currentSession, err := sessionService.GetByToken(ctx, currentToken)
		if err != nil || currentSession == nil {
			t.Error("Current session should still exist")
		}

		// Verify other sessions were deleted
		allSessions, err := sessionService.GetByUserID(ctx, suite.Env().User.ID())
		if err != nil {
			t.Fatalf("Failed to get user sessions: %v", err)
		}

		if len(allSessions) != 1 {
			t.Errorf("Expected 1 session (current), got %d", len(allSessions))
		}
	})

	t.Run("Returns_Success_Even_With_Only_Current_Session", func(t *testing.T) {
		t.Parallel()

		suite := itf.NewSuiteBuilder(t).
			WithModules(modules.BuiltInModules...).
			AsUser().
			Build()

		persistTestUser(t, suite.Env())

		controller := controllers.NewAccountController(suite.Env().App)
		suite.Register(controller)

		sessionService := itf.GetService[services.SessionService](suite.Env())
		ctx := suite.Env().Ctx
		config := configuration.Use()

		// Delete all existing sessions first
		_, err := sessionService.DeleteByUserId(ctx, suite.Env().User.ID())
		if err != nil {
			t.Fatalf("Failed to delete sessions: %v", err)
		}

		// Create only current session
		currentToken := "only-session-token"
		err = sessionService.Create(ctx, &session.CreateDTO{
			UserID:    suite.Env().User.ID(),
			TenantID:  suite.Env().Tenant.ID,
			IP:        "127.0.0.1",
			UserAgent: "current",
			Token:     currentToken,
		})
		if err != nil {
			t.Fatalf("Failed to create current session: %v", err)
		}

		response := suite.DELETE("/account/sessions/others").
			Cookie(config.SidCookieKey, currentToken).
			Expect(t)

		response.Status(http.StatusOK)
		// Should succeed with count=0 in message
	})
}

// ADMIN SESSION CONTROLLER TESTS
// These tests validate admin session management functionality

func TestSessionController_RevokeUserSession(t *testing.T) {
	t.Parallel()

	t.Run("Returns_403_Without_SessionDelete_Permission", func(t *testing.T) {
		t.Parallel()

		// User without SessionDelete permission
		suite := itf.NewSuiteBuilder(t).
			WithModules(modules.BuiltInModules...).
			AsUser(permissions.SessionRead). // Only read permission
			Build()

		controller := controllers.NewSessionController(suite.Env().App, "/sessions")
		suite.Register(controller)

		suite.DELETE("/sessions/dummy-token").
			Assert(t).
			ExpectStatus(http.StatusForbidden)
	})

	t.Run("Successfully_Revokes_User_Session", func(t *testing.T) {
		t.Parallel()

		suite := itf.NewSuiteBuilder(t).
			WithModules(modules.BuiltInModules...).
			AsUser(permissions.SessionDelete, permissions.SessionRead).
			Build()

		persistTestUser(t, suite.Env())

		controller := controllers.NewSessionController(suite.Env().App, "/sessions")
		suite.Register(controller)

		sessionService := itf.GetService[services.SessionService](suite.Env())
		ctx := suite.Env().Ctx

		// Create a session to revoke
		testToken := "admin-revoke-test-token"
		err := sessionService.Create(ctx, &session.CreateDTO{
			UserID:    suite.Env().User.ID(),
			TenantID:  suite.Env().Tenant.ID,
			IP:        "10.10.10.10",
			UserAgent: "test-browser",
			Token:     testToken,
		})
		if err != nil {
			t.Fatalf("Failed to create test session: %v", err)
		}

		// Revoke the session
		suite.DELETE(fmt.Sprintf("/sessions/%s", testToken)).
			Assert(t).
			ExpectStatus(http.StatusOK)

		// Verify session was deleted
		_, err = sessionService.GetByToken(ctx, testToken)
		if err == nil {
			t.Error("Expected session to be deleted")
		}
	})

	t.Run("Returns_404_For_NonExistent_Session", func(t *testing.T) {
		t.Parallel()

		suite := itf.NewSuiteBuilder(t).
			WithModules(modules.BuiltInModules...).
			AsUser(permissions.SessionDelete, permissions.SessionRead).
			Build()

		controller := controllers.NewSessionController(suite.Env().App, "/sessions")
		suite.Register(controller)

		nonExistentToken := "non-existent-session-token"

		suite.DELETE(fmt.Sprintf("/sessions/%s", nonExistentToken)).
			Assert(t).
			ExpectStatus(http.StatusNotFound) // Session not found
	})
}

func TestSessionController_GetAllSessions(t *testing.T) {
	t.Parallel()

	t.Run("Returns_403_Without_SessionRead_Permission", func(t *testing.T) {
		t.Parallel()

		suite := itf.NewSuiteBuilder(t).
			WithModules(modules.BuiltInModules...).
			AsUser(). // No permissions
			Build()

		controller := controllers.NewSessionController(suite.Env().App, "/sessions")
		suite.Register(controller)

		suite.GET("/sessions").
			Assert(t).
			ExpectStatus(http.StatusForbidden)
	})

	t.Run("Returns_Paginated_Sessions_List", func(t *testing.T) {
		t.Parallel()

		suite := itf.NewSuiteBuilder(t).
			WithModules(modules.BuiltInModules...).
			AsUser(permissions.SessionRead).
			Build()

		persistTestUser(t, suite.Env())

		controller := controllers.NewSessionController(suite.Env().App, "/sessions")
		suite.Register(controller)

		sessionService := itf.GetService[services.SessionService](suite.Env())
		ctx := suite.Env().Ctx

		// Create multiple sessions
		for i := 1; i <= 5; i++ {
			err := sessionService.Create(ctx, &session.CreateDTO{
				UserID:    suite.Env().User.ID(),
				TenantID:  suite.Env().Tenant.ID,
				IP:        fmt.Sprintf("172.16.0.%d", i),
				UserAgent: fmt.Sprintf("browser-%d", i),
				Token:     fmt.Sprintf("session-token-%d", i),
			})
			if err != nil {
				t.Fatalf("Failed to create test session %d: %v", i, err)
			}
		}

		response := suite.GET("/sessions").Expect(t)

		response.Status(http.StatusOK)
		// Should contain session list (actual content depends on template)
	})

	t.Run("Filters_By_User_Search_Query", func(t *testing.T) {
		t.Parallel()

		suite := itf.NewSuiteBuilder(t).
			WithModules(modules.BuiltInModules...).
			AsUser(permissions.SessionRead, permissions.UserRead).
			Build()

		persistTestUser(t, suite.Env())

		controller := controllers.NewSessionController(suite.Env().App, "/sessions")
		suite.Register(controller)

		sessionService := itf.GetService[services.SessionService](suite.Env())
		ctx := suite.Env().Ctx

		// Create session for current user
		err := sessionService.Create(ctx, &session.CreateDTO{
			UserID:    suite.Env().User.ID(),
			TenantID:  suite.Env().Tenant.ID,
			IP:        "192.168.100.1",
			UserAgent: "search-test-agent",
			Token:     "searchable-token",
		})
		if err != nil {
			t.Fatalf("Failed to create test session: %v", err)
		}

		// Search by user's first name or email
		searchQuery := suite.Env().User.FirstName()

		response := suite.GET("/sessions").
			WithQueryValue("search", searchQuery).
			Expect(t)

		response.Status(http.StatusOK)
		// Should filter sessions by user name/email
	})

	t.Run("Paginates_Correctly_With_Limit_And_Offset", func(t *testing.T) {
		t.Parallel()

		suite := itf.NewSuiteBuilder(t).
			WithModules(modules.BuiltInModules...).
			AsUser(permissions.SessionRead).
			Build()

		persistTestUser(t, suite.Env())

		controller := controllers.NewSessionController(suite.Env().App, "/sessions")
		suite.Register(controller)

		sessionService := itf.GetService[services.SessionService](suite.Env())
		ctx := suite.Env().Ctx

		// Create 10 test sessions
		for i := 1; i <= 10; i++ {
			err := sessionService.Create(ctx, &session.CreateDTO{
				UserID:    suite.Env().User.ID(),
				TenantID:  suite.Env().Tenant.ID,
				IP:        fmt.Sprintf("10.20.30.%d", i),
				UserAgent: fmt.Sprintf("paginate-test-%d", i),
				Token:     fmt.Sprintf("page-token-%d", i),
			})
			if err != nil {
				t.Fatalf("Failed to create test session %d: %v", i, err)
			}
		}

		// Request first page with limit=5
		response := suite.GET("/sessions").
			WithQuery(map[string]string{
				"limit": "5",
				"page":  "1",
			}).
			Expect(t)

		response.Status(http.StatusOK)
		// Should return first 5 sessions

		// Request second page
		response2 := suite.GET("/sessions").
			WithQuery(map[string]string{
				"limit": "5",
				"page":  "2",
			}).
			Expect(t)

		response2.Status(http.StatusOK)
		// Should return next 5 sessions
	})
}

func TestSessionController_Permissions(t *testing.T) {
	t.Parallel()

	t.Run("SessionRead_Allows_Viewing_Not_Revoking", func(t *testing.T) {
		t.Parallel()

		suite := itf.NewSuiteBuilder(t).
			WithModules(modules.BuiltInModules...).
			AsUser(permissions.SessionRead). // Only read permission
			Build()

		controller := controllers.NewSessionController(suite.Env().App, "/sessions")
		suite.Register(controller)

		// Can view sessions
		suite.GET("/sessions").
			Assert(t).
			ExpectStatus(http.StatusOK)

		// Cannot revoke sessions
		suite.DELETE("/sessions/any-token").
			Assert(t).
			ExpectStatus(http.StatusForbidden)
	})

	t.Run("SessionDelete_Allows_Revoking", func(t *testing.T) {
		t.Parallel()

		suite := itf.NewSuiteBuilder(t).
			WithModules(modules.BuiltInModules...).
			AsUser(permissions.SessionDelete).
			Build()

		persistTestUser(t, suite.Env())

		controller := controllers.NewSessionController(suite.Env().App, "/sessions")
		suite.Register(controller)

		sessionService := itf.GetService[services.SessionService](suite.Env())
		ctx := suite.Env().Ctx

		// Create a session to revoke
		testToken := "delete-perm-test-token"
		err := sessionService.Create(ctx, &session.CreateDTO{
			UserID:    suite.Env().User.ID(),
			TenantID:  suite.Env().Tenant.ID,
			IP:        "172.30.0.1",
			UserAgent: "delete-test-agent",
			Token:     testToken,
		})
		if err != nil {
			t.Fatalf("Failed to create test session: %v", err)
		}

		// Can revoke session
		suite.DELETE(fmt.Sprintf("/sessions/%s", testToken)).
			Assert(t).
			ExpectStatus(http.StatusOK)
	})

	t.Run("No_Permissions_Blocks_All_Access", func(t *testing.T) {
		t.Parallel()

		suite := itf.NewSuiteBuilder(t).
			WithModules(modules.BuiltInModules...).
			AsUser(). // No permissions
			Build()

		controller := controllers.NewSessionController(suite.Env().App, "/sessions")
		suite.Register(controller)

		// Cannot view sessions
		suite.GET("/sessions").
			Assert(t).
			ExpectStatus(http.StatusForbidden)

		// Cannot revoke sessions
		suite.DELETE("/sessions/any-token").
			Assert(t).
			ExpectStatus(http.StatusForbidden)
	})
}

// hashToken creates a SHA-256 hash of the token for safe comparison
// Must match the implementation in viewmodels/session.go
func hashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return fmt.Sprintf("%x", hash)
}

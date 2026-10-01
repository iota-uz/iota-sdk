package controllers_test

import (
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/modules"
	"github.com/iota-uz/iota-sdk/modules/core"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/core/permissions"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/controllers"
	"github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/googleoauthconfig"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/httpconfig"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/httpconfig/cookies"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/httpconfig/headers"
	"github.com/iota-uz/iota-sdk/pkg/defaults"
	"github.com/iota-uz/iota-sdk/pkg/itf"
)

var temporaryPasswordPattern = regexp.MustCompile(`data-testid="temporary-password-value"[^>]*>([^<]+)</code>`)

func newOnboardingSuite(t *testing.T) *itf.Suite {
	t.Helper()
	suite := itf.NewSuiteBuilder(t).WithComponents(core.NewComponent(&core.ModuleOptions{
		PermissionSchema: defaults.PermissionSchema(),
	})).Build()
	env := suite.Env()
	browserSessions := itf.GetService[services.BrowserSessionService](env)
	suite.Register(controllers.NewLoginControllerWithBrowserSessions(
		itf.GetService[services.AuthService](env),
		itf.GetService[services.AuthFlowService](env),
		browserSessions,
		itf.GetService[httpconfig.Config](env),
		itf.GetService[cookies.Config](env),
		itf.GetService[headers.Config](env),
		itf.GetService[googleoauthconfig.Config](env),
		&controllers.LoginControllerOptions{},
	))
	suite.Register(controllers.NewOnboardingController(
		env.App,
		itf.GetService[services.UserService](env),
		itf.GetService[services.UploadService](env),
		browserSessions,
	))
	suite.Register(controllers.NewUsersController(
		env.App,
		controllers.WithUserControllerBasePath("/users"),
		controllers.WithUserControllerPermissionSchema(defaults.PermissionSchema()),
	))
	return suite
}

func createPendingUser(t *testing.T, suite *itf.Suite, temporaryPassword string) user.User {
	t.Helper()
	email, err := internet.NewEmail("pending-" + uuid.NewString() + "@example.test")
	require.NoError(t, err)
	pending, err := user.New("", "", email, user.UILanguageEN, user.WithTenantID(suite.Env().Tenant.ID)).
		IssueTemporaryPassword(temporaryPassword, time.Now().Add(user.TemporaryPasswordTTL))
	require.NoError(t, err)
	created, err := persistence.NewUserRepository(persistence.NewUploadRepository()).Create(suite.Env().Ctx, pending)
	require.NoError(t, err)
	return created
}

func sessionCookieFrom(t *testing.T, suite *itf.Suite, response *itf.Response) *http.Cookie {
	t.Helper()
	sid := itf.GetService[cookies.Config](suite.Env()).SID
	for _, cookie := range response.Cookies() {
		if cookie.Name == sid && cookie.Value != "" {
			return cookie
		}
	}
	require.FailNow(t, "session cookie was not issued")
	return nil
}

// The whole server-side onboarding contract: the temporary password opens
// only /onboarding, the form refuses to reuse it, and completion activates the
// account atomically. Falsely green if the gate lets a pending session reach
// another page or if completion leaves the temporary password usable.
func TestOnboardingController_TemporaryPasswordFlow(t *testing.T) {
	t.Parallel()
	suite := newOnboardingSuite(t)
	env := suite.Env()
	pending := createPendingUser(t, suite, "Temporary-1")

	login := suite.POST("/login").
		FormFields(map[string]interface{}{"Email": pending.Email().Value(), "Password": "Temporary-1"}).
		Expect(t).
		Status(http.StatusFound).
		RedirectTo(services.OnboardingPath)
	cookie := sessionCookieFrom(t, suite, login)
	sid := cookie.Name

	suite.GET("/users").Cookie(sid, cookie.Value).Expect(t).
		Status(http.StatusFound).
		RedirectTo(services.OnboardingPath)

	suite.GET(services.OnboardingPath).Cookie(sid, cookie.Value).Expect(t).
		Status(http.StatusOK).
		Contains(`data-testid="onboarding-form"`).
		Contains(pending.Email().Value())

	suite.POST(services.OnboardingPath).Cookie(sid, cookie.Value).
		FormFields(map[string]interface{}{
			"FirstName": "Nodira", "LastName": "Karimova", "Language": "ru",
			"NewPassword": "Temporary-1", "ConfirmPassword": "Temporary-1",
		}).
		Expect(t).
		Status(http.StatusUnprocessableEntity)

	stillPending, err := itf.GetService[services.UserService](env).GetByID(env.Ctx, pending.ID())
	require.NoError(t, err)
	require.True(t, stillPending.IsPendingOnboarding())

	suite.POST(services.OnboardingPath).Cookie(sid, cookie.Value).
		FormFields(map[string]interface{}{
			"FirstName": "Nodira", "LastName": "Karimova", "Language": "ru",
			"NewPassword": "my own passphrase", "ConfirmPassword": "my own passphrase",
		}).
		Expect(t).
		Status(http.StatusFound).
		RedirectTo("/login?" + url.Values{"email": []string{pending.Email().Value()}}.Encode())

	completed, err := itf.GetService[services.UserService](env).GetByID(env.Ctx, pending.ID())
	require.NoError(t, err)
	assert.Equal(t, user.StatusActive, completed.Status())
	assert.Equal(t, "Nodira", completed.FirstName())
	assert.Equal(t, user.UILanguageRU, completed.UILanguage())
	assert.True(t, completed.CheckPassword("my own passphrase"))
	assert.False(t, completed.CheckPassword("Temporary-1"))
	sessions, err := itf.GetService[services.SessionService](env).GetByUserID(env.Ctx, pending.ID())
	require.NoError(t, err)
	assert.Empty(t, sessions, "completion ends the onboarding session")

	// The revoked cookie no longer opens onboarding (ITF injects an active
	// mock session, which onboarding sends back to the application).
	suite.GET(services.OnboardingPath).Cookie(sid, cookie.Value).Expect(t).
		Status(http.StatusFound).
		NotContains(`data-testid="onboarding-form"`)
}

// A full session never renders the onboarding form.
func TestOnboardingController_RequiresOnboardingSession(t *testing.T) {
	t.Parallel()
	suite := newOnboardingSuite(t)

	suite.GET(services.OnboardingPath).Expect(t).
		Status(http.StatusFound).
		RedirectTo("/")
}

// Falsely green if the administrator still sets a permanent password or the
// generated password is not the one stored for the user.
func TestUsersController_Create_IssuesTemporaryPassword(t *testing.T) {
	t.Parallel()
	suite := itf.NewSuiteBuilder(t).
		WithComponents(modules.Components()...).
		AsUser(permissions.UserRead, permissions.UserCreate).
		Build()
	persistAdministrativeTestActor(t, suite, permissions.UserRead, permissions.UserCreate)
	suite.Register(controllers.NewUsersController(
		suite.Env().App,
		controllers.WithUserControllerBasePath("/users"),
		controllers.WithUserControllerPermissionSchema(defaults.PermissionSchema()),
	))
	email := "created-" + uuid.NewString() + "@example.test"

	response := suite.POST("/users").
		FormFields(map[string]interface{}{"Email": email}).
		Expect(t).
		Status(http.StatusOK).
		Contains(`data-testid="user-created"`)
	assert.Equal(t, "no-store", response.Header("Cache-Control"))
	match := temporaryPasswordPattern.FindStringSubmatch(response.Body())
	require.Len(t, match, 2, "the temporary password is shown once")

	created, err := itf.GetService[services.UserService](suite.Env()).GetByEmail(suite.Env().Ctx, email)
	require.NoError(t, err)
	assert.True(t, created.IsPendingOnboarding())
	assert.True(t, created.CheckPassword(match[1]))
	assert.WithinDuration(t, time.Now().Add(user.TemporaryPasswordTTL), created.PasswordExpiresAt(), time.Minute)
	assert.Empty(t, created.FirstName())
}

// Falsely green if the reissue route renders a password without storing it or
// without requiring the update permission.
func TestUsersController_IssueTemporaryPassword(t *testing.T) {
	t.Parallel()
	newSuite := func(perms ...permission.Permission) *itf.Suite {
		suite := itf.NewSuiteBuilder(t).WithComponents(modules.Components()...).AsUser(perms...).Build()
		persistAdministrativeTestActor(t, suite, perms...)
		suite.Register(controllers.NewUsersController(
			suite.Env().App,
			controllers.WithUserControllerBasePath("/users"),
			controllers.WithUserControllerPermissionSchema(defaults.PermissionSchema()),
		))
		return suite
	}

	t.Run("issues and shows the password once", func(t *testing.T) {
		suite := newSuite(permissions.UserRead, permissions.UserUpdate)
		target := createTargetUserForControllerTest(t, suite, "reissue-"+uuid.NewString()+"@example.test")

		response := suite.POST(fmt.Sprintf("/users/%d/temporary-password", target.ID())).HTMX().Expect(t).
			Status(http.StatusOK).
			Contains(`data-testid="temporary-password-result"`)
		assert.Equal(t, "no-store", response.Header("Cache-Control"))
		match := temporaryPasswordPattern.FindStringSubmatch(response.Body())
		require.Len(t, match, 2)

		stored, err := persistence.NewUserRepository(persistence.NewUploadRepository()).GetByID(suite.Env().Ctx, target.ID())
		require.NoError(t, err)
		assert.True(t, stored.IsPendingOnboarding())
		assert.True(t, stored.CheckPassword(match[1]))

		suite.GET(fmt.Sprintf("/users/%d/edit", target.ID())).Expect(t).
			Status(http.StatusOK).
			Contains(`data-testid="pending-onboarding-notice"`).
			NotContains(`name="Password"`)
	})

	t.Run("requires the update permission", func(t *testing.T) {
		suite := newSuite(permissions.UserRead)
		target := createTargetUserForControllerTest(t, suite, "reissue-denied-"+uuid.NewString()+"@example.test")

		suite.POST(fmt.Sprintf("/users/%d/temporary-password", target.ID())).HTMX().Expect(t).
			Status(http.StatusForbidden)

		stored, err := persistence.NewUserRepository(persistence.NewUploadRepository()).GetByID(suite.Env().Ctx, target.ID())
		require.NoError(t, err)
		assert.False(t, stored.IsPendingOnboarding())
	})
}

// Falsely green if direct permissions are dropped on creation or the grant
// ceiling is skipped for them.
func TestUsersController_Create_AssignsDirectPermissions(t *testing.T) {
	t.Parallel()
	actorPermissions := []permission.Permission{permissions.UserRead, permissions.UserCreate, permissions.UploadRead}
	suite := itf.NewSuiteBuilder(t).WithComponents(modules.Components()...).AsUser(actorPermissions...).Build()
	persistAdministrativeTestActor(t, suite, actorPermissions...)
	ensurePermissionExistsForControllerTest(t, suite, permissions.RoleDelete)
	suite.Register(controllers.NewUsersController(
		suite.Env().App,
		controllers.WithUserControllerBasePath("/users"),
		controllers.WithUserControllerPermissionSchema(defaults.PermissionSchema()),
	))
	userService := itf.GetService[services.UserService](suite.Env())

	suite.GET("/users/new").Expect(t).
		Status(http.StatusOK).
		Contains(`data-testid="create-user-permissions"`).
		Contains(`value="` + permissions.UploadRead.ID().String() + `"`)

	granted := "granted-" + uuid.NewString() + "@example.test"
	suite.POST("/users").
		FormFields(map[string]interface{}{"Email": granted, "PermissionIDs": permissions.UploadRead.ID().String()}).
		Expect(t).
		Status(http.StatusOK).
		Contains(`data-testid="user-created"`)
	created, err := userService.GetByEmail(suite.Env().Ctx, granted)
	require.NoError(t, err)
	require.Len(t, created.Permissions(), 1)
	assert.Equal(t, permissions.UploadRead.ID(), created.Permissions()[0].ID())

	escalated := "escalated-" + uuid.NewString() + "@example.test"
	response := suite.POST("/users").
		FormFields(map[string]interface{}{"Email": escalated, "PermissionIDs": permissions.RoleDelete.ID().String()}).
		Expect(t)
	assert.NotContains(t, response.Body(), `data-testid="user-created"`)
	_, err = userService.GetByEmail(suite.Env().Ctx, escalated)
	require.Error(t, err, "a permission above the administrator's own must not be granted")
}

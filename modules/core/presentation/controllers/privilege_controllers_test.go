package controllers_test

import (
	"fmt"
	"testing"

	"github.com/iota-uz/iota-sdk/modules"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/role"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/session"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/core/permissions"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/controllers"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/httpconfig/cookies"
	"github.com/iota-uz/iota-sdk/pkg/itf"
	"github.com/iota-uz/iota-sdk/pkg/rbac"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func persistAdministrativeTestActor(t *testing.T, suite *itf.Suite, actorPermissions ...permission.Permission) {
	t.Helper()
	persistTestUser(t, suite.Env())
	_, err := suite.Env().Pool.Exec(
		suite.Env().Ctx,
		"SELECT setval(pg_get_serial_sequence('users', 'id'), (SELECT MAX(id) FROM users), true)",
	)
	require.NoError(t, err)
	permissionRepository := persistence.NewPermissionRepository()
	for _, candidate := range actorPermissions {
		require.NoError(t, permissionRepository.Save(suite.Env().Ctx, candidate))
	}
	userRepository := persistence.NewUserRepository(persistence.NewUploadRepository())
	persistedActor, err := userRepository.GetByID(suite.Env().Ctx, suite.Env().User.ID())
	require.NoError(t, err)
	require.NoError(t, userRepository.Update(
		suite.Env().Ctx,
		persistedActor.SetPermissions(actorPermissions),
	))
}

func TestAdministrativeControllers_RejectForgedEscalationRequests(t *testing.T) {
	actorPermissions := []permission.Permission{
		permissions.UserRead,
		permissions.UserUpdate,
		permissions.GroupRead,
		permissions.GroupCreate,
		permissions.SessionRead,
		permissions.SessionDelete,
	}
	suite := itf.NewSuiteBuilder(t).
		WithComponents(modules.Components()...).
		AsUser(actorPermissions...).
		Build()
	persistAdministrativeTestActor(t, suite, actorPermissions...)

	permissionRepository := persistence.NewPermissionRepository()
	require.NoError(t, permissionRepository.Save(suite.Env().Ctx, permissions.DepartmentDelete))
	roleRepository := persistence.NewRoleRepository()
	strongRole, err := roleRepository.Create(suite.Env().Ctx, role.New(
		"Hidden strong role",
		role.WithTenantID(suite.Env().TenantID()),
		role.WithPermissions([]permission.Permission{permissions.DepartmentDelete}),
	))
	require.NoError(t, err)

	userRepository := persistence.NewUserRepository(persistence.NewUploadRepository())
	targetEmail, err := internet.NewEmail("controller-target@example.com")
	require.NoError(t, err)
	target, err := userRepository.Create(suite.Env().Ctx, user.New(
		"Controller", "Target", targetEmail, user.UILanguageEN,
		user.WithTenantID(suite.Env().TenantID()),
	))
	require.NoError(t, err)

	suite.Register(controllers.NewUsersController(
		suite.Env().App,
		controllers.WithUserControllerBasePath("/users"),
		controllers.WithUserControllerPermissionSchema(&rbac.PermissionSchema{}),
	))
	suite.Register(controllers.NewGroupsController(suite.Env().App))
	suite.Register(controllers.NewSessionController(
		"/settings/sessions",
		itf.GetService[cookies.Config](suite.Env()),
	))

	t.Run("user role assignment", func(t *testing.T) {
		// Falsely green if the role is merely hidden and no forged POST reaches the backend.
		suite.GET(fmt.Sprintf("/users/%d/edit", target.ID())).Expect(t).
			Status(200).
			NotContains(strongRole.Name())

		response := suite.POST(fmt.Sprintf("/users/%d", target.ID())).
			HTMX().
			FormFields(map[string]interface{}{
				"FirstName": "Changed", "LastName": "Target",
				"Email": targetEmail.Value(), "Language": "en",
				"RoleIDs": strongRole.ID(),
			}).
			Assert(t)
		response.ExpectStatus(403).
			ExpectBodyContains("Changes were not saved").
			ExpectHeaderContains("HX-Trigger", "notify")

		reloaded, reloadErr := userRepository.GetByID(suite.Env().Ctx, target.ID())
		require.NoError(t, reloadErr)
		assert.Empty(t, reloaded.Roles())
		assert.Equal(t, "Controller", reloaded.FirstName())
	})

	t.Run("group role assignment", func(t *testing.T) {
		// Falsely green if the controller silently drops the submitted role ID.
		response := suite.POST("/groups").
			HTMX().
			FormFields(map[string]interface{}{
				"Name": "Forged controller group", "RoleIDs": strongRole.ID(),
			}).
			Assert(t)
		response.ExpectStatus(403).
			ExpectBodyContains("Changes were not saved").
			ExpectHeaderContains("HX-Trigger", "notify")
	})

	t.Run("stronger user session revoke", func(t *testing.T) {
		strongEmail, emailErr := internet.NewEmail("controller-strong@example.com")
		require.NoError(t, emailErr)
		strongTarget, createErr := userRepository.Create(suite.Env().Ctx, user.New(
			"Strong", "Controller", strongEmail, user.UILanguageEN,
			user.WithTenantID(suite.Env().TenantID()),
			user.WithPermissions([]permission.Permission{permissions.DepartmentDelete}),
		))
		require.NoError(t, createErr)
		// Falsely green if the backend is protected but the UI still presents a stronger target as editable.
		suite.GET(fmt.Sprintf("/users/%d/edit", strongTarget.ID())).Expect(t).
			Status(200).
			Contains("authorization-read-only-notice").
			Contains("disabled")
		const token = "controller-strong-session"
		sessionRepository := persistence.NewSessionRepository()
		require.NoError(t, sessionRepository.Create(
			suite.Env().Ctx,
			session.New(token, strongTarget.ID(), suite.Env().TenantID(), "127.0.0.1", "test"),
		))

		// Falsely green if the test revokes a session owned by the actor instead of the stronger target.
		response := suite.DELETE("/settings/sessions/" + token).HTMX().Assert(t)
		response.ExpectStatus(403).
			ExpectBodyContains("Changes were not saved").
			ExpectHeaderContains("HX-Trigger", "notify")
		_, reloadErr := sessionRepository.GetByToken(suite.Env().Ctx, token)
		require.NoError(t, reloadErr)
	})
}

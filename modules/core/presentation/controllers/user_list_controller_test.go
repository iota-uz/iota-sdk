package controllers_test

import (
	"testing"

	"github.com/iota-uz/iota-sdk/modules"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/core/permissions"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/controllers"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/itf"
	"github.com/iota-uz/iota-sdk/pkg/rbac"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Most of these tests deliberately do not create any `users` table rows —
// they only exercise the GET /users → BuildTableConfig → table.ContentHTMX
// wiring, which doesn't depend on row count. TestUsersController_List_LoadsRealUserRow
// below is the one that specifically exercises FindUsers against a real row
// (see its doc comment for why that used to fail under itf's per-test tx).

func registerUsersListController(t *testing.T, suite *itf.Suite) {
	t.Helper()
	controller := controllers.NewUsersController(
		suite.Env().App,
		controllers.WithUserControllerBasePath("/users"),
		controllers.WithUserControllerPermissionSchema(&rbac.PermissionSchema{}),
	)
	suite.Register(controller)
}

// TestUsersController_List_RendersTableAndFilterBuilder verifies the sfui
// table + filterbuilder chip bar wiring: a full page GET /users renders 200
// and includes the filterbuilder chip bar container.
func TestUsersController_List_RendersTableAndFilterBuilder(t *testing.T) {
	t.Parallel()

	suite := itf.NewSuiteBuilder(t).
		WithComponents(modules.Components()...).
		AsUser(permissions.UserRead).
		Build()
	registerUsersListController(t, suite)

	doc := suite.GET("/users").Expect(t).Status(200).HTML()

	doc.Element("//*[@id='users-filters']").Exists()
}

// TestUsersController_List_HidesNewUserButtonWithoutPermission verifies the
// "New user" action (now a single AddActions call site in BuildTableConfig,
// replacing the old desktop+mobile duplicate) stays gated by UserCreate.
func TestUsersController_List_HidesNewUserButtonWithoutPermission(t *testing.T) {
	t.Parallel()

	suite := itf.NewSuiteBuilder(t).
		WithComponents(modules.Components()...).
		AsUser(permissions.UserRead).
		Build()
	registerUsersListController(t, suite)

	resp := suite.GET("/users").Expect(t).Status(200)
	assert.NotContains(t, resp.Body(), `href="/users/new"`)
}

// TestUsersController_List_ShowsNewUserButtonWithPermission is the positive
// counterpart of the above.
func TestUsersController_List_ShowsNewUserButtonWithPermission(t *testing.T) {
	t.Parallel()

	suite := itf.NewSuiteBuilder(t).
		WithComponents(modules.Components()...).
		AsUser(permissions.UserRead, permissions.UserCreate).
		Build()
	registerUsersListController(t, suite)

	resp := suite.GET("/users").Expect(t).Status(200)
	assert.Contains(t, resp.Body(), `href="/users/new"`)
}

// TestUsersController_List_HTMXRowsOnly verifies that an HTMX request
// targeting the table body ("#table-body", the tbody id sfui hardcodes)
// returns 200 without erroring the ContentHTMX dispatch branch.
func TestUsersController_List_HTMXRowsOnly(t *testing.T) {
	t.Parallel()

	suite := itf.NewSuiteBuilder(t).
		WithComponents(modules.Components()...).
		AsUser(permissions.UserRead).
		Build()
	registerUsersListController(t, suite)

	resp := suite.GET("/users").HTMX().HTMXTarget("table-body").Expect(t).Status(200)
	assert.NotContains(t, resp.Body(), "<html")
}

// TestUsersController_List_SearchQueryParamAccepted verifies the Search
// query parameter is round-tripped into the rendered search input (wiring
// check; row-level search filtering is covered at the repository layer).
func TestUsersController_List_SearchQueryParamAccepted(t *testing.T) {
	t.Parallel()

	suite := itf.NewSuiteBuilder(t).
		WithComponents(modules.Components()...).
		AsUser(permissions.UserRead).
		Build()
	registerUsersListController(t, suite)

	doc := suite.GET("/users?Search=someone").Expect(t).Status(200).HTML()
	doc.Element(`//input[@name='Search' and @value='someone']`).Exists()
}

// TestUsersController_List_LoadsRealUserRow exercises FindUsers against an
// actual `users` row. user_query_repository.go's scanAndLoadUsers used to
// call loadUserWithRelations (which issues further tx.Query() calls for
// roles/permissions/groups) while the outer result set's rows were still
// open — safe against a pooled connection (a plain production request with
// no ambient transaction) but not against a single bound pgx.Tx, which is
// exactly what itf's per-test harness uses for rollback isolation: it would
// fail with "conn busy" as soon as FindUsers had to load relations for at
// least one row. scanAndLoadUsers now fully drains the outer cursor into a
// slice before loading any relation, so this must pass with a real row.
func TestUsersController_List_LoadsRealUserRow(t *testing.T) {
	t.Parallel()

	suite := itf.NewSuiteBuilder(t).
		WithComponents(modules.Components()...).
		AsUser(permissions.UserRead).
		Build()
	registerUsersListController(t, suite)

	tenantID, err := composables.UseTenantID(suite.Env().Ctx)
	require.NoError(t, err)

	email, err := internet.NewEmail("real.row@example.com")
	require.NoError(t, err)

	userRepository := persistence.NewUserRepository(persistence.NewUploadRepository())
	_, err = userRepository.Create(
		suite.Env().Ctx,
		user.New(
			"Real",
			"Row",
			email,
			user.UILanguageEN,
			user.WithTenantID(tenantID),
		),
	)
	require.NoError(t, err)

	resp := suite.GET("/users").Expect(t).Status(200)
	assert.Contains(t, resp.Body(), "Real Row")
}

package controllers_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/role"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/core/permissions"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/controllers"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/itf"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
)

type countingTx struct {
	pgx.Tx
	queries *int
}

func (tx countingTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	*tx.queries++
	return tx.Tx.Query(ctx, sql, args...)
}

func (tx countingTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	*tx.queries++
	return tx.Tx.QueryRow(ctx, sql, args...)
}

func TestGroupsController_List_QueryCountDoesNotGrowWithRows(t *testing.T) {
	actorPermissions := []permission.Permission{permissions.GroupRead, permissions.GroupUpdate, permissions.GroupDelete}
	suite := itf.NewSuiteBuilder(t).
		WithComponents(modules.Components()...).
		AsUser(actorPermissions...).
		Build()
	persistAdministrativeTestActor(t, suite, actorPermissions...)
	ctx := suite.Env().Ctx
	tenantID := suite.Env().TenantID()
	actor, err := persistence.NewUserRepository(persistence.NewUploadRepository()).GetByID(ctx, suite.Env().User.ID())
	require.NoError(t, err)
	require.Equal(t, tenantID, actor.TenantID())
	suite.AsUser(actor)
	require.NoError(t, persistence.NewPermissionRepository().Save(ctx, permissions.DepartmentDelete))

	roleRepository := persistence.NewRoleRepository()
	weakRole, err := roleRepository.Create(ctx, role.New("Group list weak role",
		role.WithTenantID(tenantID), role.WithPermissions([]permission.Permission{permissions.GroupRead})))
	require.NoError(t, err)
	strongRole, err := roleRepository.Create(ctx, role.New("Group list strong role",
		role.WithTenantID(tenantID), role.WithPermissions([]permission.Permission{permissions.DepartmentDelete})))
	require.NoError(t, err)

	insertGroup := func(name string, roleID uint) uuid.UUID {
		id := uuid.New()
		_, execErr := suite.Env().Tx.Exec(ctx, `INSERT INTO user_groups (id, type, tenant_id, name, description, created_at, updated_at)
			VALUES ($1, 'user', $2, $3, '', NOW(), NOW())`, id, tenantID, name)
		require.NoError(t, execErr)
		_, execErr = suite.Env().Tx.Exec(ctx, `INSERT INTO group_roles (group_id, role_id) VALUES ($1, $2)`, id, roleID)
		require.NoError(t, execErr)
		_, execErr = suite.Env().Tx.Exec(ctx, `INSERT INTO group_users (group_id, user_id) VALUES ($1, $2)`, id, suite.Env().User.ID())
		require.NoError(t, execErr)
		return id
	}
	weakGroupIDs := make([]uuid.UUID, 0, 12)
	for i := range 12 {
		weakGroupIDs = append(weakGroupIDs, insertGroup(fmt.Sprintf("Group list weak %02d", i), weakRole.ID()))
	}
	strongGroupID := insertGroup("Group list strong", strongRole.ID())

	queries := 0
	suite.WithMiddleware(func(ctx context.Context, _ *http.Request) context.Context {
		tx, txErr := composables.UseTx(ctx)
		require.NoError(t, txErr)
		pgxTx, ok := tx.(pgx.Tx)
		require.True(t, ok)
		return composables.WithTx(ctx, countingTx{Tx: pgxTx, queries: &queries})
	})
	suite.Register(controllers.NewGroupsController(suite.Env().App))

	measure := func(limit int) (int, string) {
		queries = 0
		body := suite.GET(fmt.Sprintf("/groups?limit=%d", limit)).HTMX().Expect(t).Status(http.StatusOK).Body()
		return queries, body
	}

	// Falsely green if both pages render the same number of rows or the counting tx is bypassed.
	oneRowQueries, _ := measure(1)
	allRowsQueries, body := measure(50)
	require.Positive(t, oneRowQueries)
	require.Equal(t, oneRowQueries, allRowsQueries)

	// Falsely green if management actions are granted to every row instead of evaluated per group.
	for _, id := range weakGroupIDs {
		require.Contains(t, body, fmt.Sprintf(`hx-get="/groups/%s"`, id))
	}
	require.Contains(t, body, fmt.Sprintf(`id="group-%s"`, strongGroupID))
	require.NotContains(t, body, fmt.Sprintf(`hx-get="/groups/%s"`, strongGroupID))
}

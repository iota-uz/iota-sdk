package automations_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/modules"
	"github.com/iota-uz/iota-sdk/modules/core"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/group"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/core/permissions"
	"github.com/iota-uz/iota-sdk/pkg/automation/builtin/actions"
	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/host"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/itf"
)

// The core actions run the user and group services as the run-as user: committed data, the services'
// own permission checks, a business-history record of an automation.
func TestCoreAutomationActions(t *testing.T) {
	h := itf.NewHarness(t, itf.HarnessConfig{
		Name: "core_automation_actions", Components: modules.Components(),
		Isolation: itf.IsolationConfig{Mode: itf.IsolationCommitted},
	})
	s := h.Scope(t)
	ctx := composables.WithPool(composables.WithTenantID(context.Background(), s.Tenant.ID), s.Pool)
	perms := []permission.Permission{permissions.UserRead, permissions.UserUpdateBlockStatus, permissions.GroupRead, permissions.GroupUpdate}
	permRepo := persistence.NewPermissionRepository()
	for _, p := range perms {
		require.NoError(t, permRepo.Save(ctx, p))
	}
	users := persistence.NewUserRepository(persistence.NewUploadRepository())
	newUser := func(name string, ps ...permission.Permission) user.User {
		email, err := internet.NewEmail(name + "-" + uuid.NewString()[:8] + "@example.test")
		require.NoError(t, err)
		u, err := users.Create(ctx, user.New(name, "Test", email, user.UILanguageEN, user.WithTenantID(s.Tenant.ID), user.WithPermissions(ps)))
		require.NoError(t, err)
		return u
	}
	admin := newUser("admin", perms...)
	target := newUser("target")
	plain := newUser("plain")

	list, err := actions.Contributed(s.Container)
	require.NoError(t, err)
	nodes, err := actions.Nodes(list...)
	require.NoError(t, err)
	byType := map[string]catalog.CallNode{}
	for _, n := range nodes {
		byType[n.Spec().Type] = n.(catalog.CallNode)
	}
	subject := func(u user.User) spi.Subject {
		return spi.Subject{TenantID: s.Tenant.ID, UserID: int64(u.ID()), Kind: spi.SubjectOwner, Permissions: host.PermissionNames(user.EffectivePermissions(u))}
	}
	in := func(as user.User, args map[string]any) catalog.CallInput {
		return catalog.CallInput{TenantID: s.Tenant.ID, IdempotencyKey: "run:" + uuid.NewString(), Params: map[string]any{"args": args},
			RunAs: subject(as), Deadline: time.Now().Add(time.Minute)}
	}

	block := byType[actions.TypeOf(core.ActionUserBlock)]
	require.NotNil(t, block)
	// Without the right nothing happens.
	_, err = block.Execute(context.Background(), in(plain, map[string]any{"user_id": int(target.ID()), "reason": "left the company"}))
	var ce *catalog.CallError
	require.ErrorAs(t, err, &ce)
	assert.Equal(t, actions.CodeForbidden, ce.Code)
	got, err := users.GetByID(ctx, target.ID())
	require.NoError(t, err)
	assert.False(t, got.IsBlocked())

	res, err := block.Execute(context.Background(), in(admin, map[string]any{"user_id": int(target.ID()), "reason": "left the company"}))
	require.NoError(t, err)
	assert.Equal(t, "done", res.Outcome)
	assert.Equal(t, true, res.Output["blocked"])
	got, err = users.GetByID(ctx, target.ID())
	require.NoError(t, err)
	assert.True(t, got.IsBlocked())
	var recorded int
	require.NoError(t, s.Pool.QueryRow(ctx, `SELECT count(*) FROM action_logs WHERE tenant_id = $1 AND event_type = 'core.user.blocked' AND actor_kind = 'service'`, s.Tenant.ID).Scan(&recorded))
	assert.Equal(t, 1, recorded, "the business history records the block")

	// A refusal of the service is the action's failure, not an unknown outcome.
	_, err = block.Execute(context.Background(), in(admin, map[string]any{"user_id": int(target.ID()), "reason": "again"}))
	require.ErrorAs(t, err, &ce)
	assert.Equal(t, actions.CodeActionFailed, ce.Code)

	res, err = byType[actions.TypeOf(core.ActionUserUnblock)].Execute(context.Background(), in(admin, map[string]any{"user_id": int(target.ID())}))
	require.NoError(t, err)
	assert.Equal(t, false, res.Output["blocked"])

	groups := persistence.NewGroupRepository(users, persistence.NewRoleRepository())
	g, err := groups.Save(composables.WithUser(ctx, admin), group.New("Robots "+uuid.NewString()[:8], group.WithTenantID(s.Tenant.ID)))
	require.NoError(t, err)
	res, err = byType[actions.TypeOf(core.ActionGroupAddUser)].Execute(context.Background(), in(admin, map[string]any{"group_id": g.ID().String(), "user_id": int(target.ID())}))
	require.NoError(t, err)
	assert.InDelta(t, 1, res.Output["members"], 0)
	res, err = byType[actions.TypeOf(core.ActionGroupRemoveUser)].Execute(context.Background(), in(admin, map[string]any{"group_id": g.ID().String(), "user_id": int(target.ID())}))
	require.NoError(t, err)
	assert.InDelta(t, 0, res.Output["members"], 0)
}

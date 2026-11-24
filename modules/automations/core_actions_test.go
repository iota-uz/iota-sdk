package automations_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/modules"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/group"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/core/permissions"
	coreservices "github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/pkg/automation/builtin/actions"
	"github.com/iota-uz/iota-sdk/pkg/automation/builtin/actions/actionstest"
	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/automation/engine"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/host"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/native"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/pool"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/runtime"
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

	// The application's native set and its command host, as the extension runtime's wiring builds them:
	// the core actions are registered once, as native commands (actions.Native).
	set, err := native.Compose(modules.Natives()...)
	require.NoError(t, err)
	nodes, err := actions.NodesOf(set)
	require.NoError(t, err)
	byType := map[string]catalog.CallNode{}
	for _, n := range nodes {
		byType[n.Spec().Type] = n.(catalog.CallNode)
	}
	cmds, _, err := actionstest.Host(s.Pool, set, s.Container, runtime.HistoryExecution(users))
	require.NoError(t, err)
	subject := func(u user.User) spi.Subject {
		return spi.Subject{TenantID: s.Tenant.ID, UserID: int64(u.ID()), Kind: spi.SubjectOwner, Permissions: host.PermissionNames(user.EffectivePermissions(u))}
	}
	// exec is a call of the node as the engine makes it: Command, the command through the host as the
	// run-as with the call's invocation ID, CommandResult; a domain refusal is the failed outcome's code.
	exec := func(n catalog.CallNode, as user.User, args map[string]any) (catalog.CallResult, error) {
		in := catalog.CallInput{TenantID: s.Tenant.ID, IdempotencyKey: "run:" + uuid.NewString(), Params: map[string]any{"args": args},
			RunAs: subject(as), Deadline: time.Now().Add(time.Minute)}
		cc := n.(catalog.CommandCall)
		command, input, err := cc.Command(in)
		if err != nil {
			return catalog.CallResult{}, err
		}
		res, err := cmds.Invoke(context.Background(), host.Request{Contract: command, Input: input, Subject: subject(as),
			InvocationID: catalog.InvocationIDOf(in.IdempotencyKey), Trigger: host.Trigger{Kind: host.TriggerWorkflow}})
		if err != nil {
			if we, ok := pool.AsWireError(err); ok && (we.Kind == pool.ErrKindDomain || we.Kind == pool.ErrKindValidation) {
				return catalog.CallResult{}, &catalog.CallError{Code: engine.CommandErrorCode(we.Code), Message: we.Message}
			}
			return catalog.CallResult{}, err
		}
		return cc.CommandResult(in, res.Output)
	}

	block := byType[actions.TypeOf(coreservices.ActionUserBlock)]
	require.NotNil(t, block)
	// Without the right nothing happens.
	_, err = exec(block, plain, map[string]any{"user_id": int(target.ID()), "reason": "left the company"})
	var ce *catalog.CallError
	require.ErrorAs(t, err, &ce)
	assert.Equal(t, actions.CodeForbidden, ce.Code)
	got, err := users.GetByID(ctx, target.ID())
	require.NoError(t, err)
	assert.False(t, got.IsBlocked())

	res, err := exec(block, admin, map[string]any{"user_id": int(target.ID()), "reason": "left the company"})
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
	_, err = exec(block, admin, map[string]any{"user_id": int(target.ID()), "reason": "again"})
	require.ErrorAs(t, err, &ce)
	assert.Equal(t, actions.CodeActionFailed, ce.Code)

	res, err = exec(byType[actions.TypeOf(coreservices.ActionUserUnblock)], admin, map[string]any{"user_id": int(target.ID())})
	require.NoError(t, err)
	assert.Equal(t, false, res.Output["blocked"])

	groups := persistence.NewGroupRepository(users, persistence.NewRoleRepository())
	g, err := groups.Save(composables.WithUser(ctx, admin), group.New("Robots "+uuid.NewString()[:8], group.WithTenantID(s.Tenant.ID)))
	require.NoError(t, err)
	res, err = exec(byType[actions.TypeOf(coreservices.ActionGroupAddUser)], admin, map[string]any{"group_id": g.ID().String(), "user_id": int(target.ID())})
	require.NoError(t, err)
	assert.InDelta(t, 1, res.Output["members"], 0)
	res, err = exec(byType[actions.TypeOf(coreservices.ActionGroupRemoveUser)], admin, map[string]any{"group_id": g.ID().String(), "user_id": int(target.ID())})
	require.NoError(t, err)
	assert.InDelta(t, 0, res.Output["members"], 0)
}

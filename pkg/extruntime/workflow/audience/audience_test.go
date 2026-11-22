package audience_test

import (
	"context"
	"os"
	"strconv"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/workflow"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/workflow/audience"
	"github.com/iota-uz/iota-sdk/pkg/itf"
)

// The migrator reads ./migrations relative to the working directory.
func TestMain(m *testing.M) {
	if err := os.Chdir("../../../.."); err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}

type fixture struct {
	t      *testing.T
	db     *pgxpool.Pool
	tenant uuid.UUID
}

func (f fixture) exec(sql string, args ...any) {
	f.t.Helper()
	_, err := f.db.Exec(context.Background(), sql, args...)
	require.NoError(f.t, err)
}

func (f fixture) user(tenant uuid.UUID, name string) int64 {
	f.t.Helper()
	var id int64
	require.NoError(f.t, f.db.QueryRow(context.Background(), `
		INSERT INTO users (first_name, last_name, email, ui_language, type, tenant_id) VALUES ($1, 'T', $2, 'en', 'user', $3) RETURNING id`,
		name, name+"-"+uuid.NewString()[:8]+"@x.test", tenant).Scan(&id))
	return id
}

func (f fixture) role(tenant uuid.UUID, name string) int64 {
	f.t.Helper()
	var id int64
	require.NoError(f.t, f.db.QueryRow(context.Background(), `INSERT INTO roles (name, type, tenant_id) VALUES ($1, 'user', $2) RETURNING id`, name, tenant).Scan(&id))
	return id
}

func (f fixture) group(tenant uuid.UUID, name string) uuid.UUID {
	f.t.Helper()
	var id uuid.UUID
	require.NoError(f.t, f.db.QueryRow(context.Background(), `INSERT INTO user_groups (name, type, tenant_id) VALUES ($1, 'user', $2) RETURNING id`, name, tenant).Scan(&id))
	return id
}

func (f fixture) newTenant() uuid.UUID {
	f.t.Helper()
	id := uuid.New()
	f.exec(`INSERT INTO tenants (id, name, domain, created_at, updated_at) VALUES ($1, $2, $3, NOW(), NOW())`, id, "t-"+id.String()[:8], id.String()[:8]+".test")
	return id
}

func TestCoreAudience(t *testing.T) {
	h := itf.NewHarness(t, itf.HarnessConfig{
		Name:      "extruntime_audience",
		Database:  itf.DatabaseConfig{Provisioning: itf.ProvisioningPerTestDatabase},
		Isolation: itf.IsolationConfig{Mode: itf.IsolationCommitted},
	})
	scope := h.Scope(t)
	f := fixture{t: t, db: scope.Pool, tenant: scope.Tenant.ID}
	ctx := context.Background()
	aud := audience.New(f.db)

	direct, viaGroup, grouped, outsider, blocked := f.user(f.tenant, "direct"), f.user(f.tenant, "viagroup"), f.user(f.tenant, "grouped"), f.user(f.tenant, "outsider"), f.user(f.tenant, "blocked")
	approvers := f.role(f.tenant, "approvers")
	committee := f.group(f.tenant, "committee")
	f.exec(`INSERT INTO user_roles (user_id, role_id) VALUES ($1, $2), ($3, $2)`, direct, approvers, blocked)
	f.exec(`INSERT INTO group_roles (group_id, role_id) VALUES ($1, $2)`, committee, approvers)
	f.exec(`INSERT INTO group_users (group_id, user_id) VALUES ($1, $2), ($1, $3)`, committee, viaGroup, grouped)
	f.exec(`UPDATE users SET is_blocked = true, block_reason = 'gone', blocked_at = now(), blocked_by = $1, blocked_by_tenant_id = tenant_id WHERE id = $2`, direct, blocked)
	var perm uuid.UUID
	require.NoError(t, f.db.QueryRow(ctx, `INSERT INTO permissions (name, resource, action, modifier) VALUES ('acme.approve', 'acme', 'approve', 'all') RETURNING id`).Scan(&perm))
	f.exec(`INSERT INTO role_permissions (role_id, permission_id) VALUES ($1, $2)`, approvers, perm)

	role := workflow.TaskAssignee{Kind: workflow.TaskAssignRole, Ref: strconv.FormatInt(approvers, 10)}
	group := workflow.TaskAssignee{Kind: workflow.TaskAssignGroup, Ref: committee.String()}
	permission := workflow.TaskAssignee{Kind: workflow.TaskAssignPermission, Ref: "acme.approve"}
	member := func(user int64, a workflow.TaskAssignee) bool {
		ok, err := aud.IsMember(ctx, f.tenant, user, a)
		require.NoError(t, err)
		return ok
	}
	require.True(t, member(direct, role), "a direct role")
	require.True(t, member(viaGroup, role), "a role through a group")
	require.False(t, member(outsider, role))
	require.False(t, member(blocked, role), "a blocked user is nobody")
	require.True(t, member(grouped, group))
	require.False(t, member(direct, group))
	require.True(t, member(viaGroup, permission), "a permission through a group's role")
	require.False(t, member(outsider, permission))

	users, err := aud.Users(ctx, f.tenant, role, 10)
	require.NoError(t, err)
	require.ElementsMatch(t, []int64{direct, viaGroup, grouped}, users)
	users, err = aud.Users(ctx, f.tenant, role, 1)
	require.NoError(t, err)
	require.Len(t, users, 1)

	// Membership is current: leaving the group ends it.
	f.exec(`DELETE FROM group_users WHERE group_id = $1 AND user_id = $2`, committee, grouped)
	require.False(t, member(grouped, group))
	require.False(t, member(grouped, role))

	// A role or group of another tenant covers nobody here, and a user of
	// another tenant is no member of this tenant's role.
	other := f.newTenant()
	foreigner := f.user(other, "foreigner")
	f.exec(`INSERT INTO user_roles (user_id, role_id) VALUES ($1, $2)`, foreigner, approvers)
	require.False(t, member(foreigner, role))
	ok, err := aud.IsMember(ctx, other, foreigner, role)
	require.NoError(t, err)
	require.False(t, ok, "the role belongs to another tenant")
	require.False(t, member(direct, workflow.TaskAssignee{Kind: workflow.TaskAssignGroup, Ref: f.group(other, "theirs").String()}))
	require.False(t, member(direct, workflow.TaskAssignee{Kind: workflow.TaskAssignRole, Ref: "not-a-number"}))
}

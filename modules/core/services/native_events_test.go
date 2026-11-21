package services_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/group"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/role"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/core/permissions"
	"github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/composition"
	"github.com/iota-uz/iota-sdk/pkg/constants"
	"github.com/iota-uz/iota-sdk/pkg/eventbus"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/events"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/itf"
)

// eventEnv is a tenant with an administrator who holds every core permission,
// so the privilege policy lets every change of these tests through.
type eventEnv struct {
	root     *testing.T // owns the environment; subtests commit through it
	f        *itf.TestEnvironment
	tenantID uuid.UUID
	actor    user.User
	users    services.UserService
	roles    services.RoleService
	groups   services.GroupService
	userRepo user.Repository
}

func newEventEnv(t *testing.T) *eventEnv {
	t.Helper()
	f := setupTest(t)
	tenantID := f.TenantID()
	permissionRepository := persistence.NewPermissionRepository()
	for _, p := range permissions.Permissions {
		require.NoError(t, permissionRepository.Save(f.Ctx, p))
	}
	userRepo := persistence.NewUserRepository(persistence.NewUploadRepository())
	actor, err := userRepo.Create(f.Ctx, user.New("Event", "Admin", internet.MustParseEmail("admin-"+uuid.NewString()+"@example.com"), user.UILanguageEN,
		user.WithTenantID(tenantID), user.WithPermissions(permissions.Permissions)))
	require.NoError(t, err)
	return &eventEnv{
		root: t, f: f, tenantID: tenantID, actor: actor, userRepo: userRepo,
		users:  *itf.GetService[services.UserService](f),
		roles:  *itf.GetService[services.RoleService](f),
		groups: *itf.GetService[services.GroupService](f),
	}
}

// newUser stores a user without events (straight through the repository).
func (e *eventEnv) newUser(t *testing.T, name string, opts ...user.Option) user.User {
	t.Helper()
	u, err := e.userRepo.Create(e.f.Ctx, user.New(name, "Person", internet.MustParseEmail(name+"-"+uuid.NewString()+"@example.com"), user.UILanguageEN,
		append([]user.Option{user.WithTenantID(e.tenantID)}, opts...)...))
	require.NoError(t, err)
	return u
}

func (e *eventEnv) newRole(t *testing.T, name string, perms ...permission.Permission) role.Role {
	t.Helper()
	r, err := e.roles.Create(e.ctx(), role.New(name+"-"+uuid.NewString(), role.WithPermissions(perms)))
	require.NoError(t, err)
	return r
}

func (e *eventEnv) newGroup(t *testing.T, name string, opts ...group.Option) group.Group {
	t.Helper()
	g, err := e.groups.Create(e.ctx(), group.New(name+"-"+uuid.NewString(), append([]group.Option{group.WithTenantID(e.tenantID)}, opts...)...))
	require.NoError(t, err)
	return g
}

// ctx acts as the actor in the test's current scope transaction (itf commits
// and renews it, so it is rebuilt on every use).
func (e *eventEnv) ctx() context.Context { return composables.WithUser(e.f.Ctx, e.actor) }

func outboxRows(t *testing.T, ctx context.Context, tenantID uuid.UUID, event string) []outboxRow {
	t.Helper()
	return itf.OutboxRowsIn(t, ctx, tenantID, event)
}

// eventCounts is the number of outbox rows of each event in the scope transaction.
func (e *eventEnv) eventCounts(t *testing.T, ctx context.Context, ids ...string) map[string]int {
	t.Helper()
	out := make(map[string]int, len(ids))
	for _, id := range ids {
		out[id] = len(itf.OutboxRowsIn(t, ctx, e.tenantID, id))
	}
	return out
}

// commitsAndRollsBack shows that the events of change commit with it and roll
// back with it, in real transactions on separate connections
// (itf.TestEnvironment.OutboxCommitsAndRollsBack), and that the acting user is
// their initiator.
func (e *eventEnv) commitsAndRollsBack(t *testing.T, ids []string, want map[string]int, domain itf.Domain, change func(ctx context.Context, suffix string) error) map[string][]outboxRow {
	t.Helper()
	e.f.CommitTx(e.root)
	got := e.f.OutboxCommitsAndRollsBack(t, e.ctx(), e.tenantID, ids, want, domain, change)
	for id, rows := range got {
		for _, r := range rows {
			require.NotNil(t, r.Initiator, "%s: the acting user is the event initiator", id)
			require.EqualValues(t, e.actor.ID(), *r.Initiator)
		}
	}
	return got
}

func payloadOf[P any](t *testing.T, r outboxRow) P {
	t.Helper()
	var p P
	require.NoError(t, json.Unmarshal(r.Payload, &p))
	return p
}

func TestUserEvents_CommitAndRollBackWithTheChange(t *testing.T) {
	e := newEventEnv(t)

	t.Run("updated", func(t *testing.T) {
		target := e.newUser(t, "Before")
		got := e.commitsAndRollsBack(t, []string{services.UserUpdatedEventID}, map[string]int{services.UserUpdatedEventID: 1}, itf.Domain{SQL: "SELECT first_name FROM users WHERE id = $1", Args: []any{target.ID()}}, func(ctx context.Context, suffix string) error {
			_, err := e.users.Update(ctx, target.SetName("After-"+suffix, "Renamed", ""))
			return err
		})
		row := got[services.UserUpdatedEventID][0]
		require.Equal(t, "core.user", row.SubjectEntity)
		require.Equal(t, fmt.Sprint(target.ID()), row.SubjectKey)
		require.JSONEq(t, fmt.Sprintf(`{"userId":"%d","firstName":"After-committed","lastName":"Renamed","changed":["firstName","lastName"]}`, target.ID()), string(row.Payload))
	})

	t.Run("updated_by_the_user", func(t *testing.T) {
		self := e.newUser(t, "Self", user.WithPermissions([]permission.Permission{permissions.UserUpdate}))
		selfCtx := composables.WithUser(e.f.Ctx, self)
		changed, err := e.users.UpdateSelf(selfCtx, self.SetName("Selfie", "Person", ""))
		require.NoError(t, err)
		require.Equal(t, "Selfie", changed.FirstName())
		rows := outboxRows(t, e.ctx(), e.tenantID, services.UserUpdatedEventID)
		last := rows[len(rows)-1]
		require.Equal(t, fmt.Sprint(self.ID()), last.SubjectKey)
		require.NotNil(t, last.Initiator, "the user who edits themselves is the initiator")
		require.EqualValues(t, self.ID(), *last.Initiator)
	})

	t.Run("blocked", func(t *testing.T) {
		target := e.newUser(t, "Block")
		got := e.commitsAndRollsBack(t, []string{services.UserBlockedEventID}, map[string]int{services.UserBlockedEventID: 1}, itf.Domain{SQL: "SELECT is_blocked::text FROM users WHERE id = $1", Args: []any{target.ID()}}, func(ctx context.Context, _ string) error {
			_, err := e.users.BlockUser(ctx, target.ID(), "Policy violation")
			return err
		})
		require.JSONEq(t, fmt.Sprintf(`{"userId":"%d","blockReason":"Policy violation"}`, target.ID()), string(got[services.UserBlockedEventID][0].Payload))
	})

	t.Run("unblocked", func(t *testing.T) {
		target := e.newUser(t, "Unblock")
		_, err := e.users.BlockUser(e.ctx(), target.ID(), "Policy violation")
		require.NoError(t, err)
		got := e.commitsAndRollsBack(t, []string{services.UserUnblockedEventID}, map[string]int{services.UserUnblockedEventID: 1}, itf.Domain{SQL: "SELECT is_blocked::text FROM users WHERE id = $1", Args: []any{target.ID()}}, func(ctx context.Context, _ string) error {
			_, err := e.users.UnblockUser(ctx, target.ID())
			return err
		})
		require.JSONEq(t, fmt.Sprintf(`{"userId":"%d"}`, target.ID()), string(got[services.UserUnblockedEventID][0].Payload))
	})

	t.Run("deleted", func(t *testing.T) {
		target := e.newUser(t, "Delete")
		got := e.commitsAndRollsBack(t, []string{services.UserDeletedEventID}, map[string]int{services.UserDeletedEventID: 1}, itf.Domain{SQL: "SELECT count(*)::text FROM users WHERE id = $1", Args: []any{target.ID()}}, func(ctx context.Context, _ string) error {
			_, err := e.users.Delete(ctx, target.ID())
			return err
		})
		require.JSONEq(t, fmt.Sprintf(`{"userId":"%d","firstName":"Delete","lastName":"Person"}`, target.ID()), string(got[services.UserDeletedEventID][0].Payload))
	})

	t.Run("role_assigned", func(t *testing.T) {
		target := e.newUser(t, "Roles")
		r1 := e.newRole(t, "first", permissions.UserRead)
		r2 := e.newRole(t, "second", permissions.UserRead)
		got := e.commitsAndRollsBack(t, []string{services.UserRoleAssignedEventID}, map[string]int{services.UserRoleAssignedEventID: 2}, itf.Domain{SQL: "SELECT count(*)::text FROM user_roles WHERE user_id = $1", Args: []any{target.ID()}}, func(ctx context.Context, _ string) error {
			_, err := e.users.Update(ctx, target.SetRoles([]role.Role{r1, r2}))
			return err
		})
		rows := got[services.UserRoleAssignedEventID]
		roleIDs := map[int64]bool{}
		for _, row := range rows {
			require.Equal(t, fmt.Sprint(target.ID()), row.SubjectKey)
			p := payloadOf[services.UserRoleAssignedPayload](t, row)
			require.EqualValues(t, target.ID(), p.UserID)
			roleIDs[p.RoleID] = true
		}
		require.Equal(t, map[int64]bool{int64(r1.ID()): true, int64(r2.ID()): true}, roleIDs)

		// A role the user already has is no change.
		before := len(outboxRows(t, e.ctx(), e.tenantID, services.UserRoleAssignedEventID))
		_, err := e.users.Update(e.ctx(), target.SetRoles([]role.Role{r1, r2}))
		require.NoError(t, err)
		require.Len(t, outboxRows(t, e.ctx(), e.tenantID, services.UserRoleAssignedEventID), before)
	})
}

func TestUserEvents_NoChangeNoEvent(t *testing.T) {
	e := newEventEnv(t)
	target := e.newUser(t, "Same")
	before := e.eventCounts(t, e.ctx(), services.UserUpdatedEventID)
	_, err := e.users.Update(e.ctx(), target)
	require.NoError(t, err)
	require.Equal(t, before, e.eventCounts(t, e.ctx(), services.UserUpdatedEventID), "an update that changes nothing publishes nothing")
}

func TestRoleEvents_CommitAndRollBackWithTheChange(t *testing.T) {
	e := newEventEnv(t)
	ids := []string{services.RoleCreatedEventID, services.RolePermissionGrantedEventID}

	t.Run("created", func(t *testing.T) {
		got := e.commitsAndRollsBack(t, ids, map[string]int{services.RoleCreatedEventID: 1, services.RolePermissionGrantedEventID: 2}, itf.Domain{SQL: "SELECT count(*)::text FROM roles WHERE tenant_id = $1", Args: []any{e.tenantID}}, func(ctx context.Context, suffix string) error {
			_, err := e.roles.Create(ctx, role.New("Support-"+suffix, role.WithPermissions([]permission.Permission{permissions.UserRead, permissions.GroupRead})))
			return err
		})
		created := got[services.RoleCreatedEventID][0]
		p := payloadOf[services.RoleCreatedPayload](t, created)
		require.Equal(t, "Support-committed", p.Name)
		require.Equal(t, "core.role", created.SubjectEntity)
		require.Equal(t, fmt.Sprint(p.RoleID), created.SubjectKey)
		granted := map[string]bool{}
		for _, row := range got[services.RolePermissionGrantedEventID] {
			gp := payloadOf[services.RolePermissionGrantedPayload](t, row)
			require.Equal(t, p.RoleID, gp.RoleID)
			granted[gp.Permission] = true
		}
		require.Equal(t, map[string]bool{"User.Read": true, "Group.Read": true}, granted)
	})

	t.Run("permission_granted", func(t *testing.T) {
		r := e.newRole(t, "grow", permissions.UserRead)
		got := e.commitsAndRollsBack(t, ids, map[string]int{services.RolePermissionGrantedEventID: 1}, itf.Domain{SQL: "SELECT count(*)::text FROM role_permissions WHERE role_id = $1", Args: []any{r.ID()}}, func(ctx context.Context, _ string) error {
			return e.roles.Update(ctx, r.SetPermissions([]permission.Permission{permissions.UserRead, permissions.RoleRead}))
		})
		require.JSONEq(t, fmt.Sprintf(`{"roleId":"%d","permission":"Role.Read"}`, r.ID()), string(got[services.RolePermissionGrantedEventID][0].Payload))

		// Removing a permission or saving the role unchanged grants nothing.
		before := e.eventCounts(t, e.ctx(), ids...)
		require.NoError(t, e.roles.Update(e.ctx(), r.SetPermissions([]permission.Permission{permissions.UserRead})))
		require.Equal(t, before, e.eventCounts(t, e.ctx(), ids...))
	})
}

func TestGroupEvents_CommitAndRollBackWithTheChange(t *testing.T) {
	e := newEventEnv(t)
	ids := []string{services.GroupCreatedEventID, services.GroupMemberAddedEventID, services.GroupMemberRemovedEventID}

	t.Run("created", func(t *testing.T) {
		member := e.newUser(t, "Founder")
		got := e.commitsAndRollsBack(t, ids, map[string]int{services.GroupCreatedEventID: 1, services.GroupMemberAddedEventID: 1}, itf.Domain{SQL: "SELECT count(*)::text FROM user_groups WHERE tenant_id = $1", Args: []any{e.tenantID}}, func(ctx context.Context, suffix string) error {
			_, err := e.groups.Create(ctx, group.New("Sales-"+suffix, group.WithTenantID(e.tenantID), group.WithUsers([]user.User{member})))
			return err
		})
		created := got[services.GroupCreatedEventID][0]
		p := payloadOf[services.GroupCreatedPayload](t, created)
		require.Equal(t, "Sales-committed", p.Name)
		require.Equal(t, "core.group", created.SubjectEntity)
		require.Equal(t, p.GroupID.String(), created.SubjectKey)
		added := payloadOf[services.GroupMemberPayload](t, got[services.GroupMemberAddedEventID][0])
		require.Equal(t, services.GroupMemberPayload{GroupID: p.GroupID, UserID: int64(member.ID()), GroupName: "Sales-committed"}, added)
	})

	t.Run("member_added_and_removed", func(t *testing.T) {
		g := e.newGroup(t, "Team")
		member := e.newUser(t, "Member")
		got := e.commitsAndRollsBack(t, ids, map[string]int{services.GroupMemberAddedEventID: 1}, itf.Domain{SQL: "SELECT count(*)::text FROM group_users WHERE group_id = $1", Args: []any{g.ID()}}, func(ctx context.Context, _ string) error {
			_, err := e.groups.AddUser(ctx, g.ID(), member)
			return err
		})
		row := got[services.GroupMemberAddedEventID][0]
		require.Equal(t, "core.group", row.SubjectEntity)
		require.Equal(t, g.ID().String(), row.SubjectKey)
		require.Equal(t, services.GroupMemberPayload{GroupID: g.ID(), UserID: int64(member.ID()), GroupName: g.Name()}, payloadOf[services.GroupMemberPayload](t, row))

		// Adding the member again is no change.
		before := e.eventCounts(t, e.ctx(), ids...)
		_, _ = e.groups.AddUser(e.ctx(), g.ID(), member)
		require.Equal(t, before, e.eventCounts(t, e.ctx(), ids...), "a member who already belongs to the group is not added again")

		got = e.commitsAndRollsBack(t, ids, map[string]int{services.GroupMemberRemovedEventID: 1}, itf.Domain{SQL: "SELECT count(*)::text FROM group_users WHERE group_id = $1", Args: []any{g.ID()}}, func(ctx context.Context, _ string) error {
			_, err := e.groups.RemoveUser(ctx, g.ID(), member)
			return err
		})
		require.Equal(t, services.GroupMemberPayload{GroupID: g.ID(), UserID: int64(member.ID()), GroupName: g.Name()},
			payloadOf[services.GroupMemberPayload](t, got[services.GroupMemberRemovedEventID][0]))
	})

	t.Run("group_update", func(t *testing.T) {
		member := e.newUser(t, "Joiner")
		g := e.newGroup(t, "Edited")
		got := e.commitsAndRollsBack(t, ids, map[string]int{services.GroupMemberAddedEventID: 1}, itf.Domain{SQL: "SELECT name FROM user_groups WHERE id = $1", Args: []any{g.ID()}}, func(ctx context.Context, _ string) error {
			_, err := e.groups.Update(ctx, g.SetName("Edited-name").AddUser(member))
			return err
		})
		require.EqualValues(t, member.ID(), payloadOf[services.GroupMemberPayload](t, got[services.GroupMemberAddedEventID][0]).UserID)
	})
}

// A group membership changed on the user form is published like one changed on
// the group, once.
func TestGroupEvents_MembershipChangedOnTheUserForm(t *testing.T) {
	e := newEventEnv(t)
	ids := []string{services.UserUpdatedEventID, services.GroupMemberAddedEventID, services.GroupMemberRemovedEventID}
	g := e.newGroup(t, "Form")
	other := e.newGroup(t, "Other")
	target := e.newUser(t, "Formed", user.WithGroupIDs([]uuid.UUID{other.ID()}))

	got := e.commitsAndRollsBack(t, ids, map[string]int{
		services.UserUpdatedEventID: 1, services.GroupMemberAddedEventID: 1, services.GroupMemberRemovedEventID: 1,
	}, itf.Domain{SQL: "SELECT first_name FROM users WHERE id = $1", Args: []any{target.ID()}}, func(ctx context.Context, suffix string) error {
		_, err := e.users.Update(ctx, target.SetName("Renamed-"+suffix, "Person", "").SetGroupIDs([]uuid.UUID{g.ID()}))
		return err
	})
	added := got[services.GroupMemberAddedEventID][0]
	require.Equal(t, "core.group", added.SubjectEntity)
	require.Equal(t, g.ID().String(), added.SubjectKey)
	require.Equal(t, services.GroupMemberPayload{GroupID: g.ID(), UserID: int64(target.ID()), GroupName: g.Name()}, payloadOf[services.GroupMemberPayload](t, added))
	removed := got[services.GroupMemberRemovedEventID][0]
	require.Equal(t, services.GroupMemberPayload{GroupID: other.ID(), UserID: int64(target.ID()), GroupName: other.Name()}, payloadOf[services.GroupMemberPayload](t, removed))
	require.Equal(t, []string{"firstName"}, payloadOf[services.UserUpdatedPayload](t, got[services.UserUpdatedEventID][0]).Changed)

	// Creating a user with a group and a role publishes both.
	role1 := e.newRole(t, "init", permissions.UserRead)
	ids = append(ids, services.UserRoleAssignedEventID, services.UserCreatedEventID)
	e.commitsAndRollsBack(t, ids, map[string]int{
		services.UserCreatedEventID: 1, services.GroupMemberAddedEventID: 1, services.UserRoleAssignedEventID: 1,
	}, itf.Domain{SQL: "SELECT count(*)::text FROM users WHERE tenant_id = $1", Args: []any{e.tenantID}}, func(ctx context.Context, suffix string) error {
		_, err := e.users.Create(ctx, user.New("New", "Person", internet.MustParseEmail("new-"+suffix+"@example.com"), user.UILanguageEN,
			user.WithTenantID(e.tenantID), user.WithGroupIDs([]uuid.UUID{g.ID()}), user.WithRoles([]role.Role{role1})))
		return err
	})
}

// An event without a tenant, or outside the transaction of a change, is an
// error, so the change rolls back instead of publishing into the wrong tenant.
func TestPublishInTx_RefusesAnEmptyTenantAndAMissingTransaction(t *testing.T) {
	e := newEventEnv(t)
	payload := services.UserDeletedPayload{UserID: 1, FirstName: "A", LastName: "B"}
	ref := &spi.ObjectRef{Entity: "core.user", Key: "1"}

	err := events.PublishInTx(e.ctx(), uuid.Nil, services.UserDeletedEvent, ref, payload)
	require.ErrorContains(t, err, "no tenant")

	noTx := composables.WithPool(context.Background(), e.f.Pool)
	err = events.PublishInTx(noTx, e.tenantID, services.UserDeletedEvent, ref, payload)
	require.ErrorContains(t, err, "transaction")

	require.Zero(t, e.eventCounts(t, e.ctx(), services.UserDeletedEventID)[services.UserDeletedEventID])
	require.NoError(t, events.PublishInTx(e.ctx(), e.tenantID, services.UserDeletedEvent, ref, payload))
	require.Equal(t, 1, e.eventCounts(t, e.ctx(), services.UserDeletedEventID)[services.UserDeletedEventID])
}

// The eventbus events (realtime tables, legacy handlers) of a change made in a
// transaction the caller owns wait for its commit and are never sent for a
// rolled back one.
func TestLegacyEvents_WaitForTheOwningTransaction(t *testing.T) {
	e := newEventEnv(t)
	target := e.newUser(t, "Legacy")
	e.f.CommitTx(t)
	bus, err := composition.Resolve[eventbus.EventBus](e.f.Container)
	require.NoError(t, err)
	var updated atomic.Int32
	unsubscribe := bus.Subscribe(func(*user.UpdatedEvent) { updated.Add(1) })
	defer unsubscribe()
	detached := context.WithValue(context.WithValue(e.ctx(), constants.TxKey, nil), constants.AfterCommitKey, nil)
	errRollBack := errors.New("roll back")

	changes := []struct {
		name   string
		change func(ctx context.Context) error
	}{
		{"Update", func(ctx context.Context) error {
			_, err := e.users.Update(ctx, target.SetName("Legacy-"+uuid.NewString(), "Person", ""))
			return err
		}},
		{"BlockUser", func(ctx context.Context) error {
			_, err := e.users.BlockUser(ctx, target.ID(), "Policy violation")
			return err
		}},
	}
	for _, c := range changes {
		name, change := c.name, c.change
		t.Run(name, func(t *testing.T) {
			updated.Store(0)
			err := composables.InTx(detached, func(txCtx context.Context) error {
				require.NoError(t, change(txCtx))
				require.Zero(t, updated.Load(), "nothing is sent before the transaction commits")
				return errRollBack
			})
			require.ErrorIs(t, err, errRollBack)
			require.Zero(t, updated.Load(), "a rolled back change sends no event")

			require.NoError(t, composables.InTx(detached, func(txCtx context.Context) error {
				require.NoError(t, change(txCtx))
				require.Zero(t, updated.Load(), "nothing is sent before the transaction commits")
				return nil
			}))
			require.EqualValues(t, 1, updated.Load(), "the committed change sends its event once, after the commit")
		})
	}
}

// A transaction the caller begins itself delivers the eventbus events of the
// services only through the owner it declares: without one nothing is sent (the
// caller may roll back), with one they are sent when the caller commits.
func TestLegacyEvents_ExternalTransactionOwner(t *testing.T) {
	e := newEventEnv(t)
	target := e.newUser(t, "External")
	e.f.CommitTx(t)
	bus, err := composition.Resolve[eventbus.EventBus](e.f.Container)
	require.NoError(t, err)
	var updated atomic.Int32
	unsubscribe := bus.Subscribe(func(*user.UpdatedEvent) { updated.Add(1) })
	defer unsubscribe()
	base := context.WithValue(context.WithValue(e.ctx(), constants.TxKey, nil), constants.AfterCommitKey, nil)
	rename := func(ctx context.Context) {
		_, err := e.users.Update(ctx, target.SetName("External-"+uuid.NewString(), "Person", ""))
		require.NoError(t, err)
	}

	t.Run("no declared owner", func(t *testing.T) {
		updated.Store(0)
		tx, err := e.f.Pool.Begin(context.Background())
		require.NoError(t, err)
		rename(composables.WithTx(base, tx))
		require.NoError(t, tx.Rollback(context.Background()))
		require.Zero(t, updated.Load(), "a change in a transaction nobody answers for sends no event, rolled back or not")
	})

	t.Run("declared owner", func(t *testing.T) {
		updated.Store(0)
		for _, commit := range []bool{false, true} {
			tx, err := e.f.Pool.Begin(context.Background())
			require.NoError(t, err)
			callbacks := &composables.TxCallbacks{}
			rename(composables.WithAfterCommit(composables.WithTx(base, tx), tx, callbacks.Add))
			require.Zero(t, updated.Load(), "nothing is sent before the owner commits")
			if !commit {
				require.NoError(t, tx.Rollback(context.Background()))
				require.Zero(t, updated.Load(), "a rolled back change sends no event")
				continue
			}
			require.NoError(t, tx.Commit(context.Background()))
			callbacks.Run()
			require.EqualValues(t, 1, updated.Load(), "the owner runs the callbacks after its commit")
		}
	})
}

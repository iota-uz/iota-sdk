package services_test

import (
	"context"
	"sort"
	"strconv"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/department"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/group"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/role"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/userposition"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/core/permissions"
	"github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/modules/logging/domain/aggregates/businessevent"
	"github.com/iota-uz/iota-sdk/modules/logging/history"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/composition"
	"github.com/iota-uz/iota-sdk/pkg/itf"
	"github.com/stretchr/testify/require"
)

type coreHistoryFixture struct {
	env       *itf.TestEnvironment
	admin     context.Context
	granted   permission.Permission
	users     *services.UserService
	roles     *services.RoleService
	groups    *services.GroupService
	sessions  *services.SessionService
	userRepo  user.Repository
	roleRepo  role.Repository
	groupRepo group.Repository
	deptRepo  department.Repository
	events    businessevent.Repository
}

type expectedEvent struct {
	eventType string
	subjects  []businessevent.Subject
	fields    []string
}

func setupCoreHistory(t *testing.T) coreHistoryFixture {
	t.Helper()
	env := setupTest(t)
	granted := permissions.DepartmentRead
	admin := persistAdmin(t, env,
		permissions.UserCreate, permissions.UserRead, permissions.UserUpdate, permissions.UserDelete, permissions.UserUpdateBlockStatus,
		permissions.RoleCreate, permissions.RoleRead, permissions.RoleUpdate, permissions.RoleDelete,
		permissions.GroupCreate, permissions.GroupRead, permissions.GroupUpdate, permissions.GroupDelete,
		permissions.DepartmentCreate, permissions.DepartmentUpdate, permissions.DepartmentDelete, granted,
		permissions.PositionCreate, permissions.PositionRead, permissions.PositionUpdate, permissions.PositionDelete,
	)
	events, err := composition.Resolve[businessevent.Repository](env.Container)
	require.NoError(t, err)
	userRepo := persistence.NewUserRepository(persistence.NewUploadRepository())
	roleRepo := persistence.NewRoleRepository()
	return coreHistoryFixture{
		env:       env,
		admin:     admin,
		granted:   granted,
		users:     itf.GetService[services.UserService](env),
		roles:     itf.GetService[services.RoleService](env),
		groups:    itf.GetService[services.GroupService](env),
		sessions:  itf.GetService[services.SessionService](env),
		userRepo:  userRepo,
		roleRepo:  roleRepo,
		groupRepo: persistence.NewGroupRepository(userRepo, roleRepo),
		deptRepo:  persistence.NewDepartmentRepository(),
		events:    events,
	}
}

func (f coreHistoryFixture) newUser(t *testing.T, opts ...user.Option) user.User {
	t.Helper()
	email, err := internet.NewEmail("history-" + uuid.NewString() + "@example.test")
	require.NoError(t, err)
	created, err := f.userRepo.Create(f.admin, user.New("Aziz", "Karimov", email, user.UILanguageEN, append([]user.Option{user.WithTenantID(f.env.TenantID())}, opts...)...))
	require.NoError(t, err)
	return created
}

func (f coreHistoryFixture) newRole(t *testing.T) role.Role {
	t.Helper()
	created, err := f.roleRepo.Create(f.admin, role.New("Viewer "+uuid.NewString()[:8],
		role.WithTenantID(f.env.TenantID()), role.WithPermissions([]permission.Permission{f.granted}),
	))
	require.NoError(t, err)
	return created
}

func (f coreHistoryFixture) newGroup(t *testing.T) group.Group {
	t.Helper()
	created, err := f.groupRepo.Save(f.admin, group.New("Accountants "+uuid.NewString()[:8], group.WithTenantID(f.env.TenantID())))
	require.NoError(t, err)
	return created
}

func (f coreHistoryFixture) newDepartment(t *testing.T) department.Department {
	t.Helper()
	created, err := f.deptRepo.Save(f.admin, department.New("D-"+uuid.NewString()[:8], orgName(t, "Finance"),
		department.WithID(uuid.New()), department.WithTenantID(f.env.TenantID()),
	))
	require.NoError(t, err)
	return created
}

func (f coreHistoryFixture) newPosition(t *testing.T, svc *services.UserPositionService) userposition.UserPosition {
	t.Helper()
	created, err := svc.Create(f.admin, userposition.New(f.newUser(t).ID(), f.newDepartment(t).ID(), orgName(t, "Accountant"),
		userposition.WithID(uuid.New()), userposition.WithTenantID(f.env.TenantID()),
	))
	require.NoError(t, err)
	return created
}

func (f coreHistoryFixture) operationEvents(t *testing.T, ctx context.Context) []expectedEvent {
	t.Helper()
	exec, ok := history.UseExecution(ctx)
	require.True(t, ok)
	events, err := f.events.ListByOperation(f.env.Ctx, exec.OperationID)
	require.NoError(t, err)
	result := make([]expectedEvent, 0, len(events))
	for _, e := range events {
		got := expectedEvent{eventType: e.Type, subjects: e.Subjects, fields: []string{}}
		for _, c := range e.Changes {
			for _, d := range c.Fields {
				got.fields = append(got.fields, d.Field)
			}
		}
		result = append(result, got)
	}
	return sortEvents(result)
}

func sortEvents(events []expectedEvent) []expectedEvent {
	for _, e := range events {
		sort.Strings(e.fields)
		sort.Slice(e.subjects, func(i, j int) bool {
			return e.subjects[i].Resource+e.subjects[i].ID < e.subjects[j].Resource+e.subjects[j].ID
		})
	}
	sort.Slice(events, func(i, j int) bool { return events[i].eventType < events[j].eventType })
	return events
}

func subject(resource string, id any) businessevent.Subject {
	switch v := id.(type) {
	case uint:
		return businessevent.Subject{Resource: resource, ID: strconv.FormatUint(uint64(v), 10)}
	case uuid.UUID:
		return businessevent.Subject{Resource: resource, ID: v.String()}
	}
	panic("unsupported subject id")
}

func TestCoreHistory_RecordsServiceMutations(t *testing.T) {
	f := setupCoreHistory(t)
	departments := itf.GetService[services.DepartmentService](f.env)
	positions := itf.GetService[services.UserPositionService](f.env)
	userFields := []string{"email", "first_name", "last_name", "middle_name", "status", "ui_language"}
	departmentFields := []string{"code", "name", "order", "status"}
	positionFields := []string{"department_id", "is_manager", "is_primary", "status", "title", "user_id"}

	tests := []struct {
		name string
		act  func(t *testing.T, ctx context.Context) []expectedEvent
	}{
		{
			name: "user created with a role",
			act: func(t *testing.T, ctx context.Context) []expectedEvent {
				t.Helper()
				viewer := f.newRole(t)
				email, err := internet.NewEmail("new-" + uuid.NewString() + "@example.test")
				require.NoError(t, err)
				created, err := f.users.Create(ctx, user.New("Nodira", "Karimova", email, user.UILanguageRU,
					user.WithTenantID(f.env.TenantID()), user.WithRoles([]role.Role{viewer}),
				))
				require.NoError(t, err)
				u := subject(services.UserResource, created.ID())
				return []expectedEvent{
					{eventType: services.UserCreated, subjects: []businessevent.Subject{u}, fields: userFields},
					{eventType: services.UserRoleAssigned, subjects: []businessevent.Subject{u, subject(services.RoleResource, viewer.ID())}, fields: []string{}},
				}
			},
		},
		{
			name: "user renamed and role taken away",
			act: func(t *testing.T, ctx context.Context) []expectedEvent {
				t.Helper()
				viewer := f.newRole(t)
				target := f.newUser(t, user.WithRoles([]role.Role{viewer}))
				_, err := f.users.Update(ctx, target.SetName("Bobur", target.LastName(), target.MiddleName()).SetRoles(nil))
				require.NoError(t, err)
				u := subject(services.UserResource, target.ID())
				return []expectedEvent{
					{eventType: services.UserRoleUnassigned, subjects: []businessevent.Subject{u, subject(services.RoleResource, viewer.ID())}, fields: []string{}},
					{eventType: services.UserUpdated, subjects: []businessevent.Subject{u}, fields: []string{"first_name"}},
				}
			},
		},
		{
			name: "user deleted",
			act: func(t *testing.T, ctx context.Context) []expectedEvent {
				t.Helper()
				target := f.newUser(t)
				_, err := f.users.Delete(ctx, target.ID())
				require.NoError(t, err)
				return []expectedEvent{{eventType: services.UserDeleted, subjects: []businessevent.Subject{subject(services.UserResource, target.ID())}, fields: userFields}}
			},
		},
		{
			name: "user blocked",
			act: func(t *testing.T, ctx context.Context) []expectedEvent {
				t.Helper()
				target := f.newUser(t)
				_, err := f.users.BlockUser(ctx, target.ID(), "left the company")
				require.NoError(t, err)
				return []expectedEvent{{eventType: services.UserBlocked, subjects: []businessevent.Subject{subject(services.UserResource, target.ID())}, fields: []string{"block_reason", "blocked"}}}
			},
		},
		{
			name: "user unblocked",
			act: func(t *testing.T, ctx context.Context) []expectedEvent {
				t.Helper()
				admin, err := composables.UseUser(f.admin)
				require.NoError(t, err)
				target := f.newUser(t).Block("on leave", admin.ID(), admin.TenantID())
				require.NoError(t, f.userRepo.Update(f.admin, target))
				_, err = f.users.UnblockUser(ctx, target.ID())
				require.NoError(t, err)
				return []expectedEvent{{eventType: services.UserUnblocked, subjects: []businessevent.Subject{subject(services.UserResource, target.ID())}, fields: []string{"block_reason", "blocked"}}}
			},
		},
		{
			name: "temporary password issued",
			act: func(t *testing.T, ctx context.Context) []expectedEvent {
				t.Helper()
				target := f.newUser(t)
				_, err := f.users.IssueTemporaryPassword(ctx, target.ID(), "")
				require.NoError(t, err)
				return []expectedEvent{{eventType: services.UserTemporaryPasswordIssued, subjects: []businessevent.Subject{subject(services.UserResource, target.ID())}, fields: []string{"password_expires_at", "status"}}}
			},
		},
		{
			name: "role created with a permission",
			act: func(t *testing.T, ctx context.Context) []expectedEvent {
				t.Helper()
				created, err := f.roles.Create(ctx, role.New("Auditor", role.WithTenantID(f.env.TenantID()), role.WithPermissions([]permission.Permission{f.granted})))
				require.NoError(t, err)
				r := subject(services.RoleResource, created.ID())
				return []expectedEvent{
					{eventType: services.RoleCreated, subjects: []businessevent.Subject{r}, fields: []string{"description", "name"}},
					{eventType: services.RolePermissionGranted, subjects: []businessevent.Subject{subject(services.PermissionResource, created.Permissions()[0].ID()), r}, fields: []string{}},
				}
			},
		},
		{
			name: "role renamed and permission removed",
			act: func(t *testing.T, ctx context.Context) []expectedEvent {
				t.Helper()
				viewer := f.newRole(t)
				require.NoError(t, f.roles.Update(ctx, viewer.SetName("Renamed "+uuid.NewString()[:8]).SetPermissions(nil)))
				r := subject(services.RoleResource, viewer.ID())
				return []expectedEvent{
					{eventType: services.RolePermissionRevoked, subjects: []businessevent.Subject{subject(services.PermissionResource, viewer.Permissions()[0].ID()), r}, fields: []string{}},
					{eventType: services.RoleUpdated, subjects: []businessevent.Subject{r}, fields: []string{"name"}},
				}
			},
		},
		{
			name: "role deleted",
			act: func(t *testing.T, ctx context.Context) []expectedEvent {
				t.Helper()
				viewer := f.newRole(t)
				require.NoError(t, f.roles.Delete(ctx, viewer.ID()))
				return []expectedEvent{{eventType: services.RoleDeleted, subjects: []businessevent.Subject{subject(services.RoleResource, viewer.ID())}, fields: []string{"description", "name"}}}
			},
		},
		{
			name: "group created with a member and a role",
			act: func(t *testing.T, ctx context.Context) []expectedEvent {
				t.Helper()
				member := f.newUser(t)
				viewer := f.newRole(t)
				created, err := f.groups.Create(ctx, group.New("Cashiers", group.WithTenantID(f.env.TenantID()),
					group.WithUsers([]user.User{member}), group.WithRoles([]role.Role{viewer}),
				))
				require.NoError(t, err)
				g := subject(services.GroupResource, created.ID())
				return []expectedEvent{
					{eventType: services.GroupCreated, subjects: []businessevent.Subject{g}, fields: []string{"description", "name"}},
					{eventType: services.GroupMemberAdded, subjects: []businessevent.Subject{g, subject(services.UserResource, member.ID())}, fields: []string{}},
					{eventType: services.GroupRoleAssigned, subjects: []businessevent.Subject{g, subject(services.RoleResource, viewer.ID())}, fields: []string{}},
				}
			},
		},
		{
			name: "user added to a group",
			act: func(t *testing.T, ctx context.Context) []expectedEvent {
				t.Helper()
				existing := f.newGroup(t)
				member := f.newUser(t)
				_, err := f.groups.AddUser(ctx, existing.ID(), member)
				require.NoError(t, err)
				return []expectedEvent{{eventType: services.GroupMemberAdded, subjects: []businessevent.Subject{subject(services.GroupResource, existing.ID()), subject(services.UserResource, member.ID())}, fields: []string{}}}
			},
		},
		{
			name: "group deleted",
			act: func(t *testing.T, ctx context.Context) []expectedEvent {
				t.Helper()
				existing := f.newGroup(t)
				require.NoError(t, f.groups.Delete(ctx, existing.ID()))
				return []expectedEvent{{eventType: services.GroupDeleted, subjects: []businessevent.Subject{subject(services.GroupResource, existing.ID())}, fields: []string{"description", "name"}}}
			},
		},
		{
			name: "department created",
			act: func(t *testing.T, ctx context.Context) []expectedEvent {
				t.Helper()
				created, err := departments.Create(ctx, department.New("D-"+uuid.NewString()[:8], orgName(t, "Sales"),
					department.WithID(uuid.New()), department.WithTenantID(f.env.TenantID()),
				))
				require.NoError(t, err)
				return []expectedEvent{{eventType: services.DepartmentCreated, subjects: []businessevent.Subject{subject(services.DepartmentResource, created.ID())}, fields: departmentFields}}
			},
		},
		{
			name: "department renamed",
			act: func(t *testing.T, ctx context.Context) []expectedEvent {
				t.Helper()
				existing := f.newDepartment(t)
				_, err := departments.Update(ctx, existing.SetName(orgName(t, "Treasury")))
				require.NoError(t, err)
				return []expectedEvent{{eventType: services.DepartmentUpdated, subjects: []businessevent.Subject{subject(services.DepartmentResource, existing.ID())}, fields: []string{"name"}}}
			},
		},
		{
			name: "department deleted",
			act: func(t *testing.T, ctx context.Context) []expectedEvent {
				t.Helper()
				existing := f.newDepartment(t)
				require.NoError(t, departments.Delete(ctx, existing.ID()))
				return []expectedEvent{{eventType: services.DepartmentDeleted, subjects: []businessevent.Subject{subject(services.DepartmentResource, existing.ID())}, fields: departmentFields}}
			},
		},
		{
			name: "position created",
			act: func(t *testing.T, ctx context.Context) []expectedEvent {
				t.Helper()
				created, err := positions.Create(ctx, userposition.New(f.newUser(t).ID(), f.newDepartment(t).ID(), orgName(t, "Cashier"),
					userposition.WithID(uuid.New()), userposition.WithTenantID(f.env.TenantID()),
				))
				require.NoError(t, err)
				return []expectedEvent{{eventType: services.UserPositionCreated, subjects: []businessevent.Subject{subject(services.UserPositionResource, created.ID())}, fields: positionFields}}
			},
		},
		{
			name: "position made managerial",
			act: func(t *testing.T, ctx context.Context) []expectedEvent {
				t.Helper()
				existing := f.newPosition(t, positions)
				_, err := positions.Update(ctx, existing.SetIsManager(true))
				require.NoError(t, err)
				return []expectedEvent{{eventType: services.UserPositionUpdated, subjects: []businessevent.Subject{subject(services.UserPositionResource, existing.ID())}, fields: []string{"is_manager"}}}
			},
		},
		{
			name: "position deleted",
			act: func(t *testing.T, ctx context.Context) []expectedEvent {
				t.Helper()
				existing := f.newPosition(t, positions)
				require.NoError(t, positions.Delete(ctx, existing.ID()))
				return []expectedEvent{{eventType: services.UserPositionDeleted, subjects: []businessevent.Subject{subject(services.UserPositionResource, existing.ID())}, fields: positionFields}}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := itf.TestExecution(f.admin)
			want := tt.act(t, ctx)
			require.Equal(t, sortEvents(want), f.operationEvents(t, ctx))
		})
	}
}

func TestCoreHistory_PasswordChangeRecordsNoSecret(t *testing.T) {
	f := setupCoreHistory(t)
	owner := f.newUser(t)
	owner, err := owner.SetPassword("Current-pass-1")
	require.NoError(t, err)
	require.NoError(t, f.userRepo.UpdatePassword(f.admin, owner.ID(), owner.Password(), owner.UpdatedAt()))
	ctx := itf.TestExecution(composables.WithUser(f.env.Ctx, owner))

	require.NoError(t, f.users.ChangePassword(ctx, "Current-pass-1", "Brand-new-pass-2"))

	want := []expectedEvent{{eventType: services.UserPasswordChanged, subjects: []businessevent.Subject{subject(services.UserResource, owner.ID())}, fields: []string{}}}
	require.Equal(t, want, f.operationEvents(t, ctx))
}

func TestCoreHistory_OnboardingWithoutUserInContext(t *testing.T) {
	f := setupCoreHistory(t)
	pending := f.newUser(t)
	pending, err := pending.IssueTemporaryPassword("Temporary-1", time.Now().Add(time.Hour))
	require.NoError(t, err)
	require.NoError(t, f.userRepo.UpdateCredentials(f.admin, pending))
	token := onboardingSessionToken(t, f.env.Ctx, f.sessions, pending)
	exec, ok := history.UseExecution(f.env.Ctx)
	require.True(t, ok)
	actor := businessevent.UserActor(pending)
	ctx := history.WithExecution(f.env.Ctx, history.Execution{Channel: businessevent.ChannelUI, OperationID: uuid.NewString(), RequestID: exec.RequestID, Actor: &actor})

	_, err = f.users.CompleteOnboarding(ctx, token, services.OnboardingInput{FirstName: "Nodira", LastName: "Karimova", Language: user.UILanguageRU, Password: "my own passphrase"})
	require.NoError(t, err)

	want := []expectedEvent{{eventType: services.UserOnboardingCompleted, subjects: []businessevent.Subject{subject(services.UserResource, pending.ID())}, fields: []string{"first_name", "last_name", "status", "ui_language"}}}
	require.Equal(t, want, f.operationEvents(t, ctx))
}

func TestCoreHistory_UnchangedUpdateRecordsNothing(t *testing.T) {
	f := setupCoreHistory(t)
	existing := f.newGroup(t)
	ctx := itf.TestExecution(f.admin)

	_, err := f.groups.Update(ctx, existing)
	require.NoError(t, err)

	require.Empty(t, f.operationEvents(t, ctx))
}

func TestCoreHistory_RollbackLeavesNoEvents(t *testing.T) {
	f := setupCoreHistory(t)
	target := f.newUser(t)

	savepoint, err := f.env.Tx.Begin(f.env.Ctx)
	require.NoError(t, err)
	operation := itf.TestExecution(composables.WithTx(f.admin, savepoint))

	_, err = f.users.BlockUser(operation, target.ID(), "left the company")
	require.NoError(t, err)
	require.Len(t, f.operationEvents(t, operation), 1)
	require.NoError(t, savepoint.Rollback(f.env.Ctx))

	require.Empty(t, f.operationEvents(t, operation))
	reloaded, err := f.userRepo.GetByID(f.admin, target.ID())
	require.NoError(t, err)
	require.False(t, reloaded.IsBlocked())
}

func TestCoreHistory_DefinitionsAreSecurityForAccess(t *testing.T) {
	for _, d := range services.HistoryDefinitions() {
		want := businessevent.CategorySecurity
		if d.Resources[0].Name == services.DepartmentResource || d.Resources[0].Name == services.UserPositionResource {
			want = businessevent.CategoryBusiness
		}
		require.Equal(t, want, d.Category, d.Type)
	}
}

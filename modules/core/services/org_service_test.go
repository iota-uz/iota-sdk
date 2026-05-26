package services_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/department"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/userposition"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/query"
	"github.com/iota-uz/iota-sdk/modules/core/permissions"
	"github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	crudmodels "github.com/iota-uz/iota-sdk/pkg/crud/models"
	"github.com/iota-uz/iota-sdk/pkg/eventbus"
	"github.com/iota-uz/iota-sdk/pkg/itf"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// withActor injects a permissioned actor into ctx so service-layer permission
// checks (CanUser) and actor resolution (UseUser) succeed. The actor only
// drives RBAC/event attribution; tenant scoping still comes from the context's
// tenant id, so a single actor is reused across tenant contexts.
func withActor(ctx context.Context) context.Context {
	actor := itf.User(
		permissions.DepartmentCreate,
		permissions.DepartmentUpdate,
		permissions.DepartmentRead,
		permissions.PositionCreate,
		permissions.PositionUpdate,
		permissions.PositionRead,
	)
	return composables.WithUser(ctx, actor)
}

func orgName(t *testing.T, base string) crudmodels.MultiLang {
	t.Helper()
	ml, err := crudmodels.NewMultiLangFromMap(map[string]string{
		"en":      base + " EN",
		"ru":      base + " RU",
		"uz":      base + " UZ",
		"uz-Cyrl": base + " UZ-CYRL",
	})
	require.NoError(t, err)
	return ml
}

func newDepartmentService() (*services.DepartmentService, department.Repository, query.OrgQueryRepository) {
	repo := persistence.NewDepartmentRepository()
	orgQuery := query.NewPgOrgQueryRepository()
	bus := eventbus.NewEventPublisher(logrus.New())
	return services.NewDepartmentService(repo, orgQuery, bus), repo, orgQuery
}

func newUserPositionService() (*services.UserPositionService, department.Repository, user.Repository) {
	uploadRepo := persistence.NewUploadRepository()
	userRepo := persistence.NewUserRepository(uploadRepo)
	deptRepo := persistence.NewDepartmentRepository()
	posRepo := persistence.NewUserPositionRepository()
	bus := eventbus.NewEventPublisher(logrus.New())
	return services.NewUserPositionService(posRepo, deptRepo, userRepo, bus), deptRepo, userRepo
}

// requireValidation asserts err is non-nil and classified as a validation error.
func requireValidation(t *testing.T, err error) {
	t.Helper()
	require.Error(t, err)
	var se *serrors.Error
	if assert.ErrorAs(t, err, &se) {
		assert.Equal(t, "validation", se.ErrorKind(), "expected a validation-kind error, got: %v", err)
	}
}

func TestDepartmentService_Create_StructuralValidation(t *testing.T) {
	t.Parallel()
	f := setupTest(t)
	svc, repo, _ := newDepartmentService()

	ctx := withActor(f.Ctx)
	tenantA, err := composables.UseTenantID(ctx)
	require.NoError(t, err)

	t.Run("rejects missing locale", func(t *testing.T) {
		partial, err := crudmodels.NewMultiLangFromMap(map[string]string{"en": "x", "ru": "y", "uz": "z"})
		require.NoError(t, err)
		_, err = svc.Create(ctx, department.New(
			"NO-LOCALE", partial,
			department.WithID(uuid.New()), department.WithTenantID(tenantA),
		))
		requireValidation(t, err)
	})

	t.Run("rejects self-referential parent", func(t *testing.T) {
		id := uuid.New()
		_, err := svc.Create(ctx, department.New(
			"SELF", orgName(t, "Self"),
			department.WithID(id), department.WithTenantID(tenantA),
			department.WithParentID(&id),
		))
		requireValidation(t, err)
	})

	t.Run("rejects non-existent parent", func(t *testing.T) {
		missing := uuid.New()
		_, err := svc.Create(ctx, department.New(
			"NOPARENT", orgName(t, "NoParent"),
			department.WithID(uuid.New()), department.WithTenantID(tenantA),
			department.WithParentID(&missing),
		))
		requireValidation(t, err)
	})

	t.Run("rejects cross-tenant parent", func(t *testing.T) {
		// Parent lives in tenant A; child attempts to attach from tenant B.
		parentID := uuid.New()
		_, err := repo.Save(ctx, department.New(
			"XT-PARENT", orgName(t, "Parent"),
			department.WithID(parentID), department.WithTenantID(tenantA),
		))
		require.NoError(t, err)

		secondTenant := mkTenant(t, f)
		ctxB := withActor(composables.WithTenantID(f.Ctx, secondTenant))
		_, err = svc.Create(ctxB, department.New(
			"XT-CHILD", orgName(t, "Child"),
			department.WithID(uuid.New()), department.WithTenantID(secondTenant),
			department.WithParentID(&parentID),
		))
		requireValidation(t, err)
	})

	t.Run("accepts valid parent in same tenant", func(t *testing.T) {
		parentID := uuid.New()
		_, err := svc.Create(ctx, department.New(
			"OK-PARENT", orgName(t, "OkParent"),
			department.WithID(parentID), department.WithTenantID(tenantA),
		))
		require.NoError(t, err)

		_, err = svc.Create(ctx, department.New(
			"OK-CHILD", orgName(t, "OkChild"),
			department.WithID(uuid.New()), department.WithTenantID(tenantA),
			department.WithParentID(&parentID),
		))
		require.NoError(t, err)
	})
}

func TestDepartmentService_Update_CycleRejected(t *testing.T) {
	t.Parallel()
	f := setupTest(t)
	svc, _, _ := newDepartmentService()

	ctx := withActor(f.Ctx)
	tenantA, err := composables.UseTenantID(ctx)
	require.NoError(t, err)

	// Build A -> B -> C.
	aID := uuid.New()
	a, err := svc.Create(ctx, department.New(
		"CYC-A", orgName(t, "A"),
		department.WithID(aID), department.WithTenantID(tenantA),
	))
	require.NoError(t, err)

	bID := uuid.New()
	_, err = svc.Create(ctx, department.New(
		"CYC-B", orgName(t, "B"),
		department.WithID(bID), department.WithTenantID(tenantA),
		department.WithParentID(&aID),
	))
	require.NoError(t, err)

	cID := uuid.New()
	_, err = svc.Create(ctx, department.New(
		"CYC-C", orgName(t, "C"),
		department.WithID(cID), department.WithTenantID(tenantA),
		department.WithParentID(&bID),
	))
	require.NoError(t, err)

	t.Run("reparenting A under its descendant C is rejected", func(t *testing.T) {
		// A is the root; making C (a descendant) its parent forms a cycle.
		_, err := svc.Update(ctx, a.SetParentID(&cID))
		requireValidation(t, err)
	})

	t.Run("reparenting A under itself is rejected", func(t *testing.T) {
		_, err := svc.Update(ctx, a.SetParentID(&aID))
		requireValidation(t, err)
	})
}

func TestUserPositionService_Create_RefValidation(t *testing.T) {
	t.Parallel()
	f := setupTest(t)
	svc, deptRepo, userRepo := newUserPositionService()

	ctx := withActor(f.Ctx)
	tenantA, err := composables.UseTenantID(ctx)
	require.NoError(t, err)

	// Valid user + department in tenant A.
	emailA, err := internet.NewEmail("pos-svc-a@gmail.com")
	require.NoError(t, err)
	userA, err := userRepo.Create(ctx, user.New(
		"Pos", "A", emailA, user.UILanguageEN, user.WithTenantID(tenantA),
	))
	require.NoError(t, err)

	deptAID := uuid.New()
	_, err = deptRepo.Save(ctx, department.New(
		"POS-A", orgName(t, "DeptA"),
		department.WithID(deptAID), department.WithTenantID(tenantA),
	))
	require.NoError(t, err)

	t.Run("rejects non-existent department", func(t *testing.T) {
		_, err := svc.Create(ctx, userposition.New(
			userA.ID(), uuid.New(), orgName(t, "Eng"),
			userposition.WithID(uuid.New()), userposition.WithTenantID(tenantA),
		))
		requireValidation(t, err)
	})

	t.Run("rejects cross-tenant department", func(t *testing.T) {
		// Department exists only in tenant B; tenant-A caller must be rejected.
		secondTenant := mkTenant(t, f)
		ctxB := composables.WithTenantID(f.Ctx, secondTenant)
		deptBID := uuid.New()
		_, err := deptRepo.Save(ctxB, department.New(
			"POS-B", orgName(t, "DeptB"),
			department.WithID(deptBID), department.WithTenantID(secondTenant),
		))
		require.NoError(t, err)

		_, err = svc.Create(ctx, userposition.New(
			userA.ID(), deptBID, orgName(t, "Eng"),
			userposition.WithID(uuid.New()), userposition.WithTenantID(tenantA),
		))
		requireValidation(t, err)
	})

	t.Run("rejects cross-tenant user", func(t *testing.T) {
		// User exists only in tenant B; assigning to a tenant-A department
		// from a tenant-A caller must be rejected on the user reference.
		secondTenant := mkTenant(t, f)
		ctxB := composables.WithTenantID(f.Ctx, secondTenant)
		emailB, err := internet.NewEmail("pos-svc-b@gmail.com")
		require.NoError(t, err)
		userB, err := userRepo.Create(ctxB, user.New(
			"Pos", "B", emailB, user.UILanguageEN, user.WithTenantID(secondTenant),
		))
		require.NoError(t, err)

		_, err = svc.Create(ctx, userposition.New(
			userB.ID(), deptAID, orgName(t, "Eng"),
			userposition.WithID(uuid.New()), userposition.WithTenantID(tenantA),
		))
		requireValidation(t, err)
	})

	t.Run("accepts valid same-tenant references", func(t *testing.T) {
		_, err := svc.Create(ctx, userposition.New(
			userA.ID(), deptAID, orgName(t, "Eng"),
			userposition.WithID(uuid.New()), userposition.WithTenantID(tenantA),
		))
		require.NoError(t, err)
	})
}

// mkTenant provisions a second tenant for cross-tenant service tests and
// returns its ID.
func mkTenant(t *testing.T, f *itf.TestEnvironment) uuid.UUID {
	t.Helper()
	secondTenant, err := itf.CreateTestTenant(f.Ctx, f.Pool)
	require.NoError(t, err)
	return secondTenant.ID
}

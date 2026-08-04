package query_test

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
	"github.com/iota-uz/iota-sdk/pkg/composables"
	crudmodels "github.com/iota-uz/iota-sdk/pkg/crud/models"
	"github.com/iota-uz/iota-sdk/pkg/itf"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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

func idSet(ids []uuid.UUID) map[uuid.UUID]struct{} {
	out := make(map[uuid.UUID]struct{}, len(ids))
	for _, id := range ids {
		out[id] = struct{}{}
	}
	return out
}

// secondTenantCtx provisions an additional tenant and returns a context scoped
// to it, mirroring the cross-tenant pattern used in the persistence tests.
func secondTenantCtx(t *testing.T, f *itf.TestEnvironment) (context.Context, uuid.UUID) {
	t.Helper()
	secondTenant, err := itf.CreateTestTenant(f.Ctx, f.Pool)
	require.NoError(t, err)
	return composables.WithTenantID(f.Ctx, secondTenant.ID), secondTenant.ID
}

func TestPgOrgQueryRepository(t *testing.T) {
	t.Parallel()
	f := setupTest(t)

	uploadRepository := persistence.NewUploadRepository()
	userRepository := persistence.NewUserRepository(uploadRepository)
	departmentRepository := persistence.NewDepartmentRepository()
	positionRepository := persistence.NewUserPositionRepository()
	orgQuery := query.NewPgOrgQueryRepository()

	tenant, err := composables.UseTenantID(f.Ctx)
	require.NoError(t, err)

	mkDept := func(code string, parent *uuid.UUID) uuid.UUID {
		id := uuid.New()
		opts := []department.Option{
			department.WithID(id),
			department.WithTenantID(tenant),
		}
		if parent != nil {
			opts = append(opts, department.WithParentID(parent))
		}
		_, err := departmentRepository.Save(f.Ctx, department.New(code, orgName(t, code), opts...))
		require.NoError(t, err)
		return id
	}

	// Hierarchy: A -> B -> C ; D is unrelated.
	deptA := mkDept("ORG-A", nil)
	deptB := mkDept("ORG-B", &deptA)
	deptC := mkDept("ORG-C", &deptB)
	deptD := mkDept("ORG-D", nil)

	email, err := internet.NewEmail("org-query@gmail.com")
	require.NoError(t, err)
	managerUser, err := userRepository.Create(f.Ctx, user.New(
		"Org", "Manager", email, user.UILanguageEN, user.WithTenantID(tenant),
	))
	require.NoError(t, err)

	// Manager of A (root), plain member of D.
	_, err = positionRepository.Save(f.Ctx, userposition.New(
		managerUser.ID(), deptA, orgName(t, "Director"),
		userposition.WithID(uuid.New()),
		userposition.WithTenantID(tenant),
		userposition.WithIsManager(true),
		userposition.WithIsPrimary(true),
	))
	require.NoError(t, err)
	_, err = positionRepository.Save(f.Ctx, userposition.New(
		managerUser.ID(), deptD, orgName(t, "Advisor"),
		userposition.WithID(uuid.New()),
		userposition.WithTenantID(tenant),
		userposition.WithIsManager(false),
	))
	require.NoError(t, err)

	t.Run("UserDepartments", func(t *testing.T) {
		ids, err := orgQuery.UserDepartments(f.Ctx, managerUser.ID())
		require.NoError(t, err)
		set := idSet(ids)
		assert.Len(t, set, 2)
		assert.Contains(t, set, deptA)
		assert.Contains(t, set, deptD)
	})

	t.Run("UserManagedDepartments_NoSubtree", func(t *testing.T) {
		ids, err := orgQuery.UserManagedDepartments(f.Ctx, managerUser.ID(), false)
		require.NoError(t, err)
		set := idSet(ids)
		assert.Len(t, set, 1)
		assert.Contains(t, set, deptA)
		assert.NotContains(t, set, deptB)
	})

	t.Run("UserManagedDepartments_Subtree", func(t *testing.T) {
		ids, err := orgQuery.UserManagedDepartments(f.Ctx, managerUser.ID(), true)
		require.NoError(t, err)
		set := idSet(ids)
		// A (managed root) plus descendants B and C. D is unrelated.
		assert.Len(t, set, 3)
		assert.Contains(t, set, deptA)
		assert.Contains(t, set, deptB)
		assert.Contains(t, set, deptC)
		assert.NotContains(t, set, deptD)
	})

	t.Run("DepartmentSubtree", func(t *testing.T) {
		ids, err := orgQuery.DepartmentSubtree(f.Ctx, deptA)
		require.NoError(t, err)
		set := idSet(ids)
		assert.Len(t, set, 3)
		assert.Contains(t, set, deptA)
		assert.Contains(t, set, deptB)
		assert.Contains(t, set, deptC)

		// Subtree from a mid node.
		ids, err = orgQuery.DepartmentSubtree(f.Ctx, deptB)
		require.NoError(t, err)
		set = idSet(ids)
		assert.Len(t, set, 2)
		assert.Contains(t, set, deptB)
		assert.Contains(t, set, deptC)
		assert.NotContains(t, set, deptA)
	})
}

// TestPgOrgQueryRepository_TenantIsolation builds an identically shaped
// hierarchy under two tenants (same codes, same depth) and asserts the
// recursive-CTE walks never bleed across the tenant boundary. This is the
// central guarantee the org model exists to provide.
func TestPgOrgQueryRepository_TenantIsolation(t *testing.T) {
	t.Parallel()
	f := setupTest(t)

	uploadRepository := persistence.NewUploadRepository()
	userRepository := persistence.NewUserRepository(uploadRepository)
	departmentRepository := persistence.NewDepartmentRepository()
	positionRepository := persistence.NewUserPositionRepository()
	orgQuery := query.NewPgOrgQueryRepository()

	ctxB, _ := secondTenantCtx(t, f)

	// mkDept creates a department in the given tenant context.
	mkDept := func(ctx context.Context, code string, parent *uuid.UUID) uuid.UUID {
		id := uuid.New()
		tid, err := composables.UseTenantID(ctx)
		require.NoError(t, err)
		opts := []department.Option{department.WithID(id), department.WithTenantID(tid)}
		if parent != nil {
			opts = append(opts, department.WithParentID(parent))
		}
		_, err = departmentRepository.Save(ctx, department.New(code, orgName(t, code), opts...))
		require.NoError(t, err)
		return id
	}

	// Identically shaped hierarchies in both tenants: ROOT -> CHILD -> LEAF.
	aRoot := mkDept(f.Ctx, "ISO-ROOT", nil)
	aChild := mkDept(f.Ctx, "ISO-CHILD", &aRoot)
	aLeaf := mkDept(f.Ctx, "ISO-LEAF", &aChild)

	bRoot := mkDept(ctxB, "ISO-ROOT", nil)
	bChild := mkDept(ctxB, "ISO-CHILD", &bRoot)
	bLeaf := mkDept(ctxB, "ISO-LEAF", &bChild)

	// Manager users, one per tenant, each managing their own root.
	mkManager := func(ctx context.Context, emailStr string, root uuid.UUID) uint {
		tid, err := composables.UseTenantID(ctx)
		require.NoError(t, err)
		email, err := internet.NewEmail(emailStr)
		require.NoError(t, err)
		u, err := userRepository.Create(ctx, user.New(
			"Iso", "Manager", email, user.UILanguageEN, user.WithTenantID(tid),
		))
		require.NoError(t, err)
		_, err = positionRepository.Save(ctx, userposition.New(
			u.ID(), root, orgName(t, "Director"),
			userposition.WithID(uuid.New()),
			userposition.WithTenantID(tid),
			userposition.WithIsManager(true),
			userposition.WithIsPrimary(true),
		))
		require.NoError(t, err)
		return u.ID()
	}
	managerA := mkManager(f.Ctx, "iso-a@gmail.com", aRoot)
	managerB := mkManager(ctxB, "iso-b@gmail.com", bRoot)

	t.Run("DepartmentSubtree stays within tenant", func(t *testing.T) {
		idsA, err := orgQuery.DepartmentSubtree(f.Ctx, aRoot)
		require.NoError(t, err)
		setA := idSet(idsA)
		assert.Len(t, setA, 3)
		assert.Contains(t, setA, aRoot)
		assert.Contains(t, setA, aChild)
		assert.Contains(t, setA, aLeaf)
		// No tenant-B node leaks in.
		assert.NotContains(t, setA, bRoot)
		assert.NotContains(t, setA, bChild)
		assert.NotContains(t, setA, bLeaf)

		idsB, err := orgQuery.DepartmentSubtree(ctxB, bRoot)
		require.NoError(t, err)
		setB := idSet(idsB)
		assert.Len(t, setB, 3)
		assert.Contains(t, setB, bRoot)
		assert.NotContains(t, setB, aRoot)
		assert.NotContains(t, setB, aChild)
		assert.NotContains(t, setB, aLeaf)
	})

	t.Run("DepartmentSubtree of another tenant root returns nothing", func(t *testing.T) {
		// Asking tenant A's context for tenant B's root must yield an empty set
		// (the CTE seed `id = $1 AND tenant_id = $2` finds no row).
		ids, err := orgQuery.DepartmentSubtree(f.Ctx, bRoot)
		require.NoError(t, err)
		assert.Empty(t, ids)
	})

	t.Run("UserManagedDepartments subtree never crosses tenants", func(t *testing.T) {
		idsA, err := orgQuery.UserManagedDepartments(f.Ctx, managerA, true)
		require.NoError(t, err)
		setA := idSet(idsA)
		assert.Len(t, setA, 3)
		assert.Contains(t, setA, aRoot)
		assert.Contains(t, setA, aChild)
		assert.Contains(t, setA, aLeaf)
		assert.NotContains(t, setA, bRoot)
		assert.NotContains(t, setA, bChild)
		assert.NotContains(t, setA, bLeaf)

		// Manager A queried under tenant B's context resolves nothing: the
		// position rows are tenant-A scoped.
		idsCross, err := orgQuery.UserManagedDepartments(ctxB, managerA, true)
		require.NoError(t, err)
		assert.Empty(t, idsCross)

		idsB, err := orgQuery.UserManagedDepartments(ctxB, managerB, true)
		require.NoError(t, err)
		setB := idSet(idsB)
		assert.Len(t, setB, 3)
		assert.Contains(t, setB, bRoot)
		assert.NotContains(t, setB, aRoot)
	})

	t.Run("UserDepartments scoped to tenant", func(t *testing.T) {
		ids, err := orgQuery.UserDepartments(f.Ctx, managerA)
		require.NoError(t, err)
		set := idSet(ids)
		assert.Contains(t, set, aRoot)
		assert.NotContains(t, set, bRoot)

		// managerA has no positions in tenant B.
		idsCross, err := orgQuery.UserDepartments(ctxB, managerA)
		require.NoError(t, err)
		assert.Empty(t, idsCross)
	})
}

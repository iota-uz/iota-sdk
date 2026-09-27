package persistence_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/department"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
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

func TestPgDepartmentRepository_CRUD(t *testing.T) {
	t.Parallel()
	f := setupTest(t)

	departmentRepository := persistence.NewDepartmentRepository()

	tenant, err := composables.UseTenantID(f.Ctx)
	require.NoError(t, err)

	t.Run("CreateAndGet", func(t *testing.T) {
		id := uuid.New()
		entity := department.New(
			"ENG",
			orgName(t, "Engineering"),
			department.WithID(id),
			department.WithTenantID(tenant),
			department.WithOrder(1),
		)

		saved, err := departmentRepository.Save(f.Ctx, entity)
		require.NoError(t, err)
		assert.Equal(t, id, saved.ID())
		assert.Equal(t, "ENG", saved.Code())
		assert.Equal(t, "Engineering EN", saved.Name("en"))
		assert.Equal(t, "Engineering RU", saved.Name("ru"))
		assert.Equal(t, "Engineering UZ-CYRL", saved.Name("uz-Cyrl"))
		assert.Equal(t, 1, saved.Order())
		assert.Equal(t, department.StatusActive, saved.Status())
		assert.Nil(t, saved.ParentID())

		retrieved, err := departmentRepository.GetByID(f.Ctx, id)
		require.NoError(t, err)
		assert.Equal(t, id, retrieved.ID())
		assert.Equal(t, "Engineering UZ", retrieved.Name("uz"))

		// Non-existent
		_, err = departmentRepository.GetByID(f.Ctx, uuid.New())
		require.ErrorIs(t, err, persistence.ErrDepartmentNotFound)
	})

	t.Run("Hierarchy", func(t *testing.T) {
		parentID := uuid.New()
		parent := department.New(
			"PARENT",
			orgName(t, "Parent"),
			department.WithID(parentID),
			department.WithTenantID(tenant),
		)
		_, err := departmentRepository.Save(f.Ctx, parent)
		require.NoError(t, err)

		childID := uuid.New()
		child := department.New(
			"CHILD",
			orgName(t, "Child"),
			department.WithID(childID),
			department.WithTenantID(tenant),
			department.WithParentID(&parentID),
		)
		savedChild, err := departmentRepository.Save(f.Ctx, child)
		require.NoError(t, err)
		require.NotNil(t, savedChild.ParentID())
		assert.Equal(t, parentID, *savedChild.ParentID())

		retrieved, err := departmentRepository.GetByID(f.Ctx, childID)
		require.NoError(t, err)
		require.NotNil(t, retrieved.ParentID())
		assert.Equal(t, parentID, *retrieved.ParentID())
	})

	t.Run("Update", func(t *testing.T) {
		id := uuid.New()
		entity := department.New(
			"UPD",
			orgName(t, "Original"),
			department.WithID(id),
			department.WithTenantID(tenant),
		)
		saved, err := departmentRepository.Save(f.Ctx, entity)
		require.NoError(t, err)

		updated := saved.
			SetName(orgName(t, "Updated")).
			SetOrder(5).
			SetStatus(department.StatusInactive)
		savedUpdated, err := departmentRepository.Save(f.Ctx, updated)
		require.NoError(t, err)
		assert.Equal(t, "Updated EN", savedUpdated.Name("en"))
		assert.Equal(t, 5, savedUpdated.Order())
		assert.Equal(t, department.StatusInactive, savedUpdated.Status())
	})

	t.Run("ExistsAndDelete", func(t *testing.T) {
		id := uuid.New()
		entity := department.New(
			"DEL",
			orgName(t, "ToDelete"),
			department.WithID(id),
			department.WithTenantID(tenant),
		)
		_, err := departmentRepository.Save(f.Ctx, entity)
		require.NoError(t, err)

		exists, err := departmentRepository.Exists(f.Ctx, id)
		require.NoError(t, err)
		assert.True(t, exists)

		require.NoError(t, departmentRepository.Delete(f.Ctx, id))

		exists, err = departmentRepository.Exists(f.Ctx, id)
		require.NoError(t, err)
		assert.False(t, exists)

		_, err = departmentRepository.GetByID(f.Ctx, id)
		require.ErrorIs(t, err, persistence.ErrDepartmentNotFound)
	})

	t.Run("CountAndPaginate", func(t *testing.T) {
		count, err := departmentRepository.Count(f.Ctx, &department.FindParams{})
		require.NoError(t, err)
		assert.GreaterOrEqual(t, count, int64(1))

		list, err := departmentRepository.GetPaginated(f.Ctx, &department.FindParams{Limit: 100})
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(list), 1)
	})
}

func TestPgDepartmentRepository_TenantIsolation(t *testing.T) {
	t.Parallel()
	f := setupTest(t)

	departmentRepository := persistence.NewDepartmentRepository()

	tenantA, err := composables.UseTenantID(f.Ctx)
	require.NoError(t, err)

	secondTenant, err := itf.CreateTestTenant(f.Ctx, f.Pool)
	require.NoError(t, err)
	ctxB := composables.WithTenantID(f.Ctx, secondTenant.ID)

	// One department per tenant; identical code to prove isolation is by tenant.
	idA := uuid.New()
	_, err = departmentRepository.Save(f.Ctx, department.New(
		"ISO", orgName(t, "Dept A"),
		department.WithID(idA), department.WithTenantID(tenantA),
	))
	require.NoError(t, err)

	idB := uuid.New()
	_, err = departmentRepository.Save(ctxB, department.New(
		"ISO", orgName(t, "Dept B"),
		department.WithID(idB), department.WithTenantID(secondTenant.ID),
	))
	require.NoError(t, err)

	t.Run("GetByID does not cross tenants", func(t *testing.T) {
		// Tenant A cannot read tenant B's department and vice versa.
		_, err := departmentRepository.GetByID(f.Ctx, idB)
		require.ErrorIs(t, err, persistence.ErrDepartmentNotFound)

		_, err = departmentRepository.GetByID(ctxB, idA)
		require.ErrorIs(t, err, persistence.ErrDepartmentNotFound)

		// Each tenant reads its own row fine.
		gotA, err := departmentRepository.GetByID(f.Ctx, idA)
		require.NoError(t, err)
		assert.Equal(t, tenantA, gotA.TenantID())

		gotB, err := departmentRepository.GetByID(ctxB, idB)
		require.NoError(t, err)
		assert.Equal(t, secondTenant.ID, gotB.TenantID())
	})

	t.Run("Exists is tenant scoped", func(t *testing.T) {
		exists, err := departmentRepository.Exists(f.Ctx, idB)
		require.NoError(t, err)
		assert.False(t, exists, "tenant A must not see tenant B's department")

		exists, err = departmentRepository.Exists(ctxB, idB)
		require.NoError(t, err)
		assert.True(t, exists)
	})

	t.Run("GetPaginated excludes other tenant rows", func(t *testing.T) {
		listA, err := departmentRepository.GetPaginated(f.Ctx, &department.FindParams{Limit: 1000})
		require.NoError(t, err)
		for _, d := range listA {
			assert.Equal(t, tenantA, d.TenantID(), "tenant A list must not contain tenant B rows")
			assert.NotEqual(t, idB, d.ID())
		}

		listB, err := departmentRepository.GetPaginated(ctxB, &department.FindParams{Limit: 1000})
		require.NoError(t, err)
		require.Len(t, listB, 1)
		assert.Equal(t, idB, listB[0].ID())
	})
}

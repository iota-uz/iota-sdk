package persistence_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/department"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/userposition"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/itf"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPgUserPositionRepository_CRUD(t *testing.T) {
	t.Parallel()
	f := setupTest(t)

	uploadRepository := persistence.NewUploadRepository()
	userRepository := persistence.NewUserRepository(uploadRepository)
	departmentRepository := persistence.NewDepartmentRepository()
	positionRepository := persistence.NewUserPositionRepository()

	tenant, err := composables.UseTenantID(f.Ctx)
	require.NoError(t, err)

	email, err := internet.NewEmail("position-test@gmail.com")
	require.NoError(t, err)
	createdUser, err := userRepository.Create(f.Ctx, user.New(
		"Pos",
		"Tester",
		email,
		user.UILanguageEN,
		user.WithTenantID(tenant),
	))
	require.NoError(t, err)

	deptID := uuid.New()
	_, err = departmentRepository.Save(f.Ctx, department.New(
		"POS-DEPT",
		orgName(t, "Position Department"),
		department.WithID(deptID),
		department.WithTenantID(tenant),
	))
	require.NoError(t, err)

	t.Run("CreateManagerPosition", func(t *testing.T) {
		id := uuid.New()
		entity := userposition.New(
			createdUser.ID(),
			deptID,
			orgName(t, "Head of Department"),
			userposition.WithID(id),
			userposition.WithTenantID(tenant),
			userposition.WithIsManager(true),
			userposition.WithIsPrimary(true),
		)

		saved, err := positionRepository.Save(f.Ctx, entity)
		require.NoError(t, err)
		assert.Equal(t, id, saved.ID())
		assert.Equal(t, createdUser.ID(), saved.UserID())
		assert.Equal(t, deptID, saved.DepartmentID())
		assert.True(t, saved.IsManager())
		assert.True(t, saved.IsPrimary())
		assert.Equal(t, "Head of Department EN", saved.Title("en"))
		assert.Equal(t, "Head of Department UZ-CYRL", saved.Title("uz-Cyrl"))
		assert.Equal(t, userposition.StatusActive, saved.Status())

		retrieved, err := positionRepository.GetByID(f.Ctx, id)
		require.NoError(t, err)
		assert.True(t, retrieved.IsManager())
		assert.Equal(t, "Head of Department RU", retrieved.Title("ru"))
	})

	t.Run("Update", func(t *testing.T) {
		id := uuid.New()
		entity := userposition.New(
			createdUser.ID(),
			deptID,
			orgName(t, "Engineer"),
			userposition.WithID(id),
			userposition.WithTenantID(tenant),
		)
		saved, err := positionRepository.Save(f.Ctx, entity)
		require.NoError(t, err)
		assert.False(t, saved.IsManager())

		updated := saved.SetIsManager(true).SetTitle(orgName(t, "Senior Engineer"))
		savedUpdated, err := positionRepository.Save(f.Ctx, updated)
		require.NoError(t, err)
		assert.True(t, savedUpdated.IsManager())
		assert.Equal(t, "Senior Engineer EN", savedUpdated.Title("en"))
	})

	t.Run("ExistsAndDelete", func(t *testing.T) {
		id := uuid.New()
		entity := userposition.New(
			createdUser.ID(),
			deptID,
			orgName(t, "Temp"),
			userposition.WithID(id),
			userposition.WithTenantID(tenant),
		)
		_, err := positionRepository.Save(f.Ctx, entity)
		require.NoError(t, err)

		exists, err := positionRepository.Exists(f.Ctx, id)
		require.NoError(t, err)
		assert.True(t, exists)

		require.NoError(t, positionRepository.Delete(f.Ctx, id))

		_, err = positionRepository.GetByID(f.Ctx, id)
		require.ErrorIs(t, err, persistence.ErrUserPositionNotFound)
	})
}

func TestPgUserPositionRepository_TenantIsolation(t *testing.T) {
	t.Parallel()
	f := setupTest(t)

	uploadRepository := persistence.NewUploadRepository()
	userRepository := persistence.NewUserRepository(uploadRepository)
	departmentRepository := persistence.NewDepartmentRepository()
	positionRepository := persistence.NewUserPositionRepository()

	tenantA, err := composables.UseTenantID(f.Ctx)
	require.NoError(t, err)
	secondTenant, err := itf.CreateTestTenant(f.Ctx, f.Pool)
	require.NoError(t, err)
	ctxB := composables.WithTenantID(f.Ctx, secondTenant.ID)

	// Per-tenant user + department + position.
	mkPosition := func(ctx context.Context, tid uuid.UUID, emailStr, deptCode string) uuid.UUID {
		email, err := internet.NewEmail(emailStr)
		require.NoError(t, err)
		u, err := userRepository.Create(ctx, user.New(
			"Iso", "User", email, user.UILanguageEN, user.WithTenantID(tid),
		))
		require.NoError(t, err)

		deptID := uuid.New()
		_, err = departmentRepository.Save(ctx, department.New(
			deptCode, orgName(t, deptCode),
			department.WithID(deptID), department.WithTenantID(tid),
		))
		require.NoError(t, err)

		posID := uuid.New()
		_, err = positionRepository.Save(ctx, userposition.New(
			u.ID(), deptID, orgName(t, "Engineer"),
			userposition.WithID(posID), userposition.WithTenantID(tid),
		))
		require.NoError(t, err)
		return posID
	}

	posA := mkPosition(f.Ctx, tenantA, "iso-pos-a@gmail.com", "ISO-A")
	posB := mkPosition(ctxB, secondTenant.ID, "iso-pos-b@gmail.com", "ISO-B")

	t.Run("GetByID does not cross tenants", func(t *testing.T) {
		_, err := positionRepository.GetByID(f.Ctx, posB)
		require.ErrorIs(t, err, persistence.ErrUserPositionNotFound)

		_, err = positionRepository.GetByID(ctxB, posA)
		require.ErrorIs(t, err, persistence.ErrUserPositionNotFound)

		gotA, err := positionRepository.GetByID(f.Ctx, posA)
		require.NoError(t, err)
		assert.Equal(t, tenantA, gotA.TenantID())
	})

	t.Run("GetPaginated excludes other tenant rows", func(t *testing.T) {
		listA, err := positionRepository.GetPaginated(f.Ctx, &userposition.FindParams{Limit: 1000})
		require.NoError(t, err)
		for _, p := range listA {
			assert.Equal(t, tenantA, p.TenantID())
			assert.NotEqual(t, posB, p.ID())
		}

		listB, err := positionRepository.GetPaginated(ctxB, &userposition.FindParams{Limit: 1000})
		require.NoError(t, err)
		require.Len(t, listB, 1)
		assert.Equal(t, posB, listB[0].ID())
	})
}

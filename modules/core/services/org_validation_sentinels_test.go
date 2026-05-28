package services_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/department"
	persistence "github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/stretchr/testify/require"
)

// TestDepartmentValidationSentinels guards the error contract the SDK exposes
// to admin-CRUD controllers: every well-known validation/conflict failure
// surfaces as a sentinel that can be matched with errors.Is, so controllers can
// render a per-field i18n message instead of leaking a 500. Add a sentinel +
// case here whenever a new write-path invariant is introduced.
func TestDepartmentValidationSentinels(t *testing.T) {
	t.Parallel()
	f := setupTest(t)
	svc, repo, _ := newDepartmentService()

	ctx := withActor(f.Ctx)
	tenant, err := composables.UseTenantID(ctx)
	require.NoError(t, err)

	// Build A -> B so we can test cycle (parent = own descendant) and a
	// non-existent parent against a real tenant.
	aID := uuid.New()
	a, err := svc.Create(ctx, department.New(
		"SENT-A", orgName(t, "SentA"),
		department.WithID(aID), department.WithTenantID(tenant),
	))
	require.NoError(t, err)

	bID := uuid.New()
	_, err = svc.Create(ctx, department.New(
		"SENT-B", orgName(t, "SentB"),
		department.WithID(bID), department.WithTenantID(tenant),
		department.WithParentID(&aID),
	))
	require.NoError(t, err)

	t.Run("self-loop wraps ErrDepartmentSelfLoop", func(t *testing.T) {
		_, err := svc.Update(ctx, a.SetParentID(&aID))
		require.Error(t, err)
		require.ErrorIs(t, err, services.ErrDepartmentSelfLoop)
	})

	t.Run("cycle wraps ErrDepartmentCycle", func(t *testing.T) {
		// A is root; making B (its descendant) the parent creates a cycle.
		_, err := svc.Update(ctx, a.SetParentID(&bID))
		require.Error(t, err)
		require.ErrorIs(t, err, services.ErrDepartmentCycle)
	})

	t.Run("missing parent wraps ErrDepartmentParentNotFound", func(t *testing.T) {
		ghost := uuid.New()
		_, err := svc.Create(ctx, department.New(
			"SENT-MISS", orgName(t, "Missing"),
			department.WithID(uuid.New()), department.WithTenantID(tenant),
			department.WithParentID(&ghost),
		))
		require.Error(t, err)
		require.ErrorIs(t, err, services.ErrDepartmentParentNotFound)
	})

	t.Run("duplicate code wraps persistence.ErrDepartmentDuplicateCode", func(t *testing.T) {
		// Save a row with code SENT-DUP, then try to save a *different* aggregate
		// with the same code in the same tenant — the unique constraint fires.
		first := department.New(
			"SENT-DUP", orgName(t, "First"),
			department.WithID(uuid.New()), department.WithTenantID(tenant),
		)
		_, err := repo.Save(ctx, first)
		require.NoError(t, err)

		dup := department.New(
			"SENT-DUP", orgName(t, "Second"),
			department.WithID(uuid.New()), department.WithTenantID(tenant),
		)
		_, err = repo.Save(ctx, dup)
		require.Error(t, err)
		require.ErrorIs(t, err, persistence.ErrDepartmentDuplicateCode)
	})
}

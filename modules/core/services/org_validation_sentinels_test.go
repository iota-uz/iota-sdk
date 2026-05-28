package services_test

import (
	"context"
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
// table entry here whenever a new write-path invariant is introduced.
func TestDepartmentValidationSentinels(t *testing.T) {
	t.Parallel()
	f := setupTest(t)
	svc, repo, _ := newDepartmentService()

	ctx := withActor(f.Ctx)
	tenant, err := composables.UseTenantID(ctx)
	require.NoError(t, err)

	// Shared topology A -> B used by the SelfLoop/Cycle cases so the cycle
	// detector has a real descendant subtree to walk.
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

	type sentinelCase struct {
		name     string
		setup    func(t *testing.T) // optional pre-state for the action
		action   func(ctx context.Context) error
		expected error
	}

	cases := []sentinelCase{
		{
			name: "SelfLoop",
			action: func(ctx context.Context) error {
				_, err := svc.Update(ctx, a.SetParentID(&aID))
				return err
			},
			expected: services.ErrDepartmentSelfLoop,
		},
		{
			name: "Cycle",
			action: func(ctx context.Context) error {
				// A is root; making B (its descendant) the parent creates a cycle.
				_, err := svc.Update(ctx, a.SetParentID(&bID))
				return err
			},
			expected: services.ErrDepartmentCycle,
		},
		{
			name: "MissingParent",
			action: func(ctx context.Context) error {
				ghost := uuid.New()
				_, err := svc.Create(ctx, department.New(
					"SENT-MISS", orgName(t, "Missing"),
					department.WithID(uuid.New()), department.WithTenantID(tenant),
					department.WithParentID(&ghost),
				))
				return err
			},
			expected: services.ErrDepartmentParentNotFound,
		},
		{
			name: "DuplicateCode",
			setup: func(t *testing.T) {
				t.Helper()
				// Save the first row that "claims" the SENT-DUP code; the action
				// then tries to insert a different aggregate with the same code in
				// the same tenant, hitting the unique constraint.
				first := department.New(
					"SENT-DUP", orgName(t, "First"),
					department.WithID(uuid.New()), department.WithTenantID(tenant),
				)
				_, err := repo.Save(ctx, first)
				require.NoError(t, err)
			},
			action: func(ctx context.Context) error {
				dup := department.New(
					"SENT-DUP", orgName(t, "Second"),
					department.WithID(uuid.New()), department.WithTenantID(tenant),
				)
				_, err := repo.Save(ctx, dup)
				return err
			},
			expected: persistence.ErrDepartmentDuplicateCode,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.setup != nil {
				tc.setup(t)
			}
			err := tc.action(ctx)
			require.Error(t, err)
			require.ErrorIs(t, err, tc.expected)
		})
	}
}

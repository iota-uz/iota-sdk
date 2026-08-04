package services_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/query"
	"github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/itf"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestOrgQueryService_NoPermissionRequired is a regression for the bug found
// during PR #2893 live testing: OrgQueryService used to gate every method on
// `permissions.DepartmentRead`, which silently broke EDO dept-scoped visibility
// for any user whose role lacked that permission. The fix removes the gate at
// this service layer — admin Department CRUD still gates through its own
// controller, but internal resolvers (EDO AccessResolver, future HR/CRM
// resolvers) read structural data without forcing every end-user role to carry
// `DepartmentRead`. Tenant isolation lives in the repository layer.
func TestOrgQueryService_NoPermissionRequired(t *testing.T) {
	t.Parallel()
	f := setupTest(t)

	svc := services.NewOrgQueryService(query.NewPgOrgQueryRepository())

	// Actor has NO DepartmentRead (no permissions at all beyond what the empty
	// itf.User grants). Before the fix this would yield ErrForbidden on every
	// call; after the fix all three methods return a clean empty result.
	unprivilegedActor := itf.User()
	ctx := composables.WithUser(f.Ctx, unprivilegedActor)

	t.Run("UserDepartments returns no error", func(t *testing.T) {
		ids, err := svc.UserDepartments(ctx, unprivilegedActor.ID())
		require.NoError(t, err)
		// No positions seeded for this throwaway user → empty slice, not nil-error.
		assert.Empty(t, ids)
	})

	t.Run("UserManagedDepartments returns no error", func(t *testing.T) {
		ids, err := svc.UserManagedDepartments(ctx, unprivilegedActor.ID(), true)
		require.NoError(t, err)
		assert.Empty(t, ids)
	})

	t.Run("DepartmentSubtree returns no error", func(t *testing.T) {
		// Random UUID for a non-existent dept — the call must still succeed
		// (returns empty subtree), not 403.
		ids, err := svc.DepartmentSubtree(ctx, uuid.New())
		require.NoError(t, err)
		assert.Empty(t, ids)
	})
}

package interop

import (
	"context"

	"github.com/iota-uz/iota-sdk/modules/bichat/permissions"
	"github.com/iota-uz/iota-sdk/pkg/composables"
)

// getUserPermissions returns a list of permission strings for the current user
func getUserPermissions(ctx context.Context) []string {
	perms := []string{}

	if composables.CanUser(ctx, permissions.BiChatAccess) == nil {
		perms = append(perms, "bichat.access")
	}
	if composables.CanUser(ctx, permissions.BiChatReadAll) == nil {
		perms = append(perms, "bichat.read_all")
	}
	if composables.CanUser(ctx, permissions.BiChatExport) == nil {
		perms = append(perms, "bichat.export")
	}

	return perms
}

package application

import (
	"testing"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	"github.com/iota-uz/iota-sdk/modules/core/permissions"
	"github.com/stretchr/testify/require"
)

func TestCanReceiveTenantUpdates(t *testing.T) {
	tenant := uuid.New()
	email, err := internet.NewEmail("ws@example.com")
	require.NoError(t, err)
	cases := []struct {
		name   string
		tenant uuid.UUID
		perms  []permission.Permission
		status user.Status
		kind   user.Type
		want   bool
	}{
		{"matching tenant and permission", tenant, []permission.Permission{permissions.UserRead}, user.StatusActive, user.TypeUser, true},
		{"other tenant", uuid.New(), []permission.Permission{permissions.UserRead}, user.StatusActive, user.TypeUser, false},
		{"missing permission", tenant, nil, user.StatusActive, user.TypeUser, false},
		{"inactive account", tenant, []permission.Permission{permissions.UserRead}, user.StatusPendingOnboarding, user.TypeUser, false},
		{"system account", tenant, []permission.Permission{permissions.UserRead}, user.StatusActive, user.TypeSystem, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			u := user.New("WS", "User", email, user.UILanguageEN, user.WithTenantID(tc.tenant), user.WithPermissions(tc.perms), user.WithStatus(tc.status), user.WithType(tc.kind))
			require.Equal(t, tc.want, CanReceiveTenantUpdates(u, tenant, permissions.UserRead))
		})
	}
	require.False(t, CanReceiveTenantUpdates(nil, tenant, permissions.UserRead))
	blocked := user.New("WS", "Blocked", email, user.UILanguageEN, user.WithTenantID(tenant), user.WithPermissions([]permission.Permission{permissions.UserRead}), user.WithIsBlocked(true))
	require.False(t, CanReceiveTenantUpdates(blocked, tenant, permissions.UserRead))
}

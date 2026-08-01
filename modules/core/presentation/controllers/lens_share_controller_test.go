package controllers

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/role"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	"github.com/iota-uz/iota-sdk/modules/core/permissions"
	"github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	lensshare "github.com/iota-uz/iota-sdk/pkg/lens/share"
	"github.com/stretchr/testify/require"
)

type lensShareRoleRepository struct {
	roles  []role.Role
	params *role.FindParams
}

func (r *lensShareRoleRepository) Count(context.Context, *role.FindParams) (int64, error) {
	return int64(len(r.roles)), nil
}
func (r *lensShareRoleRepository) GetAll(context.Context) ([]role.Role, error) { return r.roles, nil }
func (r *lensShareRoleRepository) GetPaginated(_ context.Context, params *role.FindParams) ([]role.Role, error) {
	r.params = params
	return r.roles, nil
}
func (r *lensShareRoleRepository) GetByID(context.Context, uint) (role.Role, error) { return nil, nil }
func (r *lensShareRoleRepository) Create(_ context.Context, value role.Role) (role.Role, error) {
	return value, nil
}
func (r *lensShareRoleRepository) Update(_ context.Context, value role.Role) (role.Role, error) {
	return value, nil
}
func (r *lensShareRoleRepository) Delete(context.Context, uint) error { return nil }

func TestLensShareAccessUsesTenantAndEffectivePermissions(t *testing.T) {
	t.Parallel()
	tenantID := uuid.New()
	actorRole := role.New("Analyst", role.WithID(7), role.WithTenantID(tenantID))
	repository := &lensShareRoleRepository{roles: []role.Role{
		role.New("Manager", role.WithID(9), role.WithTenantID(tenantID)),
	}}
	service := services.NewRoleService(repository, nil)
	actor := user.New("Lens", "Admin", internet.MustParseEmail("lens@example.com"), user.UILanguageEN,
		user.WithID(42), user.WithTenantID(tenantID), user.WithRoles([]role.Role{actorRole}),
		user.WithPermissions([]permission.Permission{permissions.LensViewTeamManage, permissions.LensExportSchedule}),
	)
	request := httptest.NewRequest("GET", "/lens/share/views", nil)
	request = request.WithContext(composables.WithUser(composables.WithTenantID(request.Context(), tenantID), actor))

	access, err := lensShareAccess(service)(request)
	require.NoError(t, err)
	require.Equal(t, tenantID, access.TenantID)
	require.Equal(t, uint(42), access.UserID)
	require.Equal(t, []uint{7}, access.RoleIDs)
	require.True(t, access.ManageTeam)
	require.True(t, access.ScheduleMail)
	require.Equal(t, []lensshare.RoleRef{{ID: 9, Name: "Manager"}}, access.Roles)
	require.NotNil(t, repository.params)
	require.Equal(t, 200, repository.params.Limit)
	require.Len(t, repository.params.Filters, 1)
}

func TestLensShareAccessRejectsTenantMismatch(t *testing.T) {
	t.Parallel()
	actorTenant := uuid.New()
	actor := user.New("Lens", "User", internet.MustParseEmail("user@example.com"), user.UILanguageEN,
		user.WithID(42), user.WithTenantID(actorTenant),
	)
	request := httptest.NewRequest("GET", "/lens/share/views", nil)
	request = request.WithContext(composables.WithUser(composables.WithTenantID(request.Context(), uuid.New()), actor))

	_, err := lensShareAccess(services.NewRoleService(&lensShareRoleRepository{}, nil))(request)
	require.ErrorContains(t, err, "actor tenant does not match")
}

func TestLensShareAccessDoesNotAdvertiseCapabilitiesWithoutPermissions(t *testing.T) {
	t.Parallel()
	tenantID := uuid.New()
	actor := user.New("Lens", "Viewer", internet.MustParseEmail("viewer@example.com"), user.UILanguageEN,
		user.WithID(43), user.WithTenantID(tenantID),
	)
	repository := &lensShareRoleRepository{}
	request := httptest.NewRequest("GET", "/lens/share/views", nil)
	request = request.WithContext(composables.WithUser(composables.WithTenantID(request.Context(), tenantID), actor))

	access, err := lensShareAccess(services.NewRoleService(repository, nil))(request)
	require.NoError(t, err)
	require.False(t, access.ManageTeam)
	require.False(t, access.ScheduleMail)
	require.Nil(t, repository.params, "roles must not be queried for viewers who cannot manage team views")
}

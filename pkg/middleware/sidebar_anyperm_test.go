package middleware

import (
	"testing"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	"github.com/iota-uz/iota-sdk/pkg/types"
	"github.com/stretchr/testify/require"
)

// permA / permB model the eai "Portfolio.Read" and "Reinsurance.Contract.Read"
// permissions that gate the Portfolio nav item with Logic: Any.
var (
	permA = permission.MustCreate(uuid.MustParse("019db8a0-2001-7000-8000-000000000001"), "Portfolio.Read", "portfolio", "read", permission.ModifierAll)
	permB = permission.MustCreate(uuid.MustParse("019db8a0-3001-7000-8000-000000000002"), "Reinsurance.Contract.Read", "reinsurance_contract", "read", permission.ModifierAll)
)

func userWith(perms ...permission.Permission) user.User {
	return user.New("Test", "User", internet.MustParseEmail("test@example.com"), user.UILanguageEN, user.WithPermissions(perms))
}

// A user holding only permA must still see a nav item gated Any[A, B].
func TestFilterItems_AnyLogic_LeafVisibleWithOnePermission(t *testing.T) {
	t.Parallel()
	items := []types.NavigationItem{
		{Key: "portfolio", Href: "/portfolio/policies", Permissions: []permission.Permission{permA, permB}, Logic: types.PermissionLogicAny},
	}
	out := filterItems(items, userWith(permA))
	require.Equal(t, []string{"portfolio"}, keysOf(out), "Any[A,B] leaf must be visible to a user holding A")
	require.Equal(t, types.PermissionLogicAny, out[0].Logic, "filterItems must preserve Logic on surviving items")
}

// The real eai shape: a parent gated Any[A, B] whose children each require one
// permission. A user with only A should see the parent with its A-children.
func TestFilterItems_AnyLogic_ParentVisibleWithSurvivingChildren(t *testing.T) {
	t.Parallel()
	items := []types.NavigationItem{
		{
			Key:         "portfolio",
			Href:        "/portfolio/policies",
			Permissions: []permission.Permission{permA, permB},
			Logic:       types.PermissionLogicAny,
			Children: []types.NavigationItem{
				{Key: "portfolio.direct", Href: "/portfolio/policies", Permissions: []permission.Permission{permA}},
				{Key: "reinsurance", Href: "/reinsurance/contracts/outgoing", Permissions: []permission.Permission{permB}},
				{Key: "portfolio.archive", Href: "/portfolio/archive", Permissions: []permission.Permission{permA}},
			},
		},
	}
	out := filterItems(items, userWith(permA))
	require.Equal(t, []string{"portfolio"}, keysOf(out), "parent must survive when it has surviving children")
	require.Equal(t, []string{"portfolio.direct", "portfolio.archive"}, keysOf(out[0].Children))
	require.Equal(t, types.PermissionLogicAny, out[0].Logic, "filterItems must preserve parent Logic")
}

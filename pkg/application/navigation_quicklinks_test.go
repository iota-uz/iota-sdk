package application

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/iota-uz/iota-sdk/pkg/spotlight"
	"github.com/iota-uz/iota-sdk/pkg/types"
	"github.com/stretchr/testify/require"
)

func TestApplication_RegisterNavItems_AddsQuickLinks(t *testing.T) {
	app, err := New(&ApplicationOptions{
		Bundle:             LoadBundle(),
		SupportedLanguages: []string{"en"},
	})
	require.NoError(t, err)

	perms := []permission.Permission{
		permission.New(permission.WithName("users.read")),
	}

	app.RegisterNavItems(
		types.NavigationItem{
			Name:        "NavigationLinks.Users",
			Href:        "/users",
			Keywords:    []string{"staff", "directory"},
			Permissions: perms,
			Children: []types.NavigationItem{
				{
					Name: "NavigationLinks.UserSessions",
					Href: "/account/sessions",
				},
			},
		},
	)

	docs, err := spotlight.CollectDocuments(context.Background(), app.QuickLinks(), typesProviderScope())
	require.NoError(t, err)
	require.Len(t, docs, 2)

	require.Equal(t, "/users", docs[0].URL)
	require.Equal(t, []string{"users.read"}, docs[0].Access.AllowedPermissions)
	require.Contains(t, docs[0].Body, "staff")
	require.Contains(t, docs[0].Body, "directory")
	require.Contains(t, docs[0].Body, "navigationlinks")
	require.Contains(t, docs[0].Body, "users")

	require.Equal(t, "/account/sessions", docs[1].URL)
	require.Equal(t, "public", string(docs[1].Access.Visibility))
	require.Contains(t, docs[1].Body, "account")
	require.Contains(t, docs[1].Body, "sessions")
}

func TestDefaultSupportedLanguages(t *testing.T) {
	require.Equal(t, []string{"en", "ru", "uz", "zh"}, DefaultSupportedLanguages())
}

func TestApplication_AppendNavChildren_AddsQuickLinksForChildren(t *testing.T) {
	app, err := New(&ApplicationOptions{
		Bundle:             LoadBundle(),
		SupportedLanguages: []string{"en"},
	})
	require.NoError(t, err)

	app.RegisterNavItems(types.NavigationItem{
		Name: "NavigationLinks.Parent",
		Href: "/parent",
	})
	app.AppendNavChildren("NavigationLinks.Parent", types.NavigationItem{
		Name:     "NavigationLinks.Child",
		Href:     "/parent/child?tab=details",
		Keywords: []string{"deep", "child"},
	})

	docs, err := spotlight.CollectDocuments(context.Background(), app.QuickLinks(), typesProviderScope())
	require.NoError(t, err)
	require.Len(t, docs, 2)
	require.Equal(t, "/parent/child?tab=details", docs[1].URL)
	require.Contains(t, docs[1].Body, "deep")
	require.Contains(t, docs[1].Body, "child")
	require.Contains(t, docs[1].Body, "tab")
	require.Contains(t, docs[1].Body, "details")
}

func typesProviderScope() spotlight.ProviderScope {
	return spotlight.ProviderScope{
		TenantID: uuid.New(),
		Language: "en",
	}
}

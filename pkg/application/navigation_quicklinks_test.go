package application

import (
	"context"
	"embed"
	"testing"

	"github.com/benbjohnson/hashfs"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/iota-uz/iota-sdk/pkg/spotlight"
	"github.com/iota-uz/iota-sdk/pkg/types"
	"github.com/stretchr/testify/require"
)

type testRuntimeSource struct {
	navItems []types.NavigationItem
	applets  []Applet
}

func (s *testRuntimeSource) Controllers() []Controller          { return nil }
func (s *testRuntimeSource) Middleware() []mux.MiddlewareFunc   { return nil }
func (s *testRuntimeSource) Assets() []*embed.FS                { return nil }
func (s *testRuntimeSource) HashFSAssets() []*hashfs.FS         { return nil }
func (s *testRuntimeSource) LocaleFiles() []*embed.FS           { return nil }
func (s *testRuntimeSource) GraphSchemas() []GraphSchema        { return nil }
func (s *testRuntimeSource) Applets() []Applet                  { return s.applets }
func (s *testRuntimeSource) NavItems() []types.NavigationItem   { return s.navItems }
func (s *testRuntimeSource) QuickLinks() []*spotlight.QuickLink { return nil }
func (s *testRuntimeSource) SpotlightProviders() []spotlight.SearchProvider {
	return nil
}

func attachRuntimeSource(t *testing.T, app Application, source RuntimeSource) {
	t.Helper()
	binder, ok := app.(RuntimeBinder)
	require.True(t, ok, "application must support runtime binding")
	require.NoError(t, binder.AttachRuntimeSource(source))
}

func TestApplication_AttachRuntimeSource_AddsQuickLinks(t *testing.T) {
	app, err := New(&ApplicationOptions{
		Bundle:             LoadBundle(),
		SupportedLanguages: []string{"en"},
	})
	require.NoError(t, err)

	perms := []permission.Permission{
		permission.New(permission.WithName("users.read")),
	}

	attachRuntimeSource(t, app, &testRuntimeSource{
		navItems: []types.NavigationItem{{
			Name:        "NavigationLinks.Users",
			Href:        "/users",
			Keywords:    []string{"staff", "directory"},
			Permissions: perms,
			Children: []types.NavigationItem{{
				Name: "NavigationLinks.UserSessions",
				Href: "/account/sessions",
			}},
		}},
	})

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

func TestApplication_AttachRuntimeSource_AddsNestedQuickLinksForChildren(t *testing.T) {
	app, err := New(&ApplicationOptions{
		Bundle:             LoadBundle(),
		SupportedLanguages: []string{"en"},
	})
	require.NoError(t, err)

	attachRuntimeSource(t, app, &testRuntimeSource{
		navItems: []types.NavigationItem{{
			Name: "NavigationLinks.Parent",
			Href: "/parent",
			Children: []types.NavigationItem{{
				Name:     "NavigationLinks.Child",
				Href:     "/parent/child?tab=details",
				Keywords: []string{"deep", "child"},
			}},
		}},
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

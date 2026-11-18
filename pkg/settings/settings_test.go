package settings_test

import (
	"context"
	"testing"

	"github.com/iota-uz/go-i18n/v2/i18n"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/iota-uz/iota-sdk/modules/core/permissions"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/intl"
	"github.com/iota-uz/iota-sdk/pkg/itf"
	"github.com/iota-uz/iota-sdk/pkg/settings"
	"github.com/iota-uz/iota-sdk/pkg/types"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/language"
)

func registry() settings.Source {
	return settings.SourceFunc(func() []types.SettingsSection {
		return []types.SettingsSection{
			{Key: "general", Name: "Hub.General", Pages: []types.SettingsPage{
				{Key: "open", Name: "Hub.Open", Description: "Hub.Open.Summary", Href: "/open"},
				{Key: "literal", Name: "Already translated", Href: "/literal"},
			}},
			{Key: "access", Name: "Hub.Access", Pages: []types.SettingsPage{
				{Key: "users", Name: "Users", Href: "/users", Permissions: []permission.Permission{permissions.UserRead}},
				{Key: "all", Name: "All", Href: "/all", Permissions: []permission.Permission{permissions.UserRead, permissions.RoleRead}},
				{Key: "any", Name: "Any", Href: "/any", Logic: types.PermissionLogicAny, Permissions: []permission.Permission{permissions.GroupRead, permissions.RoleRead}},
			}},
			{Key: "restricted", Name: "Restricted", Pages: []types.SettingsPage{
				{Key: "roles", Name: "Roles", Href: "/roles", Permissions: []permission.Permission{permissions.RoleRead}},
			}},
		}
	})
}

func pageKeys(sections []settings.ResolvedSection) map[string][]string {
	out := make(map[string][]string, len(sections))
	for _, section := range sections {
		for _, page := range section.Pages {
			out[section.Key] = append(out[section.Key], page.Key)
		}
	}
	return out
}

// TestResolve_FiltersByPermission is falsely green if HasPermission ignored
// Logic: the "all" and "any" pages need different permission sets to differ.
func TestResolve_FiltersByPermission(t *testing.T) {
	cases := []struct {
		name  string
		perms []permission.Permission
		want  map[string][]string
	}{
		{
			name:  "no permissions sees only open pages",
			perms: nil,
			want:  map[string][]string{"general": {"open", "literal"}},
		},
		{
			name:  "single permission satisfies all-of with one entry, not two",
			perms: []permission.Permission{permissions.UserRead},
			want:  map[string][]string{"general": {"open", "literal"}, "access": {"users"}},
		},
		{
			name:  "one of the any-of permissions is enough",
			perms: []permission.Permission{permissions.GroupRead},
			want:  map[string][]string{"general": {"open", "literal"}, "access": {"any"}},
		},
		{
			name:  "full permission set sees everything",
			perms: []permission.Permission{permissions.UserRead, permissions.RoleRead},
			want: map[string][]string{
				"general":    {"open", "literal"},
				"access":     {"users", "all", "any"},
				"restricted": {"roles"},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := composables.WithUser(context.Background(), itf.User(tc.perms...))
			require.Equal(t, tc.want, pageKeys(settings.Resolve(ctx, registry())))
		})
	}
}

func TestResolve_WithoutUserIsEmpty(t *testing.T) {
	require.Empty(t, settings.Resolve(context.Background(), registry()))
	require.Empty(t, settings.Resolve(composables.WithUser(context.Background(), itf.User()), nil))
}

// TestResolve_TranslatesKeysAndKeepsLiterals is falsely green if Resolve
// returned raw keys and the bundle happened to define keys equal to their text.
func TestResolve_TranslatesKeysAndKeepsLiterals(t *testing.T) {
	bundle := i18n.NewBundle(language.English)
	require.NoError(t, bundle.AddMessages(language.English,
		&i18n.Message{ID: "Hub.General", Other: "General settings"},
		&i18n.Message{ID: "Hub.Open", Other: "Open page"},
		&i18n.Message{ID: "Hub.Open.Summary", Other: "Visible to everyone"},
	))
	ctx := intl.WithLocalizer(context.Background(), i18n.NewLocalizer(bundle, "en"))
	ctx = composables.WithUser(ctx, itf.User())

	sections := settings.Resolve(ctx, registry())
	require.Len(t, sections, 1)
	require.Equal(t, "General settings", sections[0].Name)
	require.Equal(t, settings.ResolvedPage{Key: "open", Name: "Open page", Description: "Visible to everyone", Href: "/open"}, sections[0].Pages[0])
	require.Equal(t, "Already translated", sections[0].Pages[1].Name)
	require.Empty(t, sections[0].Pages[1].Description)
}

func TestResolveFromContext_RequiresContainer(t *testing.T) {
	_, err := settings.ResolveFromContext(context.Background())
	require.Error(t, err)
}

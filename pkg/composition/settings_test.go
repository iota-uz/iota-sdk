package composition

import (
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/types"
	"github.com/stretchr/testify/require"
)

func settingsPageKeys(sections []types.SettingsSection) map[string][]string {
	out := make(map[string][]string, len(sections))
	for _, section := range sections {
		for _, page := range section.Pages {
			out[section.Key] = append(out[section.Key], page.Key)
		}
	}
	return out
}

func settingsSectionKeys(sections []types.SettingsSection) []string {
	out := make([]string, 0, len(sections))
	for _, section := range sections {
		out = append(out, section.Key)
	}
	return out
}

// TestEngineCompileSettingsMergesSortsAndRemoves is falsely green if the
// container returned the contributions verbatim and the assertions only
// checked counts; it checks exact keys and order instead.
func TestEngineCompileSettingsMergesSortsAndRemoves(t *testing.T) {
	engine := NewEngine()
	err := engine.Register(
		testComponent{
			descriptor: Descriptor{Name: "core"},
			build: func(builder *Builder) error {
				AddSettings(builder,
					types.SettingsSection{Key: "access", Name: "Access", Priority: 20, Pages: []types.SettingsPage{
						{Key: "core.users", Name: "Users", Href: "/users", Order: 10},
						{Key: "core.roles", Name: "Roles", Href: "/roles", Order: 20},
					}},
					types.SettingsSection{Key: "general", Name: "General", Priority: 10, Pages: []types.SettingsPage{
						{Key: "core.logo", Name: "Logo", Href: "/settings/logo", Order: 10},
					}},
					types.SettingsSection{Key: "legacy", Name: "Legacy", Priority: 5, Pages: []types.SettingsPage{
						{Key: "core.legacy", Name: "Legacy", Href: "/legacy"},
					}},
				)
				return nil
			},
		},
		testComponent{
			descriptor: Descriptor{Name: "audit", Requires: []string{"core"}},
			build: func(builder *Builder) error {
				ContributeSettings(builder, func(*Container) ([]types.SettingsSection, error) {
					return []types.SettingsSection{{Key: "access", Pages: []types.SettingsPage{
						{Key: "audit.history", Name: "History", Href: "/history", Order: 15},
						{Key: "audit.archive", Name: "Archive", Href: "/archive", Order: 15},
					}}}, nil
				})
				return nil
			},
		},
		testComponent{
			descriptor: Descriptor{Name: "host", Requires: []string{"audit"}},
			build: func(builder *Builder) error {
				RemoveSettingsPagesByKey(builder, "core.roles", "core.legacy", "")
				AddSettings(builder, types.SettingsSection{Key: "general", Name: "Host general", Priority: 1, Pages: []types.SettingsPage{
					{Key: "host.billing", Name: "Billing", Href: "/billing", Order: 5},
				}})
				return nil
			},
		},
	)
	require.NoError(t, err)

	container, err := engine.Compile(BuildContext{})
	require.NoError(t, err)

	sections := container.SettingsSections()
	require.Equal(t, []string{"general", "access"}, settingsSectionKeys(sections), "sections sort by priority, emptied section is omitted")
	require.Equal(t, "General", sections[0].Name, "first contribution owns the section name")
	require.Equal(t, 10, sections[0].Priority, "first contribution owns the section priority")
	require.Equal(t, map[string][]string{
		"general": {"host.billing", "core.logo"},
		"access":  {"core.users", "audit.archive", "audit.history"},
	}, settingsPageKeys(sections), "pages merge by section key and sort by order then key")

	sections[0].Pages[0].Name = "mutated"
	require.Equal(t, "Billing", container.SettingsSections()[0].Pages[0].Name, "accessor returns a copy")
}

func TestEngineCompileSettingsRejectsInvalidContributions(t *testing.T) {
	cases := []struct {
		name    string
		section types.SettingsSection
		extra   *types.SettingsSection
		message string
	}{
		{
			name:    "section without key",
			section: types.SettingsSection{Name: "Nameless", Pages: []types.SettingsPage{{Key: "a", Href: "/a"}}},
			message: "has no key",
		},
		{
			name:    "page without key",
			section: types.SettingsSection{Key: "s", Pages: []types.SettingsPage{{Name: "A", Href: "/a"}}},
			message: "has no key",
		},
		{
			name:    "page without href",
			section: types.SettingsSection{Key: "s", Pages: []types.SettingsPage{{Key: "a"}}},
			message: "has no href",
		},
		{
			name:    "duplicate page key across sections",
			section: types.SettingsSection{Key: "s", Pages: []types.SettingsPage{{Key: "a", Href: "/a"}}},
			extra:   &types.SettingsSection{Key: "t", Pages: []types.SettingsPage{{Key: "a", Href: "/b"}}},
			message: `duplicate settings page "a"`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			engine := NewEngine()
			require.NoError(t, engine.Register(testComponent{
				descriptor: Descriptor{Name: "core"},
				build: func(builder *Builder) error {
					AddSettings(builder, tc.section)
					if tc.extra != nil {
						AddSettings(builder, *tc.extra)
					}
					return nil
				},
			}))
			_, err := engine.Compile(BuildContext{})
			require.Error(t, err)
			require.Contains(t, err.Error(), tc.message)
		})
	}
}

func TestEngineCompileWithoutSettingsHasEmptyRegistry(t *testing.T) {
	engine := NewEngine()
	require.NoError(t, engine.Register(testComponent{descriptor: Descriptor{Name: "core"}}))
	container, err := engine.Compile(BuildContext{})
	require.NoError(t, err)
	require.Empty(t, container.SettingsSections())
}

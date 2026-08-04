package sidebar

import (
	"testing"

	"github.com/iota-uz/go-i18n/v2/i18n"
	"github.com/iota-uz/iota-sdk/pkg/types"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/language"
)

func TestWorkspaceTabGroupBuilderLocalizesAndFlattensWorkspaceContainers(t *testing.T) {
	bundle := i18n.NewBundle(language.English)
	require.NoError(t, bundle.AddMessages(language.English,
		&i18n.Message{ID: "NavWorkspaces.ERP", Other: "ERP"},
		&i18n.Message{ID: "NavWorkspaces.CRM", Other: "CRM"},
	))
	localizer := i18n.NewLocalizer(bundle, language.English.String())

	collection := WorkspaceTabGroupBuilder(
		[]types.NavigationItem{
			{Name: "Dashboard", Href: "/"},
			{
				Name:      "CRM",
				Href:      "/crm",
				Workspace: "crm",
				Children: []types.NavigationItem{
					{Name: "Clients", Href: "/crm/clients", Workspace: "crm"},
				},
			},
		},
		[]types.NavWorkspace{
			{Key: "erp", Label: "NavWorkspaces.ERP", Default: true, Order: 0},
			{Key: "crm", Label: "NavWorkspaces.CRM", Order: 1},
		},
		localizer,
	)

	require.Equal(t, "erp", collection.DefaultValue)
	require.Len(t, collection.Groups, 2)
	require.Equal(t, "ERP", collection.Groups[0].Label)
	require.Equal(t, "Dashboard", collection.Groups[0].Items[0].(interface{ Text() string }).Text())
	require.Equal(t, "CRM", collection.Groups[1].Label)
	require.Equal(t, "Clients", collection.Groups[1].Items[0].(interface{ Text() string }).Text())
}

func TestWorkspaceTabGroupBuilderHonorsChildWorkspaceOverride(t *testing.T) {
	collection := WorkspaceTabGroupBuilder(
		[]types.NavigationItem{
			{
				Name:      "CRM",
				Href:      "/crm",
				Workspace: "crm",
				Children: []types.NavigationItem{
					{Name: "Clients", Href: "/crm/clients"},
					// Child declares a different workspace: it must land in ERP,
					// not be flattened into the parent's CRM workspace.
					{Name: "Shared Report", Href: "/reports", Workspace: "erp"},
				},
			},
		},
		[]types.NavWorkspace{
			{Key: "erp", Label: "ERP", Default: true, Order: 0},
			{Key: "crm", Label: "CRM", Order: 1},
		},
		nil,
	)

	require.Len(t, collection.Groups, 2)
	require.Equal(t, "erp", collection.Groups[0].Value)
	require.Len(t, collection.Groups[0].Items, 1)
	require.Equal(t, "Shared Report", collection.Groups[0].Items[0].(interface{ Text() string }).Text())
	require.Equal(t, "crm", collection.Groups[1].Value)
	require.Len(t, collection.Groups[1].Items, 1)
	require.Equal(t, "Clients", collection.Groups[1].Items[0].(interface{ Text() string }).Text())
}

func TestBuildTabGroupsWithWorkspacesFallsBackToLegacyBuilder(t *testing.T) {
	collection := BuildTabGroupsWithWorkspaces(
		[]types.NavigationItem{
			{
				Name: "CRM",
				Href: "/crm",
				Children: []types.NavigationItem{
					{Name: "Clients", Href: "/crm/clients"},
				},
			},
		},
		nil,
		nil,
	)

	require.Len(t, collection.Groups, 1)
	require.Equal(t, "crm", collection.Groups[0].Value)
	require.Equal(t, "Clients", collection.Groups[0].Items[0].(interface{ Text() string }).Text())
}

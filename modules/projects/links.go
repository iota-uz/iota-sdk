package projects

import (
	icons "github.com/iota-uz/icons/phosphor"
	"github.com/iota-uz/iota-sdk/pkg/types"
)

var (
	ProjectsItem = types.NavigationItem{
		Name:        "NavigationLinks.Projects",
		Href:        "/projects",
		Permissions: nil,
		Children:    nil,
	}
	ProjectStagesItem = types.NavigationItem{
		Name:        "NavigationLinks.ProjectStages",
		Href:        "/project-stages",
		Permissions: nil,
		Children:    nil,
	}
)

var NavItems = []types.NavigationItem{
	{
		Name:        "NavigationLinks.Projects",
		Href:        "#projects",
		Permissions: nil,
		Icon:        icons.FolderOpen(icons.Props{Size: "24"}),
		Children: []types.NavigationItem{
			ProjectsItem,
			ProjectStagesItem,
		},
	},
}

// Package projects provides this package.
package projects

import (
	icons "github.com/iota-uz/icons/phosphor"
	"github.com/iota-uz/iota-sdk/pkg/application"
)

var ProjectsNavNode = application.NavNode{
	ID:       "projects",
	TitleKey: "NavigationLinks.Projects",
	Icon:     icons.FolderOpen(icons.Props{Size: "24"}),
	Order:    4,
}

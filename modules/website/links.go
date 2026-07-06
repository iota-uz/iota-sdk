// Package website provides this package.
package website

import (
	icons "github.com/iota-uz/icons/phosphor"
	"github.com/iota-uz/iota-sdk/pkg/application"
)

var WebsiteNavNode = application.NavNode{
	ID:       "website",
	TitleKey: "NavigationLinks.Website",
	Icon:     icons.Globe(icons.Props{Size: "20"}),
}

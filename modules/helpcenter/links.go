package helpcenter

import (
	icons "github.com/iota-uz/icons/phosphor"
	"github.com/iota-uz/iota-sdk/pkg/application"
)

var HelpCenterNavNode = application.NavNode{
	ID:       "helpcenter",
	TitleKey: "NavigationLinks.HelpCenter",
	Path:     "/help",
	Icon:     icons.Question(icons.Props{Size: "20"}),
}

package website

import (
	icons "github.com/iota-uz/icons/phosphor"
	"github.com/iota-uz/iota-sdk/pkg/types"
)

var AIChatLink = types.NavigationItem{
	Name:     "NavigationLinks.AIChatbot",
	Icon:     icons.Robot(icons.Props{Size: "20"}),
	Href:     "/website/ai-chat",
	Children: nil,
}

var WebsiteLink = types.NavigationItem{
	Name: "NavigationLinks.Website",
	Icon: icons.Globe(icons.Props{Size: "20"}),
	Href: "/website",
	Children: []types.NavigationItem{
		AIChatLink,
	},
}

var NavItems = []types.NavigationItem{
	WebsiteLink,
}

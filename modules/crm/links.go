package crm

import (
	icons "github.com/iota-uz/icons/phosphor"

	"github.com/iota-uz/iota-sdk/pkg/types"
)

var ClientsLink = types.NavigationItem{
	Name:     "NavigationLinks.Clients",
	Icon:     icons.Users(icons.Props{Size: "20"}),
	Href:     "/crm/clients",
	Children: nil,
}

var ChatsLink = types.NavigationItem{
	Name:     "NavigationLinks.Chats",
	Icon:     icons.ChatCircle(icons.Props{Size: "20"}),
	Href:     "/crm/chats",
	Children: nil,
}

var CRMLink = types.NavigationItem{
	Name: "NavigationLinks.CRM",
	Icon: icons.Handshake(icons.Props{Size: "20"}),
	Href: "#",
	Children: []types.NavigationItem{
		ClientsLink,
		ChatsLink,
	},
}

var NavItems = []types.NavigationItem{
	CRMLink,
}

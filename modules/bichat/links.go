package bichat

import (
	icons "github.com/iota-uz/icons/phosphor"
	"github.com/iota-uz/iota-sdk/pkg/types"
)

var BiChatLink = types.NavigationItem{
	Name:     "NavigationLinks.BiChat",
	Icon:     icons.ChatCircle(icons.Props{Size: "20"}),
	Href:     "/bi-chat",
	Children: nil,
}

var NavItems = []types.NavigationItem{
	BiChatLink,
}

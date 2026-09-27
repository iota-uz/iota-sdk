// Package warehouse provides this package.
package warehouse

import (
	icons "github.com/iota-uz/icons/phosphor"
	"github.com/iota-uz/iota-sdk/pkg/application"
)

var WarehouseLink = application.NavNode{
	ID:       "warehouse",
	TitleKey: "NavigationLinks.Warehouse",
	Icon:     icons.Warehouse(icons.Props{Size: "20"}),
}

// Package finance provides this package.
package finance

import (
	icons "github.com/iota-uz/icons/phosphor"
	"github.com/iota-uz/iota-sdk/pkg/application"
)

var (
	FinanceLink = application.NavNode{
		ID:       "finance",
		TitleKey: "NavigationLinks.Finances",
		Icon:     icons.Money(icons.Props{Size: "20"}),
	}
	FinanceEnumsLink = application.NavNode{
		ID:       "finance.enums",
		Parent:   "finance",
		TitleKey: "NavigationLinks.Finance.Enums",
		Order:    70,
	}
	FinanceReportsLink = application.NavNode{
		ID:       "finance.reports",
		Parent:   "finance",
		TitleKey: "NavigationLinks.Reports",
		Order:    80,
	}
)

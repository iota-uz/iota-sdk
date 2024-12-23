package finance

import (
	"github.com/iota-agency/iota-sdk/pkg/presentation/templates/icons"
	"github.com/iota-agency/iota-sdk/pkg/types"
)

var (
	ExpenseCategoriesItem = types.NavigationItem{
		Name:        "NavigationLinks.ExpenseCategories",
		Href:        "/finance/expense-categories",
		Permissions: nil,
		Children:    nil,
	}
	PaymentsItem = types.NavigationItem{
		Name:        "NavigationLinks.Payments",
		Href:        "/finance/payments",
		Permissions: nil,
		Children:    nil,
	}
	ExpensesItem = types.NavigationItem{
		Name:        "NavigationLinks.Expenses",
		Href:        "/finance/expenses",
		Permissions: nil,
		Children:    nil,
	}
	AccountsItem = types.NavigationItem{
		Name:        "NavigationLinks.Accounts",
		Href:        "/finance/accounts",
		Permissions: nil,
		Children:    nil,
	}
)

var FinanceItem = types.NavigationItem{
	Name: "NavigationLinks.Finances",
	Href: "/finance",
	Icon: icons.Money(icons.Props{Size: "20"}),
	Children: []types.NavigationItem{
		ExpenseCategoriesItem,
		PaymentsItem,
		ExpensesItem,
		AccountsItem,
	},
}

var NavItems = []types.NavigationItem{FinanceItem}

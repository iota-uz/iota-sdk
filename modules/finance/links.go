package finance

import (
	icons "github.com/iota-uz/icons/phosphor"
	"github.com/iota-uz/iota-sdk/pkg/types"
)

var (
	ExpenseCategoriesItem = types.NavigationItem{
		Name:        "NavigationLinks.ExpenseCategories",
		Href:        "/finance/expense-categories",
		Permissions: nil,
		Children:    nil,
	}
	PaymentCategoriesItem = types.NavigationItem{
		Name:        "NavigationLinks.PaymentCategories",
		Href:        "/finance/payment-categories",
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
	CounterpartiesItem = types.NavigationItem{
		Name:        "NavigationLinks.Counterparties",
		Href:        "/finance/counterparties",
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
		PaymentCategoriesItem,
		PaymentsItem,
		ExpensesItem,
		AccountsItem,
		CounterpartiesItem,
	},
}

var NavItems = []types.NavigationItem{FinanceItem}

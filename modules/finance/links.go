package finance

import (
	icons "github.com/iota-uz/icons/phosphor"
	"github.com/iota-uz/iota-sdk/pkg/types"
)

var (
	TransactionsItem = types.NavigationItem{
		Name:        "NavigationLinks.Transactions",
		Href:        "/finance/transactions",
		Permissions: nil,
		Children:    nil,
	}
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
	InventoryItem = types.NavigationItem{
		Name:        "NavigationLinks.Inventory",
		Href:        "/finance/inventory",
		Permissions: nil,
		Children:    nil,
	}
	ReportsItem = types.NavigationItem{
		Name:        "NavigationLinks.Reports",
		Href:        "/finance/reports",
		Permissions: nil,
		Children: []types.NavigationItem{
			{
				Name:        "NavigationLinks.IncomeStatement",
				Href:        "/finance/reports/income-statement",
				Permissions: nil,
				Children:    nil,
			},
			{
				Name:        "NavigationLinks.CashflowStatement",
				Href:        "/finance/reports/cashflow",
				Permissions: nil,
				Children:    nil,
			},
		},
	}
)

var FinanceItem = types.NavigationItem{
	Name: "NavigationLinks.Finances",
	Href: "/finance",
	Icon: icons.Money(icons.Props{Size: "20"}),
	Children: []types.NavigationItem{
		TransactionsItem,
		ExpenseCategoriesItem,
		PaymentCategoriesItem,
		PaymentsItem,
		ExpensesItem,
		AccountsItem,
		CounterpartiesItem,
		InventoryItem,
		ReportsItem,
	},
}

var NavItems = []types.NavigationItem{FinanceItem}

package finance

import (
	icons "github.com/iota-uz/icons/phosphor"
	"github.com/iota-uz/iota-sdk/pkg/types"
)

var (
	TransactionsItem = types.NavigationItem{
		Name:        "NavigationLinks.Transactions",
		Href:        "/finance/overview?tab=transactions",
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
		Href:        "/finance/overview?tab=payments",
		Permissions: nil,
		Children:    nil,
	}
	ExpensesItem = types.NavigationItem{
		Name:        "NavigationLinks.Expenses",
		Href:        "/finance/overview?tab=expenses",
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
	DebtsItem = types.NavigationItem{
		Name:        "NavigationLinks.Debts",
		Href:        "/finance/debts",
		Permissions: nil,
		Children:    nil,
	}
	DebtAggregatesItem = types.NavigationItem{
		Name:        "NavigationLinks.DebtAggregates",
		Href:        "/finance/debt-aggregates",
		Permissions: nil,
		Children:    nil,
	}
	EnumsItem = types.NavigationItem{
		Name:        "NavigationLinks.Finance.Enums",
		Href:        "/finance/enums",
		Permissions: nil,
		Children: []types.NavigationItem{
			ExpenseCategoriesItem,
			PaymentCategoriesItem,
		},
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
		{
			Name:        "NavigationLinks.FinancialOverview",
			Href:        "/finance/overview",
			Permissions: nil,
			Children:    nil,
		},
		DebtsItem,
		DebtAggregatesItem,
		AccountsItem,
		CounterpartiesItem,
		InventoryItem,
		EnumsItem,
		ReportsItem,
	},
}

var NavItems = []types.NavigationItem{FinanceItem}

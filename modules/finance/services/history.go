package services

import (
	"context"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/debt"
	"github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/expense"
	expensecategory "github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/expense_category"
	moneyaccount "github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/money_account"
	"github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/payment"
	paymentcategory "github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/payment_category"
	"github.com/iota-uz/iota-sdk/modules/finance/domain/entities/counterparty"
	"github.com/iota-uz/iota-sdk/modules/finance/domain/entities/inventory"
	"github.com/iota-uz/iota-sdk/modules/finance/domain/entities/transaction"
	"github.com/iota-uz/iota-sdk/modules/logging/domain/aggregates/businessevent"
	"github.com/iota-uz/iota-sdk/modules/logging/history"
)

const (
	PaymentResource = "finance.payment"
	ExpenseResource = "finance.expense"
	UploadResource  = "core.upload"

	PaymentCreated      = "finance.payment.created"
	PaymentUpdated      = "finance.payment.updated"
	PaymentDeleted      = "finance.payment.deleted"
	PaymentFileAttached = "finance.payment.file_attached"
	PaymentFileDetached = "finance.payment.file_detached"
	ExpenseCreated      = "finance.expense.created"
	ExpenseUpdated      = "finance.expense.updated"
	ExpenseDeleted      = "finance.expense.deleted"
	ExpenseFileAttached = "finance.expense.file_attached"
	ExpenseFileDetached = "finance.expense.file_detached"
)

const (
	TransactionResource     = "finance.transaction"
	MoneyAccountResource    = "finance.money_account"
	CounterpartyResource    = "finance.counterparty"
	DebtResource            = "finance.debt"
	InventoryResource       = "finance.inventory"
	ExpenseCategoryResource = "finance.expense_category"
	PaymentCategoryResource = "finance.payment_category"

	TransactionCreated     = "finance.transaction.created"
	TransactionUpdated     = "finance.transaction.updated"
	TransactionDeleted     = "finance.transaction.deleted"
	MoneyAccountCreated    = "finance.money_account.created"
	MoneyAccountUpdated    = "finance.money_account.updated"
	MoneyAccountDeleted    = "finance.money_account.deleted"
	CounterpartyCreated    = "finance.counterparty.created"
	CounterpartyUpdated    = "finance.counterparty.updated"
	CounterpartyDeleted    = "finance.counterparty.deleted"
	DebtCreated            = "finance.debt.created"
	DebtUpdated            = "finance.debt.updated"
	DebtDeleted            = "finance.debt.deleted"
	DebtSettled            = "finance.debt.settled"
	DebtWrittenOff         = "finance.debt.written_off"
	DebtCancelled          = "finance.debt.cancelled"
	InventoryCreated       = "finance.inventory.created"
	InventoryUpdated       = "finance.inventory.updated"
	InventoryDeleted       = "finance.inventory.deleted"
	ExpenseCategoryCreated = "finance.expense_category.created"
	ExpenseCategoryUpdated = "finance.expense_category.updated"
	ExpenseCategoryDeleted = "finance.expense_category.deleted"
	PaymentCategoryCreated = "finance.payment_category.created"
	PaymentCategoryUpdated = "finance.payment_category.updated"
	PaymentCategoryDeleted = "finance.payment_category.deleted"
)

// HistoryDefinitions are the business history event types of the finance module.
func HistoryDefinitions() []history.Definition {
	defs := make([]history.Definition, 0, 40)
	defs = append(defs, history.Lifecycle("finance.payment", "Payments.History", history.Resource{Name: PaymentResource, LabelKey: "NavigationLinks.Payments", Fields: []history.Field{
		{Name: "amount", LabelKey: "Payments.Single.Amount", Type: history.ValueMoney},
		{Name: "account_id", LabelKey: "Payments.Single.AccountID", Type: history.ValueString},
		{Name: "category_id", LabelKey: "Payments.Single.CategoryID", Type: history.ValueString},
		{Name: "counterparty_id", LabelKey: "Payments.Single.CounterpartyID.Label", Type: history.ValueString},
		{Name: "transaction_date", LabelKey: "Payments.Single.Date", Type: history.ValueDate},
		{Name: "accounting_period", LabelKey: "Payments.Single.AccountingPeriod", Type: history.ValueDate},
		{Name: "comment", LabelKey: "Payments.Single.Comment", Type: history.ValueString},
	}})...)
	defs = append(defs, history.Lifecycle("finance.expense", "Expenses.History", history.Resource{Name: ExpenseResource, Fields: []history.Field{
		{Name: "amount", LabelKey: "Expenses.Single.Amount", Type: history.ValueMoney},
		{Name: "account_id", LabelKey: "Expenses.Single.AccountID", Type: history.ValueString},
		{Name: "category_id", LabelKey: "Expenses.Single.CategoryID", Type: history.ValueString},
		{Name: "date", LabelKey: "Expenses.Single.Date", Type: history.ValueDate},
		{Name: "accounting_period", LabelKey: "Expenses.Single.AccountingPeriod", Type: history.ValueDate},
		{Name: "comment", LabelKey: "Expenses.Single.Comment", Type: history.ValueString},
	}})...)
	defs = append(defs,
		fileDefinition(PaymentFileAttached, "Payments.History.FileAttached", PaymentResource),
		fileDefinition(PaymentFileDetached, "Payments.History.FileDetached", PaymentResource),
		fileDefinition(ExpenseFileAttached, "Expenses.History.FileAttached", ExpenseResource),
		fileDefinition(ExpenseFileDetached, "Expenses.History.FileDetached", ExpenseResource),
	)
	defs = append(defs, history.Lifecycle("finance.transaction", "Transactions.History", history.Resource{Name: TransactionResource, Fields: []history.Field{
		{Name: "type", LabelKey: "Transactions.Single.Type", Type: history.ValueString},
		{Name: "amount", LabelKey: "Transactions.Single.Amount", Type: history.ValueMoney},
		{Name: "origin_account_id", LabelKey: "Transactions.Single.OriginAccount", Type: history.ValueString},
		{Name: "destination_account_id", LabelKey: "Transactions.Single.DestinationAccount", Type: history.ValueString},
		{Name: "exchange_rate", LabelKey: "Transactions.Single.ExchangeRate", Type: history.ValueDecimal},
		{Name: "destination_amount", LabelKey: "Transactions.Single.DestinationAmount", Type: history.ValueMoney},
		{Name: "transaction_date", LabelKey: "Transactions.Single.Date", Type: history.ValueDate},
		{Name: "accounting_period", LabelKey: "Transactions.Single.AccountingPeriod", Type: history.ValueDate},
		{Name: "comment", LabelKey: "Transactions.Single.Comment", Type: history.ValueString},
	}})...)
	defs = append(defs, history.Lifecycle("finance.money_account", "MoneyAccounts.History", history.Resource{Name: MoneyAccountResource, Fields: []history.Field{
		{Name: "name", LabelKey: "MoneyAccounts.Single.Name", Type: history.ValueString},
		{Name: "account_number", LabelKey: "MoneyAccounts.Single.AccountNumber", Type: history.ValueString},
		{Name: "balance", LabelKey: "MoneyAccounts.Single.Balance", Type: history.ValueMoney},
		{Name: "description", LabelKey: "MoneyAccounts.Single._Description", Type: history.ValueString},
	}})...)
	defs = append(defs, history.Lifecycle("finance.counterparty", "Counterparties.History", history.Resource{Name: CounterpartyResource, Fields: []history.Field{
		{Name: "name", LabelKey: "Counterparties.Single.Name", Type: history.ValueString},
		{Name: "tin", LabelKey: "Counterparties.Single.TIN", Type: history.ValueString},
		{Name: "type", LabelKey: "Counterparties.Single.Type", Type: history.ValueString},
		{Name: "legal_type", LabelKey: "Counterparties.Single.LegalType", Type: history.ValueString},
		{Name: "legal_address", LabelKey: "Counterparties.Single.LegalAddress", Type: history.ValueString},
	}})...)
	debtResource := history.Resource{Name: DebtResource, Fields: []history.Field{
		{Name: "type", LabelKey: "Debts.Single.Type", Type: history.ValueString},
		{Name: "status", LabelKey: "Debts.Single.Status", Type: history.ValueString},
		{Name: "counterparty_id", LabelKey: "Debts.Single.CounterpartyID", Type: history.ValueString},
		{Name: "original_amount", LabelKey: "Debts.Single.OriginalAmount", Type: history.ValueMoney},
		{Name: "outstanding_amount", LabelKey: "Debts.Single.OutstandingAmount", Type: history.ValueMoney},
		{Name: "due_date", LabelKey: "Debts.Single.DueDate", Type: history.ValueDate},
		{Name: "money_account_id", LabelKey: "Debts.Single.Account", Type: history.ValueString},
		{Name: "project_id", LabelKey: "Debts.Single.Project", Type: history.ValueString},
		{Name: "settlement_transaction_id", LabelKey: "Debts.Single.SettlementTransactionID", Type: history.ValueString},
		{Name: "description", LabelKey: "Debts.Single._Description", Type: history.ValueString},
	}}
	defs = append(defs, history.Lifecycle("finance.debt", "Debts.History", debtResource)...)
	defs = append(defs,
		actionDefinition(DebtSettled, "Debts.History.Settled", debtResource),
		actionDefinition(DebtWrittenOff, "Debts.History.WrittenOff", debtResource),
		actionDefinition(DebtCancelled, "Debts.History.Cancelled", debtResource),
	)
	defs = append(defs, history.Lifecycle("finance.inventory", "Inventory.History", history.Resource{Name: InventoryResource, Fields: []history.Field{
		{Name: "name", LabelKey: "Inventory.Single.Name", Type: history.ValueString},
		{Name: "price", LabelKey: "Inventory.Single.Price", Type: history.ValueMoney},
		{Name: "quantity", LabelKey: "Inventory.Single.Quantity", Type: history.ValueInteger},
		{Name: "description", LabelKey: "Inventory.Single._Description", Type: history.ValueString},
	}})...)
	defs = append(defs, history.Lifecycle("finance.expense_category", "ExpenseCategories.History", history.Resource{Name: ExpenseCategoryResource, Fields: []history.Field{
		{Name: "name", LabelKey: "ExpenseCategories.Single.Name", Type: history.ValueString},
		{Name: "is_cogs", LabelKey: "ExpenseCategories.Single.IsCOGS", Type: history.ValueBoolean},
		{Name: "description", LabelKey: "ExpenseCategories.Single._Description", Type: history.ValueString},
	}})...)
	defs = append(defs, history.Lifecycle("finance.payment_category", "PaymentCategories.History", history.Resource{Name: PaymentCategoryResource, Fields: []history.Field{
		{Name: "name", LabelKey: "PaymentCategories.Single.Name", Type: history.ValueString},
		{Name: "description", LabelKey: "PaymentCategories.Single._Description", Type: history.ValueString},
	}})...)
	return defs
}

func actionDefinition(eventType, keyPrefix string, resource history.Resource) history.Definition {
	return history.Definition{
		Type:           eventType,
		Version:        1,
		Category:       businessevent.CategoryBusiness,
		TitleKey:       keyPrefix + ".Title",
		DescriptionKey: keyPrefix + ".Summary",
		Resources:      []history.Resource{resource},
	}
}

func fileDefinition(eventType, keyPrefix, resource string) history.Definition {
	return history.Definition{
		Type:           eventType,
		Version:        1,
		Category:       businessevent.CategoryBusiness,
		TitleKey:       keyPrefix + ".Title",
		DescriptionKey: keyPrefix + ".Summary",
		Resources:      []history.Resource{{Name: resource}, {Name: UploadResource}},
	}
}

func paymentSnapshot(p payment.Payment) history.Snapshot {
	s := history.Snapshot{
		"amount":            history.Money(p.Amount()),
		"counterparty_id":   history.UUID(p.CounterpartyID()),
		"transaction_date":  history.Date(p.TransactionDate()),
		"accounting_period": history.Date(p.AccountingPeriod()),
		"comment":           p.Comment(),
	}
	if p.Account() != nil {
		s["account_id"] = p.Account().ID().String()
	}
	if p.Category() != nil {
		s["category_id"] = p.Category().ID().String()
	}
	return s
}

func expenseSnapshot(e expense.Expense) history.Snapshot {
	s := history.Snapshot{
		"amount":            history.Money(e.Amount()),
		"date":              history.Date(e.Date()),
		"accounting_period": history.Date(e.AccountingPeriod()),
		"comment":           e.Comment(),
	}
	if e.Account() != nil {
		s["account_id"] = e.Account().ID().String()
	}
	if e.Category() != nil {
		s["category_id"] = e.Category().ID().String()
	}
	return s
}

func recordFileEvent(ctx context.Context, w history.Writer, eventType, resource, id string, uploadID uint) error {
	_, err := w.Record(ctx, history.Input{
		Type:    eventType,
		Version: 1,
		Outcome: businessevent.OutcomeSucceeded,
		Subjects: []businessevent.Subject{
			{Resource: resource, ID: id},
			{Resource: UploadResource, ID: strconv.FormatUint(uint64(uploadID), 10)},
		},
	})
	return err
}

func transactionSnapshot(t transaction.Transaction) history.Snapshot {
	s := history.Snapshot{
		"type":                   string(t.TransactionType()),
		"amount":                 history.Money(t.Amount()),
		"origin_account_id":      history.UUID(t.OriginAccountID()),
		"destination_account_id": history.UUID(t.DestinationAccountID()),
		"destination_amount":     history.Money(t.DestinationAmount()),
		"transaction_date":       history.Date(t.TransactionDate()),
		"accounting_period":      history.Date(t.AccountingPeriod()),
		"comment":                t.Comment(),
	}
	if rate := t.ExchangeRate(); rate != nil {
		s["exchange_rate"] = strconv.FormatFloat(*rate, 'f', -1, 64)
	}
	return s
}

func moneyAccountSnapshot(a moneyaccount.Account) history.Snapshot {
	return history.Snapshot{
		"name":           a.Name(),
		"account_number": a.AccountNumber(),
		"balance":        history.Money(a.Balance()),
		"description":    a.Description(),
	}
}

func counterpartySnapshot(c counterparty.Counterparty) history.Snapshot {
	s := history.Snapshot{
		"name":          c.Name(),
		"type":          string(c.Type()),
		"legal_type":    string(c.LegalType()),
		"legal_address": c.LegalAddress(),
	}
	if c.Tin() != nil {
		s["tin"] = c.Tin().Value()
	}
	return s
}

func debtSnapshot(d debt.Debt) history.Snapshot {
	return history.Snapshot{
		"type":                      string(d.Type()),
		"status":                    string(d.Status()),
		"counterparty_id":           history.UUID(d.CounterpartyID()),
		"original_amount":           history.Money(d.OriginalAmount()),
		"outstanding_amount":        history.Money(d.OutstandingAmount()),
		"due_date":                  optionalDate(d.DueDate()),
		"money_account_id":          optionalUUID(d.MoneyAccountID()),
		"project_id":                optionalUUID(d.ProjectID()),
		"settlement_transaction_id": optionalUUID(d.SettlementTransactionID()),
		"description":               d.Description(),
	}
}

func inventorySnapshot(i inventory.Inventory) history.Snapshot {
	return history.Snapshot{
		"name":        i.Name(),
		"price":       history.Money(i.Price()),
		"quantity":    i.Quantity(),
		"description": i.Description(),
	}
}

func expenseCategorySnapshot(c expensecategory.ExpenseCategory) history.Snapshot {
	return history.Snapshot{
		"name":        c.Name(),
		"is_cogs":     c.IsCOGS(),
		"description": c.Description(),
	}
}

func paymentCategorySnapshot(c paymentcategory.PaymentCategory) history.Snapshot {
	return history.Snapshot{
		"name":        c.Name(),
		"description": c.Description(),
	}
}

func optionalUUID(id *uuid.UUID) any {
	if id == nil {
		return nil
	}
	return history.UUID(*id)
}

func optionalDate(t *time.Time) any {
	if t == nil {
		return nil
	}
	return history.Date(*t)
}

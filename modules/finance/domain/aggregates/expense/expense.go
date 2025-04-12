package expense

import (
	"time"

	category "github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/expense_category"
	moneyaccount "github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/money_account"
)

type Option func(e *expense)

// Option setters
func WithID(id uint) Option {
	return func(e *expense) {
		e.id = id
	}
}

func WithAmount(amount float64) Option {
	return func(e *expense) {
		e.amount = amount
	}
}

func WithAccount(account moneyaccount.Account) Option {
	return func(e *expense) {
		e.account = account
	}
}

func WithCategory(category category.ExpenseCategory) Option {
	return func(e *expense) {
		e.category = category
	}
}

func WithComment(comment string) Option {
	return func(e *expense) {
		e.comment = comment
	}
}

func WithTransactionID(transactionID uint) Option {
	return func(e *expense) {
		e.transactionID = transactionID
	}
}

func WithAccountingPeriod(accountingPeriod time.Time) Option {
	return func(e *expense) {
		e.accountingPeriod = accountingPeriod
	}
}

func WithDate(date time.Time) Option {
	return func(e *expense) {
		e.date = date
	}
}

func WithCreatedAt(createdAt time.Time) Option {
	return func(e *expense) {
		e.createdAt = createdAt
	}
}

func WithUpdatedAt(updatedAt time.Time) Option {
	return func(e *expense) {
		e.updatedAt = updatedAt
	}
}

// Interface
type Expense interface {
	ID() uint
	Amount() float64
	Account() moneyaccount.Account
	Category() category.ExpenseCategory
	Comment() string
	TransactionID() uint
	AccountingPeriod() time.Time
	Date() time.Time
	CreatedAt() time.Time
	UpdatedAt() time.Time
}

// Implementation
func New(
	amount float64,
	account moneyaccount.Account,
	category category.ExpenseCategory,
	date time.Time,
	opts ...Option,
) Expense {
	e := &expense{
		id:               0,
		amount:           amount,
		account:          account,
		category:         category,
		comment:          "",
		transactionID:    0,
		accountingPeriod: time.Time{},
		date:             date,
		createdAt:        time.Now(),
		updatedAt:        time.Now(),
	}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

type expense struct {
	id               uint
	amount           float64
	account          moneyaccount.Account
	category         category.ExpenseCategory
	comment          string
	transactionID    uint
	accountingPeriod time.Time
	date             time.Time
	createdAt        time.Time
	updatedAt        time.Time
}

func (e *expense) ID() uint {
	return e.id
}

func (e *expense) Amount() float64 {
	return e.amount
}

func (e *expense) Account() moneyaccount.Account {
	return e.account
}

func (e *expense) Category() category.ExpenseCategory {
	return e.category
}

func (e *expense) Comment() string {
	return e.comment
}

func (e *expense) TransactionID() uint {
	return e.transactionID
}

func (e *expense) AccountingPeriod() time.Time {
	return e.accountingPeriod
}

func (e *expense) Date() time.Time {
	return e.date
}

func (e *expense) CreatedAt() time.Time {
	return e.createdAt
}

func (e *expense) UpdatedAt() time.Time {
	return e.updatedAt
}

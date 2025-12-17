package expense

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	category "github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/expense_category"
	moneyaccount "github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/money_account"
	"github.com/iota-uz/iota-sdk/pkg/money"
)

type Option func(e *expense)

// Option setters
func WithID(id uuid.UUID) Option {
	return func(e *expense) {
		e.id = id
	}
}

func WithAmount(amount *money.Money) Option {
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

func WithTransactionID(transactionID uuid.UUID) Option {
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

func WithTenantID(tenantID uuid.UUID) Option {
	return func(e *expense) {
		e.tenantID = tenantID
	}
}

func WithAttachments(attachments []uint) Option {
	return func(e *expense) {
		e.attachments = attachments
	}
}

// Interface
type Expense interface {
	ID() uuid.UUID
	Amount() *money.Money
	Account() moneyaccount.Account
	Category() category.ExpenseCategory
	Comment() string
	TransactionID() uuid.UUID
	AccountingPeriod() time.Time
	Date() time.Time
	CreatedAt() time.Time
	UpdatedAt() time.Time
	TenantID() uuid.UUID

	SetAccount(account moneyaccount.Account) Expense
	SetCategory(category category.ExpenseCategory) Expense
	SetComment(comment string) Expense
	SetAmount(amount *money.Money) Expense
	SetDate(date time.Time) Expense
	SetAccountingPeriod(period time.Time) Expense

	// Attachment methods
	GetAttachments() []uint
	HasAttachment(uploadID uint) bool
	AttachFile(uploadID uint) (Expense, error)
	DetachFile(uploadID uint) (Expense, error)
}

// Implementation
func New(
	amount *money.Money,
	account moneyaccount.Account,
	category category.ExpenseCategory,
	date time.Time,
	opts ...Option,
) Expense {
	e := &expense{
		id:               uuid.New(),
		amount:           amount,
		account:          account,
		category:         category,
		comment:          "",
		transactionID:    uuid.Nil,
		accountingPeriod: time.Time{},
		date:             date,
		createdAt:        time.Now(),
		updatedAt:        time.Now(),
		tenantID:         uuid.Nil,
		attachments:      []uint{},
	}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

type expense struct {
	id               uuid.UUID
	amount           *money.Money
	account          moneyaccount.Account
	category         category.ExpenseCategory
	comment          string
	transactionID    uuid.UUID
	accountingPeriod time.Time
	date             time.Time
	createdAt        time.Time
	updatedAt        time.Time
	tenantID         uuid.UUID
	attachments      []uint
}

func (e *expense) ID() uuid.UUID {
	return e.id
}

func (e *expense) Amount() *money.Money {
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

func (e *expense) TransactionID() uuid.UUID {
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

func (e *expense) SetAccount(account moneyaccount.Account) Expense {
	result := *e
	result.account = account
	result.updatedAt = time.Now()
	return &result
}

func (e *expense) SetCategory(category category.ExpenseCategory) Expense {
	result := *e
	result.category = category
	result.updatedAt = time.Now()
	return &result
}

func (e *expense) SetComment(comment string) Expense {
	result := *e
	result.comment = comment
	result.updatedAt = time.Now()
	return &result
}

func (e *expense) SetAmount(amount *money.Money) Expense {
	result := *e
	result.amount = amount
	result.updatedAt = time.Now()
	return &result
}

func (e *expense) SetDate(date time.Time) Expense {
	result := *e
	result.date = date
	result.updatedAt = time.Now()
	return &result
}

func (e *expense) SetAccountingPeriod(period time.Time) Expense {
	result := *e
	result.accountingPeriod = period
	result.updatedAt = time.Now()
	return &result
}

func (e *expense) TenantID() uuid.UUID {
	return e.tenantID
}

// Attachment methods
func (e *expense) GetAttachments() []uint {
	// Return a copy to prevent external modification
	attachments := make([]uint, len(e.attachments))
	copy(attachments, e.attachments)
	return attachments
}

func (e *expense) HasAttachment(uploadID uint) bool {
	for _, id := range e.attachments {
		if id == uploadID {
			return true
		}
	}
	return false
}

func (e *expense) AttachFile(uploadID uint) (Expense, error) {
	if uploadID == 0 {
		return nil, fmt.Errorf("upload ID cannot be zero")
	}

	if e.HasAttachment(uploadID) {
		return nil, fmt.Errorf("file with ID %d is already attached", uploadID)
	}

	result := *e
	result.attachments = make([]uint, len(e.attachments)+1)
	copy(result.attachments, e.attachments)
	result.attachments[len(e.attachments)] = uploadID
	result.updatedAt = time.Now()
	return &result, nil
}

func (e *expense) DetachFile(uploadID uint) (Expense, error) {
	if uploadID == 0 {
		return nil, fmt.Errorf("upload ID cannot be zero")
	}

	if !e.HasAttachment(uploadID) {
		return nil, fmt.Errorf("file with ID %d is not attached", uploadID)
	}

	result := *e
	result.attachments = make([]uint, 0, len(e.attachments)-1)
	for _, id := range e.attachments {
		if id != uploadID {
			result.attachments = append(result.attachments, id)
		}
	}
	result.updatedAt = time.Now()
	return &result, nil
}

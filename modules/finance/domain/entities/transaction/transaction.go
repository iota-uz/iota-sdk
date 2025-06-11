package transaction

import (
	"time"

	"github.com/google/uuid"
)

type Option func(t *transaction)

// Option setters
func WithID(id uuid.UUID) Option {
	return func(t *transaction) {
		t.id = id
	}
}

func WithTenantID(tenantID uuid.UUID) Option {
	return func(t *transaction) {
		t.tenantID = tenantID
	}
}

func WithOriginAccountID(accountID uuid.UUID) Option {
	return func(t *transaction) {
		t.originAccountID = accountID
	}
}

func WithDestinationAccountID(accountID uuid.UUID) Option {
	return func(t *transaction) {
		t.destinationAccountID = accountID
	}
}

func WithTransactionDate(date time.Time) Option {
	return func(t *transaction) {
		t.transactionDate = date
	}
}

func WithAccountingPeriod(period time.Time) Option {
	return func(t *transaction) {
		t.accountingPeriod = period
	}
}

func WithComment(comment string) Option {
	return func(t *transaction) {
		t.comment = comment
	}
}

func WithCreatedAt(createdAt time.Time) Option {
	return func(t *transaction) {
		t.createdAt = createdAt
	}
}

func WithExchangeRate(rate *float64) Option {
	return func(t *transaction) {
		t.exchangeRate = rate
	}
}

func WithDestinationAmount(amount *float64) Option {
	return func(t *transaction) {
		t.destinationAmount = amount
	}
}

func New(
	amount float64,
	transactionType Type,
	opts ...Option,
) Transaction {
	t := &transaction{
		id:                   uuid.Nil,
		tenantID:             uuid.Nil,
		amount:               amount,
		originAccountID:      uuid.Nil,
		destinationAccountID: uuid.Nil,
		transactionDate:      time.Now(),
		accountingPeriod:     time.Now(),
		transactionType:      transactionType,
		comment:              "",
		createdAt:            time.Now(),
		exchangeRate:         nil,
		destinationAmount:    nil,
	}
	for _, opt := range opts {
		opt(t)
	}
	return t
}

func NewDeposit(
	amount float64,
	originAccount,
	destinationAccount uuid.UUID,
	date time.Time,
	accountingPeriod time.Time,
	comment string,
) Transaction {
	return New(
		amount,
		Deposit,
		WithOriginAccountID(originAccount),
		WithDestinationAccountID(destinationAccount),
		WithTransactionDate(date),
		WithAccountingPeriod(accountingPeriod),
		WithComment(comment),
	)
}

func NewWithdrawal(
	amount float64,
	originAccount,
	destinationAccount uuid.UUID,
	date time.Time,
	accountingPeriod time.Time,
	comment string,
) Transaction {
	return New(
		amount,
		Withdrawal,
		WithOriginAccountID(originAccount),
		WithDestinationAccountID(destinationAccount),
		WithTransactionDate(date),
		WithAccountingPeriod(accountingPeriod),
		WithComment(comment),
	)
}

func NewExchange(
	amount float64,
	originAccount,
	destinationAccount uuid.UUID,
	date time.Time,
	accountingPeriod time.Time,
	comment string,
	exchangeRate float64,
	destinationAmount float64,
) Transaction {
	return New(
		amount,
		Exchange,
		WithOriginAccountID(originAccount),
		WithDestinationAccountID(destinationAccount),
		WithTransactionDate(date),
		WithAccountingPeriod(accountingPeriod),
		WithComment(comment),
		WithExchangeRate(&exchangeRate),
		WithDestinationAmount(&destinationAmount),
	)
}

type transaction struct {
	id                   uuid.UUID
	tenantID             uuid.UUID
	amount               float64
	originAccountID      uuid.UUID
	destinationAccountID uuid.UUID
	transactionDate      time.Time
	accountingPeriod     time.Time
	transactionType      Type
	comment              string
	createdAt            time.Time

	// Exchange operation fields
	exchangeRate      *float64 // Exchange rate used for currency conversion
	destinationAmount *float64 // Amount in destination currency (for exchange operations)
}

func (t *transaction) ID() uuid.UUID {
	return t.id
}

func (t *transaction) SetID(id uuid.UUID) {
	t.id = id
}

func (t *transaction) TenantID() uuid.UUID {
	return t.tenantID
}

func (t *transaction) UpdateTenantID(id uuid.UUID) Transaction {
	result := *t
	result.tenantID = id
	return &result
}

func (t *transaction) Amount() float64 {
	return t.amount
}

func (t *transaction) UpdateAmount(amount float64) Transaction {
	result := *t
	result.amount = amount
	return &result
}

func (t *transaction) OriginAccountID() uuid.UUID {
	return t.originAccountID
}

func (t *transaction) UpdateOriginAccountID(accountID uuid.UUID) Transaction {
	result := *t
	result.originAccountID = accountID
	return &result
}

func (t *transaction) DestinationAccountID() uuid.UUID {
	return t.destinationAccountID
}

func (t *transaction) UpdateDestinationAccountID(accountID uuid.UUID) Transaction {
	result := *t
	result.destinationAccountID = accountID
	return &result
}

func (t *transaction) TransactionDate() time.Time {
	return t.transactionDate
}

func (t *transaction) UpdateTransactionDate(date time.Time) Transaction {
	result := *t
	result.transactionDate = date
	return &result
}

func (t *transaction) AccountingPeriod() time.Time {
	return t.accountingPeriod
}

func (t *transaction) UpdateAccountingPeriod(period time.Time) Transaction {
	result := *t
	result.accountingPeriod = period
	return &result
}

func (t *transaction) TransactionType() Type {
	return t.transactionType
}

func (t *transaction) UpdateTransactionType(transactionType Type) Transaction {
	result := *t
	result.transactionType = transactionType
	return &result
}

func (t *transaction) Comment() string {
	return t.comment
}

func (t *transaction) UpdateComment(comment string) Transaction {
	result := *t
	result.comment = comment
	return &result
}

func (t *transaction) CreatedAt() time.Time {
	return t.createdAt
}

func (t *transaction) ExchangeRate() *float64 {
	return t.exchangeRate
}

func (t *transaction) UpdateExchangeRate(rate *float64) Transaction {
	result := *t
	result.exchangeRate = rate
	return &result
}

func (t *transaction) DestinationAmount() *float64 {
	return t.destinationAmount
}

func (t *transaction) UpdateDestinationAmount(amount *float64) Transaction {
	result := *t
	result.destinationAmount = amount
	return &result
}

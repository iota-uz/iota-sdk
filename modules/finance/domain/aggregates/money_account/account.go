package moneyaccount

import (
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/currency"
	"github.com/iota-uz/iota-sdk/modules/finance/domain/entities/transaction"

	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	"github.com/iota-uz/iota-sdk/pkg/constants"
)

type Option func(a *account)

// Option setters
func WithID(id uint) Option {
	return func(a *account) {
		a.id = id
	}
}

func WithTenantID(tenantID uuid.UUID) Option {
	return func(a *account) {
		a.tenantID = tenantID
	}
}

func WithName(name string) Option {
	return func(a *account) {
		a.name = name
	}
}

func WithAccountNumber(accountNumber string) Option {
	return func(a *account) {
		a.accountNumber = accountNumber
	}
}

func WithDescription(description string) Option {
	return func(a *account) {
		a.description = description
	}
}

func WithBalance(balance float64) Option {
	return func(a *account) {
		a.balance = balance
	}
}

func WithCurrency(currency currency.Currency) Option {
	return func(a *account) {
		a.currency = currency
	}
}

func WithCreatedAt(createdAt time.Time) Option {
	return func(a *account) {
		a.createdAt = createdAt
	}
}

func WithUpdatedAt(updatedAt time.Time) Option {
	return func(a *account) {
		a.updatedAt = updatedAt
	}
}

func New(
	name string,
	currency currency.Currency,
	opts ...Option,
) Account {
	a := &account{
		id:            0,
		tenantID:      uuid.Nil,
		name:          name,
		accountNumber: "",
		description:   "",
		balance:       0.0,
		currency:      currency,
		createdAt:     time.Now(),
		updatedAt:     time.Now(),
	}
	for _, opt := range opts {
		opt(a)
	}
	return a
}

type account struct {
	id            uint
	tenantID      uuid.UUID
	name          string
	accountNumber string
	description   string
	balance       float64
	currency      currency.Currency
	createdAt     time.Time
	updatedAt     time.Time
}

func (a *account) ID() uint {
	return a.id
}

func (a *account) SetID(id uint) {
	a.id = id
}

func (a *account) TenantID() uuid.UUID {
	return a.tenantID
}

func (a *account) UpdateTenantID(id uuid.UUID) Account {
	result := *a
	result.tenantID = id
	return &result
}

func (a *account) Name() string {
	return a.name
}

func (a *account) UpdateName(name string) Account {
	result := *a
	result.name = name
	return &result
}

func (a *account) AccountNumber() string {
	return a.accountNumber
}

func (a *account) UpdateAccountNumber(accountNumber string) Account {
	result := *a
	result.accountNumber = accountNumber
	return &result
}

func (a *account) Description() string {
	return a.description
}

func (a *account) UpdateDescription(description string) Account {
	result := *a
	result.description = description
	return &result
}

func (a *account) Balance() float64 {
	return a.balance
}

func (a *account) UpdateBalance(balance float64) Account {
	result := *a
	result.balance = balance
	return &result
}

func (a *account) Currency() currency.Currency {
	return a.currency
}

func (a *account) UpdateCurrency(currency currency.Currency) Account {
	result := *a
	result.currency = currency
	return &result
}

func (a *account) CreatedAt() time.Time {
	return a.createdAt
}

func (a *account) UpdatedAt() time.Time {
	return a.updatedAt
}

func (a *account) InitialTransaction() transaction.Transaction {
	return transaction.NewDeposit(
		a.balance,
		0,
		a.id,
		a.createdAt,
		a.createdAt,
		"",
	)
}

func (a *account) Ok(l ut.Translator) (map[string]string, bool) {
	errors := map[string]string{}
	errs := constants.Validate.Struct(a)
	if errs == nil {
		return errors, true
	}
	for _, err := range errs.(validator.ValidationErrors) {
		errors[err.Field()] = err.Translate(l)
	}
	return errors, len(errors) == 0
}

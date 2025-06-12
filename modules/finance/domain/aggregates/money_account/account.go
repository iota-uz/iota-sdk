package moneyaccount

import (
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/finance/domain/entities/transaction"
	"github.com/iota-uz/iota-sdk/pkg/money"

	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	"github.com/iota-uz/iota-sdk/pkg/constants"
)

type Option func(a *account)

// Option setters
func WithID(id uuid.UUID) Option {
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

func WithBalance(balance *money.Money) Option {
	return func(a *account) {
		a.balance = balance
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
	balance *money.Money,
	opts ...Option,
) Account {
	a := &account{
		id:            uuid.New(),
		tenantID:      uuid.Nil,
		name:          name,
		accountNumber: "",
		description:   "",
		balance:       balance,
		createdAt:     time.Now(),
		updatedAt:     time.Now(),
	}
	for _, opt := range opts {
		opt(a)
	}
	return a
}

type account struct {
	id            uuid.UUID
	tenantID      uuid.UUID
	name          string
	accountNumber string
	description   string
	balance       *money.Money
	createdAt     time.Time
	updatedAt     time.Time
}

func (a *account) ID() uuid.UUID {
	return a.id
}

func (a *account) SetID(id uuid.UUID) {
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

func (a *account) Balance() *money.Money {
	return a.balance
}

func (a *account) UpdateBalance(balance *money.Money) Account {
	result := *a
	result.balance = balance
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
		uuid.Nil,
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

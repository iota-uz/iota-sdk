package payment

import (
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	moneyaccount "github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/money_account"
	paymentcategory "github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/payment_category"
)

type Option func(p *payment)

// Option setters
func WithID(id uint) Option {
	return func(p *payment) {
		p.id = id
	}
}

func WithTenantID(tenantID uuid.UUID) Option {
	return func(p *payment) {
		p.tenantID = tenantID
	}
}

func WithTransactionID(transactionID uint) Option {
	return func(p *payment) {
		p.transactionID = transactionID
	}
}

func WithCounterpartyID(counterpartyID uint) Option {
	return func(p *payment) {
		p.counterpartyID = counterpartyID
	}
}

func WithComment(comment string) Option {
	return func(p *payment) {
		p.comment = comment
	}
}

func WithAccount(account moneyaccount.Account) Option {
	return func(p *payment) {
		p.account = account
	}
}

func WithUser(user user.User) Option {
	return func(p *payment) {
		p.user = user
	}
}

func WithTransactionDate(date time.Time) Option {
	return func(p *payment) {
		p.transactionDate = date
	}
}

func WithAccountingPeriod(period time.Time) Option {
	return func(p *payment) {
		p.accountingPeriod = period
	}
}

func WithCreatedAt(createdAt time.Time) Option {
	return func(p *payment) {
		p.createdAt = createdAt
	}
}

func WithUpdatedAt(updatedAt time.Time) Option {
	return func(p *payment) {
		p.updatedAt = updatedAt
	}
}

func New(
	amount float64,
	category paymentcategory.PaymentCategory,
	opts ...Option,
) Payment {
	p := &payment{
		id:               0,
		tenantID:         uuid.Nil,
		amount:           amount,
		transactionID:    0,
		counterpartyID:   0,
		category:         category,
		transactionDate:  time.Now(),
		accountingPeriod: time.Now(),
		comment:          "",
		account:          nil,
		user:             nil,
		createdAt:        time.Now(),
		updatedAt:        time.Now(),
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

type payment struct {
	id               uint
	tenantID         uuid.UUID
	amount           float64
	transactionID    uint
	counterpartyID   uint
	category         paymentcategory.PaymentCategory
	transactionDate  time.Time
	accountingPeriod time.Time
	comment          string
	account          moneyaccount.Account
	user             user.User
	createdAt        time.Time
	updatedAt        time.Time
}

func (p *payment) ID() uint {
	return p.id
}

func (p *payment) SetID(id uint) {
	p.id = id
}

func (p *payment) Amount() float64 {
	return p.amount
}

func (p *payment) UpdateAmount(a float64) Payment {
	result := *p
	result.amount = a
	result.updatedAt = time.Now()
	return &result
}

func (p *payment) TransactionID() uint {
	return p.transactionID
}

func (p *payment) CounterpartyID() uint {
	return p.counterpartyID
}

func (p *payment) UpdateCounterpartyID(id uint) Payment {
	result := *p
	result.counterpartyID = id
	result.updatedAt = time.Now()
	return &result
}

func (p *payment) Category() paymentcategory.PaymentCategory {
	return p.category
}

func (p *payment) TransactionDate() time.Time {
	return p.transactionDate
}

func (p *payment) UpdateTransactionDate(t time.Time) Payment {
	result := *p
	result.transactionDate = t
	result.updatedAt = time.Now()
	return &result
}

func (p *payment) AccountingPeriod() time.Time {
	return p.accountingPeriod
}

func (p *payment) UpdateAccountingPeriod(t time.Time) Payment {
	result := *p
	result.accountingPeriod = t
	result.updatedAt = time.Now()
	return &result
}

func (p *payment) Comment() string {
	return p.comment
}

func (p *payment) UpdateComment(s string) Payment {
	result := *p
	result.comment = s
	result.updatedAt = time.Now()
	return &result
}

func (p *payment) Account() moneyaccount.Account {
	return p.account
}

func (p *payment) User() user.User {
	return p.user
}

func (p *payment) CreatedAt() time.Time {
	return p.createdAt
}

func (p *payment) UpdatedAt() time.Time {
	return p.updatedAt
}

func (p *payment) TenantID() uuid.UUID {
	return p.tenantID
}

func (p *payment) UpdateTenantID(id uuid.UUID) Payment {
	result := *p
	result.tenantID = id
	return &result
}

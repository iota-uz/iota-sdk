package payment

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	moneyaccount "github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/money_account"
	paymentcategory "github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/payment_category"
	"github.com/iota-uz/iota-sdk/pkg/money"
)

type Option func(p *payment)

// Option setters
func WithID(id uuid.UUID) Option {
	return func(p *payment) {
		p.id = id
	}
}

func WithTenantID(tenantID uuid.UUID) Option {
	return func(p *payment) {
		p.tenantID = tenantID
	}
}

func WithTransactionID(transactionID uuid.UUID) Option {
	return func(p *payment) {
		p.transactionID = transactionID
	}
}

func WithCounterpartyID(counterpartyID uuid.UUID) Option {
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

func WithAttachments(attachments []uint) Option {
	return func(p *payment) {
		p.attachments = attachments
	}
}

func New(
	amount *money.Money,
	category paymentcategory.PaymentCategory,
	opts ...Option,
) Payment {
	p := &payment{
		id:               uuid.New(),
		tenantID:         uuid.Nil,
		amount:           amount,
		transactionID:    uuid.Nil,
		counterpartyID:   uuid.Nil,
		category:         category,
		transactionDate:  time.Now(),
		accountingPeriod: time.Now(),
		comment:          "",
		account:          nil,
		user:             nil,
		createdAt:        time.Now(),
		updatedAt:        time.Now(),
		attachments:      []uint{},
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

type payment struct {
	id               uuid.UUID
	tenantID         uuid.UUID
	amount           *money.Money
	transactionID    uuid.UUID
	counterpartyID   uuid.UUID
	category         paymentcategory.PaymentCategory
	transactionDate  time.Time
	accountingPeriod time.Time
	comment          string
	account          moneyaccount.Account
	user             user.User
	createdAt        time.Time
	updatedAt        time.Time
	attachments      []uint
}

func (p *payment) ID() uuid.UUID {
	return p.id
}

func (p *payment) SetID(id uuid.UUID) {
	p.id = id
}

func (p *payment) Amount() *money.Money {
	return p.amount
}

func (p *payment) UpdateAmount(a *money.Money) Payment {
	result := *p
	result.amount = a
	result.updatedAt = time.Now()
	return &result
}

func (p *payment) TransactionID() uuid.UUID {
	return p.transactionID
}

func (p *payment) CounterpartyID() uuid.UUID {
	return p.counterpartyID
}

func (p *payment) UpdateCounterpartyID(id uuid.UUID) Payment {
	result := *p
	result.counterpartyID = id
	result.updatedAt = time.Now()
	return &result
}

func (p *payment) Category() paymentcategory.PaymentCategory {
	return p.category
}

func (p *payment) UpdateCategory(category paymentcategory.PaymentCategory) Payment {
	result := *p
	result.category = category
	result.updatedAt = time.Now()
	return &result
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

// Attachment methods
func (p *payment) GetAttachments() []uint {
	// Return a copy to prevent external modification
	attachments := make([]uint, len(p.attachments))
	copy(attachments, p.attachments)
	return attachments
}

func (p *payment) HasAttachment(uploadID uint) bool {
	for _, id := range p.attachments {
		if id == uploadID {
			return true
		}
	}
	return false
}

func (p *payment) AttachFile(uploadID uint) (Payment, error) {
	if uploadID == 0 {
		return nil, fmt.Errorf("upload ID cannot be zero")
	}

	if p.HasAttachment(uploadID) {
		return nil, fmt.Errorf("file with ID %d is already attached", uploadID)
	}

	result := *p
	result.attachments = make([]uint, len(p.attachments)+1)
	copy(result.attachments, p.attachments)
	result.attachments[len(p.attachments)] = uploadID
	result.updatedAt = time.Now()
	return &result, nil
}

func (p *payment) DetachFile(uploadID uint) (Payment, error) {
	if uploadID == 0 {
		return nil, fmt.Errorf("upload ID cannot be zero")
	}

	if !p.HasAttachment(uploadID) {
		return nil, fmt.Errorf("file with ID %d is not attached", uploadID)
	}

	result := *p
	result.attachments = make([]uint, 0, len(p.attachments)-1)
	for _, id := range p.attachments {
		if id != uploadID {
			result.attachments = append(result.attachments, id)
		}
	}
	result.updatedAt = time.Now()
	return &result, nil
}

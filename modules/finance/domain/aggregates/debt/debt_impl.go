package debt

import (
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/pkg/money"
)

type Option func(d *debt)

// Option setters
func WithID(id uuid.UUID) Option {
	return func(d *debt) {
		d.id = id
	}
}

func WithTenantID(tenantID uuid.UUID) Option {
	return func(d *debt) {
		d.tenantID = tenantID
	}
}

func WithCounterpartyID(counterpartyID uuid.UUID) Option {
	return func(d *debt) {
		d.counterpartyID = counterpartyID
	}
}

func WithDescription(description string) Option {
	return func(d *debt) {
		d.description = description
	}
}

func WithDueDate(dueDate *time.Time) Option {
	return func(d *debt) {
		d.dueDate = dueDate
	}
}

func WithUser(user user.User) Option {
	return func(d *debt) {
		d.user = user
	}
}

func WithCreatedAt(createdAt time.Time) Option {
	return func(d *debt) {
		d.createdAt = createdAt
	}
}

func WithUpdatedAt(updatedAt time.Time) Option {
	return func(d *debt) {
		d.updatedAt = updatedAt
	}
}

func WithSettlementTransactionID(transactionID *uuid.UUID) Option {
	return func(d *debt) {
		d.settlementTransactionID = transactionID
	}
}

func New(
	debtType DebtType,
	amount *money.Money,
	opts ...Option,
) Debt {
	d := &debt{
		id:                      uuid.New(),
		tenantID:                uuid.Nil,
		debtType:                debtType,
		status:                  DebtStatusPending,
		counterpartyID:          uuid.Nil,
		originalAmount:          amount,
		outstandingAmount:       amount,
		description:             "",
		dueDate:                 nil,
		settlementTransactionID: nil,
		user:                    nil,
		createdAt:               time.Now(),
		updatedAt:               time.Now(),
	}
	for _, opt := range opts {
		opt(d)
	}
	return d
}

type debt struct {
	id                      uuid.UUID
	tenantID                uuid.UUID
	debtType                DebtType
	status                  DebtStatus
	counterpartyID          uuid.UUID
	originalAmount          *money.Money
	outstandingAmount       *money.Money
	description             string
	dueDate                 *time.Time
	settlementTransactionID *uuid.UUID
	user                    user.User
	createdAt               time.Time
	updatedAt               time.Time
}

func (d *debt) ID() uuid.UUID {
	return d.id
}

func (d *debt) SetID(id uuid.UUID) {
	d.id = id
}

func (d *debt) TenantID() uuid.UUID {
	return d.tenantID
}

func (d *debt) UpdateTenantID(id uuid.UUID) Debt {
	result := *d
	result.tenantID = id
	result.updatedAt = time.Now()
	return &result
}

func (d *debt) Type() DebtType {
	return d.debtType
}

func (d *debt) UpdateType(debtType DebtType) Debt {
	result := *d
	result.debtType = debtType
	result.updatedAt = time.Now()
	return &result
}

func (d *debt) Status() DebtStatus {
	return d.status
}

func (d *debt) UpdateStatus(status DebtStatus) Debt {
	result := *d
	result.status = status
	result.updatedAt = time.Now()
	return &result
}

func (d *debt) CounterpartyID() uuid.UUID {
	return d.counterpartyID
}

func (d *debt) UpdateCounterpartyID(id uuid.UUID) Debt {
	result := *d
	result.counterpartyID = id
	result.updatedAt = time.Now()
	return &result
}

func (d *debt) OriginalAmount() *money.Money {
	return d.originalAmount
}

func (d *debt) UpdateOriginalAmount(amount *money.Money) Debt {
	result := *d
	result.originalAmount = amount
	result.updatedAt = time.Now()
	return &result
}

func (d *debt) OutstandingAmount() *money.Money {
	return d.outstandingAmount
}

func (d *debt) UpdateOutstandingAmount(amount *money.Money) Debt {
	result := *d
	result.outstandingAmount = amount
	result.updatedAt = time.Now()
	return &result
}

func (d *debt) Description() string {
	return d.description
}

func (d *debt) UpdateDescription(description string) Debt {
	result := *d
	result.description = description
	result.updatedAt = time.Now()
	return &result
}

func (d *debt) DueDate() *time.Time {
	return d.dueDate
}

func (d *debt) UpdateDueDate(dueDate *time.Time) Debt {
	result := *d
	result.dueDate = dueDate
	result.updatedAt = time.Now()
	return &result
}

func (d *debt) SettlementTransactionID() *uuid.UUID {
	return d.settlementTransactionID
}

func (d *debt) UpdateSettlementTransactionID(transactionID *uuid.UUID) Debt {
	result := *d
	result.settlementTransactionID = transactionID
	result.updatedAt = time.Now()
	return &result
}

func (d *debt) User() user.User {
	return d.user
}

func (d *debt) CreatedAt() time.Time {
	return d.createdAt
}

func (d *debt) UpdatedAt() time.Time {
	return d.updatedAt
}

// CounterpartyAggregate functional options
type CounterpartyAggregateOption func(ca *counterpartyAggregate)

func WithAggCounterpartyID(id uuid.UUID) CounterpartyAggregateOption {
	return func(ca *counterpartyAggregate) {
		ca.counterpartyID = id
	}
}

func WithAggTotalReceivable(amount float64) CounterpartyAggregateOption {
	return func(ca *counterpartyAggregate) {
		ca.totalReceivable = amount
	}
}

func WithAggTotalPayable(amount float64) CounterpartyAggregateOption {
	return func(ca *counterpartyAggregate) {
		ca.totalPayable = amount
	}
}

func WithAggTotalOutstandingReceivable(amount float64) CounterpartyAggregateOption {
	return func(ca *counterpartyAggregate) {
		ca.totalOutstandingReceivable = amount
	}
}

func WithAggTotalOutstandingPayable(amount float64) CounterpartyAggregateOption {
	return func(ca *counterpartyAggregate) {
		ca.totalOutstandingPayable = amount
	}
}

func WithAggDebtCount(count int) CounterpartyAggregateOption {
	return func(ca *counterpartyAggregate) {
		ca.debtCount = count
	}
}

func WithAggCurrencyCode(code string) CounterpartyAggregateOption {
	return func(ca *counterpartyAggregate) {
		ca.currencyCode = code
	}
}

// NewCounterpartyAggregate creates a new CounterpartyAggregate with functional options
func NewCounterpartyAggregate(
	counterpartyID uuid.UUID,
	opts ...CounterpartyAggregateOption,
) CounterpartyAggregate {
	ca := &counterpartyAggregate{
		counterpartyID:             counterpartyID,
		totalReceivable:            0,
		totalPayable:               0,
		totalOutstandingReceivable: 0,
		totalOutstandingPayable:    0,
		debtCount:                  0,
		currencyCode:               "",
	}
	for _, opt := range opts {
		opt(ca)
	}
	return ca
}

// counterpartyAggregate private implementation
type counterpartyAggregate struct {
	counterpartyID             uuid.UUID
	totalReceivable            float64
	totalPayable               float64
	totalOutstandingReceivable float64
	totalOutstandingPayable    float64
	debtCount                  int
	currencyCode               string
}

func (ca *counterpartyAggregate) CounterpartyID() uuid.UUID {
	return ca.counterpartyID
}

func (ca *counterpartyAggregate) TotalReceivable() float64 {
	return ca.totalReceivable
}

func (ca *counterpartyAggregate) TotalPayable() float64 {
	return ca.totalPayable
}

func (ca *counterpartyAggregate) TotalOutstandingReceivable() float64 {
	return ca.totalOutstandingReceivable
}

func (ca *counterpartyAggregate) TotalOutstandingPayable() float64 {
	return ca.totalOutstandingPayable
}

func (ca *counterpartyAggregate) DebtCount() int {
	return ca.debtCount
}

func (ca *counterpartyAggregate) CurrencyCode() string {
	return ca.currencyCode
}

func (ca *counterpartyAggregate) NetAmount() float64 {
	return ca.totalReceivable - ca.totalPayable
}

func (ca *counterpartyAggregate) UpdateCounterpartyID(id uuid.UUID) CounterpartyAggregate {
	result := *ca
	result.counterpartyID = id
	return &result
}

func (ca *counterpartyAggregate) UpdateTotalReceivable(amount float64) CounterpartyAggregate {
	result := *ca
	result.totalReceivable = amount
	return &result
}

func (ca *counterpartyAggregate) UpdateTotalPayable(amount float64) CounterpartyAggregate {
	result := *ca
	result.totalPayable = amount
	return &result
}

func (ca *counterpartyAggregate) UpdateTotalOutstandingReceivable(amount float64) CounterpartyAggregate {
	result := *ca
	result.totalOutstandingReceivable = amount
	return &result
}

func (ca *counterpartyAggregate) UpdateTotalOutstandingPayable(amount float64) CounterpartyAggregate {
	result := *ca
	result.totalOutstandingPayable = amount
	return &result
}

func (ca *counterpartyAggregate) UpdateDebtCount(count int) CounterpartyAggregate {
	result := *ca
	result.debtCount = count
	return &result
}

func (ca *counterpartyAggregate) UpdateCurrencyCode(code string) CounterpartyAggregate {
	result := *ca
	result.currencyCode = code
	return &result
}

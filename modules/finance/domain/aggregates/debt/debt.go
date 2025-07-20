package debt

import (
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/pkg/money"
)

type DebtType string

const (
	DebtTypeReceivable DebtType = "RECEIVABLE" // Money owed to us
	DebtTypePayable    DebtType = "PAYABLE"    // Money we owe
)

type DebtStatus string

const (
	DebtStatusPending    DebtStatus = "PENDING"     // Outstanding debt
	DebtStatusSettled    DebtStatus = "SETTLED"     // Fully paid
	DebtStatusPartial    DebtStatus = "PARTIAL"     // Partially paid
	DebtStatusWrittenOff DebtStatus = "WRITTEN_OFF" // Bad debt written off
)

type Debt interface {
	ID() uuid.UUID
	SetID(id uuid.UUID)

	TenantID() uuid.UUID
	UpdateTenantID(id uuid.UUID) Debt

	Type() DebtType
	UpdateType(debtType DebtType) Debt

	Status() DebtStatus
	UpdateStatus(status DebtStatus) Debt

	CounterpartyID() uuid.UUID
	UpdateCounterpartyID(partyID uuid.UUID) Debt

	OriginalAmount() *money.Money
	UpdateOriginalAmount(amount *money.Money) Debt

	OutstandingAmount() *money.Money
	UpdateOutstandingAmount(amount *money.Money) Debt

	Description() string
	UpdateDescription(description string) Debt

	DueDate() *time.Time
	UpdateDueDate(dueDate *time.Time) Debt

	SettlementTransactionID() *uuid.UUID
	UpdateSettlementTransactionID(transactionID *uuid.UUID) Debt

	User() user.User
	CreatedAt() time.Time
	UpdatedAt() time.Time
}

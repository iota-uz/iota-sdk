package services

import (
	"context"
	"reflect"
	"slices"
	"time"

	"github.com/google/uuid"

	"github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/debt"
	"github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/expense"
	"github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/payment"
	"github.com/iota-uz/iota-sdk/modules/finance/permissions"
	"github.com/iota-uz/iota-sdk/modules/logging/history"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/events"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/native"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/money"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// Versioned IDs of the finance business events. Extension packages subscribe to
// them by these IDs. Each is published in the transaction of the change it
// reports (events.PublishInTx), so it commits or rolls back with that change;
// the eventbus events after commit stay as they were (dual publish).
const (
	PaymentCreatedEventID = "finance.payment.created.v1"
	PaymentUpdatedEventID = "finance.payment.updated.v1"
	ExpenseCreatedEventID = "finance.expense.created.v1"
	DebtCreatedEventID    = "finance.debt.created.v1"
	DebtSettledEventID    = "finance.debt.settled.v1"
	DebtWrittenOffEventID = "finance.debt.written_off.v1"
	DebtCancelledEventID  = "finance.debt.cancelled.v1"
)

// PaymentCreatedPayload is the payload of finance.payment.created.v1.
type PaymentCreatedPayload struct {
	PaymentID       uuid.UUID  `json:"paymentId"`
	Amount          money.Wire `json:"amount"`
	AccountID       uuid.UUID  `json:"accountId"`
	CategoryID      *uuid.UUID `json:"categoryId,omitempty"`
	CounterpartyID  *uuid.UUID `json:"counterpartyId,omitempty"`
	TransactionDate string     `json:"transactionDate" wire:"date"`
	Comment         *string    `json:"comment,omitempty"`
}

// PaymentUpdatedPayload is the payload of finance.payment.updated.v1: the
// payment after the update and the attributes of the payment it changed. The
// vocabulary of Changed is declared by the payload type itself (the amount, the
// account, the date and the comment live in the payment's transaction, not in
// columns of the finance.payment entity, so they are not entity fields).
type PaymentUpdatedPayload struct {
	PaymentID uuid.UUID  `json:"paymentId"`
	Amount    money.Wire `json:"amount"`
	Changed   []string   `json:"changed" wire:"enum=accountId|accountingPeriod|amount|categoryId|comment|counterpartyId|transactionDate"`
}

// ExpenseCreatedPayload is the payload of finance.expense.created.v1.
type ExpenseCreatedPayload struct {
	ExpenseID  uuid.UUID  `json:"expenseId"`
	Amount     money.Wire `json:"amount"`
	AccountID  uuid.UUID  `json:"accountId"`
	CategoryID uuid.UUID  `json:"categoryId"`
	Date       string     `json:"date" wire:"date"`
}

// DebtCreatedPayload is the payload of finance.debt.created.v1.
type DebtCreatedPayload struct {
	DebtID         uuid.UUID  `json:"debtId"`
	Type           string     `json:"type" wire:"enum=receivable|payable"`
	CounterpartyID uuid.UUID  `json:"counterpartyId"`
	OriginalAmount money.Wire `json:"originalAmount"`
	DueDate        *string    `json:"dueDate,omitempty" wire:"date"`
	ProjectID      *uuid.UUID `json:"projectId,omitempty"`
}

// DebtSettledPayload is the payload of finance.debt.settled.v1, published for
// every settlement. Status is partial while something stays outstanding and
// settled once the debt is paid in full. SettledAmount is what this settlement
// paid off: the outstanding amount before it minus OutstandingAmount.
type DebtSettledPayload struct {
	DebtID                  uuid.UUID  `json:"debtId"`
	Type                    string     `json:"type" wire:"enum=receivable|payable"`
	Status                  string     `json:"status" wire:"enum=partial|settled"`
	CounterpartyID          uuid.UUID  `json:"counterpartyId"`
	OriginalAmount          money.Wire `json:"originalAmount"`
	OutstandingAmount       money.Wire `json:"outstandingAmount"`
	SettledAmount           money.Wire `json:"settledAmount"`
	SettlementTransactionID *uuid.UUID `json:"settlementTransactionId,omitempty"`
	DueDate                 *string    `json:"dueDate,omitempty" wire:"date"`
}

// DebtClosedPayload is the payload of finance.debt.written_off.v1 and
// finance.debt.cancelled.v1.
type DebtClosedPayload struct {
	DebtID         uuid.UUID  `json:"debtId"`
	Type           string     `json:"type" wire:"enum=receivable|payable"`
	CounterpartyID uuid.UUID  `json:"counterpartyId"`
	OriginalAmount money.Wire `json:"originalAmount"`
}

func eventLabel(en, ru, uz, uzCyrl string) manifest.Label {
	return manifest.Label{"en": en, "ru": ru, "uz": uz, "uz-Cyrl": uzCyrl}
}

// PaymentCreatedEvent is published by PaymentService.Create.
var PaymentCreatedEvent = native.Event[PaymentCreatedPayload](PaymentCreatedEventID, native.EventSpec{
	Subject:     PaymentResource,
	Label:       eventLabel("Payment created", "Платёж создан", "To'lov yaratildi", "Тўлов яратилди"),
	Description: "A payment was created. Published in the transaction that creates the payment.",
	Descriptions: eventLabel(
		"A payment was created.", "Создан платёж.", "To'lov yaratildi.", "Тўлов яратилди."),
	ReadPermission: permissions.PaymentRead.Name(),
	Sensitive:      map[string][]string{"comment": {permissions.PaymentRead.Name()}},
	Category:       "finance.payments",
})

// PaymentUpdatedEvent is published by PaymentService.Update when an attribute changed.
var PaymentUpdatedEvent = native.Event[PaymentUpdatedPayload](PaymentUpdatedEventID, native.EventSpec{
	Subject:     PaymentResource,
	Label:       eventLabel("Payment updated", "Платёж изменён", "To'lov o'zgartirildi", "Тўлов ўзгартирилди"),
	Description: "A payment was updated. Published in the transaction that updates the payment, only when an attribute changed; changed lists the changed payment attributes (amount, accountId, categoryId, counterpartyId, transactionDate, accountingPeriod, comment).",
	Descriptions: eventLabel(
		"A payment was updated; the changed fields are listed.",
		"Платёж изменён; перечислены изменённые поля.",
		"To'lov o'zgartirildi; o'zgargan maydonlar ko'rsatiladi.",
		"Тўлов ўзгартирилди; ўзгарган майдонлар кўрсатилади."),
	ReadPermission: permissions.PaymentRead.Name(),
	Category:       "finance.payments",
})

// ExpenseCreatedEvent is published by ExpenseService.Create.
var ExpenseCreatedEvent = native.Event[ExpenseCreatedPayload](ExpenseCreatedEventID, native.EventSpec{
	Subject:     ExpenseResource,
	Label:       eventLabel("Expense created", "Расход создан", "Xarajat yaratildi", "Харажат яратилди"),
	Description: "An expense was created. Published in the transaction that creates the expense.",
	Descriptions: eventLabel(
		"An expense was created.", "Создан расход.", "Xarajat yaratildi.", "Харажат яратилди."),
	ReadPermission: permissions.ExpenseRead.Name(),
	Category:       "finance.expenses",
})

// DebtCreatedEvent is published by DebtService.Create.
var DebtCreatedEvent = native.Event[DebtCreatedPayload](DebtCreatedEventID, native.EventSpec{
	Subject:     DebtResource,
	Label:       eventLabel("Debt created", "Долг создан", "Qarz yaratildi", "Қарз яратилди"),
	Description: "A debt was created. Published in the transaction that creates the debt.",
	Descriptions: eventLabel(
		"A debt was created.", "Создан долг.", "Qarz yaratildi.", "Қарз яратилди."),
	ReadPermission: permissions.DebtRead.Name(),
	Category:       "finance.debts",
})

// DebtSettledEvent is published by DebtService.Settle for a partial and for a
// full settlement; status tells them apart.
var DebtSettledEvent = native.Event[DebtSettledPayload](DebtSettledEventID, native.EventSpec{
	Subject:     DebtResource,
	Label:       eventLabel("Debt settled", "Долг погашен", "Qarz to'landi", "Қарз тўланди"),
	Description: "A debt was settled, partially or in full. Published in the transaction that records the settlement; status is partial or settled.",
	Descriptions: eventLabel(
		"A debt was paid off, partially or in full; status tells which.",
		"Долг погашен частично или полностью; статус показывает, как именно.",
		"Qarz qisman yoki to'liq to'landi; holat qaysi ekanini ko'rsatadi.",
		"Қарз қисман ёки тўлиқ тўланди; ҳолат қайсилигини кўрсатади."),
	ReadPermission: permissions.DebtRead.Name(),
	Category:       "finance.debts",
})

// DebtWrittenOffEvent is published by DebtService.WriteOff.
var DebtWrittenOffEvent = native.Event[DebtClosedPayload](DebtWrittenOffEventID, native.EventSpec{
	Subject:     DebtResource,
	Label:       eventLabel("Debt written off", "Долг списан", "Qarz hisobdan chiqarildi", "Қарз ҳисобдан чиқарилди"),
	Description: "A debt was written off as bad. Published in the transaction that writes it off.",
	Descriptions: eventLabel(
		"A debt was written off as bad.", "Долг списан как безнадёжный.",
		"Qarz umidsiz sifatida hisobdan chiqarildi.", "Қарз умидсиз сифатида ҳисобдан чиқарилди."),
	ReadPermission: permissions.DebtRead.Name(),
	Category:       "finance.debts",
})

// DebtCancelledEvent is published by DebtService.Cancel.
var DebtCancelledEvent = native.Event[DebtClosedPayload](DebtCancelledEventID, native.EventSpec{
	Subject:     DebtResource,
	Label:       eventLabel("Debt cancelled", "Долг отменён", "Qarz bekor qilindi", "Қарз бекор қилинди"),
	Description: "A debt that is no longer owed was cancelled. Published in the transaction that cancels it.",
	Descriptions: eventLabel(
		"A debt that is no longer owed was cancelled.", "Отменён долг, который больше не нужно платить.",
		"Endi to'lanmaydigan qarz bekor qilindi.", "Энди тўланмайдиган қарз бекор қилинди."),
	ReadPermission: permissions.DebtRead.Name(),
	Category:       "finance.debts",
})

// FinanceEvents are the finance business events the module contributes to the
// application's native set (finance.Natives).
func FinanceEvents() []native.Item {
	return []native.Item{
		PaymentCreatedEvent, PaymentUpdatedEvent, ExpenseCreatedEvent,
		DebtCreatedEvent, DebtSettledEvent, DebtWrittenOffEvent, DebtCancelledEvent,
	}
}

func ref(entity string, id uuid.UUID) *spi.ObjectRef {
	return &spi.ObjectRef{Entity: entity, Key: id.String()}
}

func nonNilUUID(id uuid.UUID) *uuid.UUID {
	if id == uuid.Nil {
		return nil
	}
	return &id
}

func stringOrNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func dateOnly(t time.Time) string { return t.Format(time.DateOnly) }

func dateOrNil(t *time.Time) *string {
	if t == nil || t.IsZero() {
		return nil
	}
	d := dateOnly(*t)
	return &d
}

func debtTypeWire(t debt.DebtType) string {
	if t == debt.DebtTypePayable {
		return "payable"
	}
	return "receivable"
}

// paymentFields maps the history snapshot keys of a payment to the attribute
// IDs of PaymentUpdatedPayload.Changed.
var paymentFields = []struct{ snapshot, field string }{
	{"amount", "amount"}, {"account_id", "accountId"}, {"category_id", "categoryId"},
	{"counterparty_id", "counterpartyId"}, {"transaction_date", "transactionDate"},
	{"accounting_period", "accountingPeriod"}, {"comment", "comment"},
}

// changedPaymentFields lists the payment attributes whose values differ.
func changedPaymentFields(before, after history.Snapshot) []string {
	out := []string{}
	for _, f := range paymentFields {
		if !reflect.DeepEqual(before[f.snapshot], after[f.snapshot]) {
			out = append(out, f.field)
		}
	}
	slices.Sort(out)
	return out
}

// publishPaymentCreated publishes the creation of created. The amount is the
// one the caller asked for (entity): the stored form reads every amount back in
// a default currency.
func publishPaymentCreated(ctx context.Context, entity, created payment.Payment) error {
	const op = serrors.Op("PaymentService.publishPaymentCreated")
	amount, err := money.ToWire(entity.Amount())
	if err != nil {
		return serrors.Wrap(op, err)
	}
	if created.Account() == nil {
		return serrors.New(serrors.Invalid, "payment has no account").WithOp(op)
	}
	p := PaymentCreatedPayload{
		PaymentID: created.ID(), Amount: amount, AccountID: created.Account().ID(),
		CounterpartyID:  nonNilUUID(created.CounterpartyID()),
		TransactionDate: dateOnly(created.TransactionDate()),
		Comment:         stringOrNil(created.Comment()),
	}
	if created.Category() != nil {
		p.CategoryID = nonNilUUID(created.Category().ID())
	}
	return serrors.Wrap(op, events.PublishInTx(ctx, created.TenantID(), PaymentCreatedEvent, ref(PaymentResource, created.ID()), p))
}

// publishPaymentUpdated publishes nothing when no field changed. The amount is
// the one the caller asked for (entity), see publishPaymentCreated.
func publishPaymentUpdated(ctx context.Context, before, entity, after payment.Payment) error {
	const op = serrors.Op("PaymentService.publishPaymentUpdated")
	changed := changedPaymentFields(paymentSnapshot(before), paymentSnapshot(after))
	if len(changed) == 0 {
		return nil
	}
	amount, err := money.ToWire(entity.Amount())
	if err != nil {
		return serrors.Wrap(op, err)
	}
	return serrors.Wrap(op, events.PublishInTx(ctx, after.TenantID(), PaymentUpdatedEvent, ref(PaymentResource, after.ID()),
		PaymentUpdatedPayload{PaymentID: after.ID(), Amount: amount, Changed: changed}))
}

// publishExpenseCreated publishes the creation of created. The amount is the
// one the caller asked for (entity), see publishPaymentCreated.
func publishExpenseCreated(ctx context.Context, entity, created expense.Expense) error {
	const op = serrors.Op("ExpenseService.publishExpenseCreated")
	amount, err := money.ToWire(entity.Amount())
	if err != nil {
		return serrors.Wrap(op, err)
	}
	if created.Account() == nil || created.Category() == nil {
		return serrors.New(serrors.Invalid, "expense has no account or category").WithOp(op)
	}
	return serrors.Wrap(op, events.PublishInTx(ctx, created.TenantID(), ExpenseCreatedEvent, ref(ExpenseResource, created.ID()), ExpenseCreatedPayload{
		ExpenseID: created.ID(), Amount: amount, AccountID: created.Account().ID(),
		CategoryID: created.Category().ID(), Date: dateOnly(created.Date()),
	}))
}

func publishDebtCreated(ctx context.Context, created debt.Debt) error {
	const op = serrors.Op("DebtService.publishDebtCreated")
	original, err := money.ToWire(created.OriginalAmount())
	if err != nil {
		return serrors.Wrap(op, err)
	}
	return serrors.Wrap(op, events.PublishInTx(ctx, created.TenantID(), DebtCreatedEvent, ref(DebtResource, created.ID()), DebtCreatedPayload{
		DebtID: created.ID(), Type: debtTypeWire(created.Type()), CounterpartyID: created.CounterpartyID(),
		OriginalAmount: original, DueDate: dateOrNil(created.DueDate()), ProjectID: created.ProjectID(),
	}))
}

// publishDebtSettled publishes the settlement of before into after.
func publishDebtSettled(ctx context.Context, before, after debt.Debt) error {
	const op = serrors.Op("DebtService.publishDebtSettled")
	original, err := money.ToWire(after.OriginalAmount())
	if err != nil {
		return serrors.Wrap(op, err)
	}
	outstanding, err := money.ToWire(after.OutstandingAmount())
	if err != nil {
		return serrors.Wrap(op, err)
	}
	settledAmount, err := before.OutstandingAmount().Subtract(after.OutstandingAmount())
	if err != nil {
		return serrors.Wrap(op, err)
	}
	settled, err := money.ToWire(settledAmount)
	if err != nil {
		return serrors.Wrap(op, err)
	}
	status := "partial"
	if after.Status() == debt.DebtStatusSettled {
		status = "settled"
	}
	return serrors.Wrap(op, events.PublishInTx(ctx, after.TenantID(), DebtSettledEvent, ref(DebtResource, after.ID()), DebtSettledPayload{
		DebtID: after.ID(), Type: debtTypeWire(after.Type()), Status: status, CounterpartyID: after.CounterpartyID(),
		OriginalAmount: original, OutstandingAmount: outstanding, SettledAmount: settled,
		SettlementTransactionID: after.SettlementTransactionID(), DueDate: dateOrNil(after.DueDate()),
	}))
}

func debtClosedPayload(closed debt.Debt) (DebtClosedPayload, error) {
	original, err := money.ToWire(closed.OriginalAmount())
	if err != nil {
		return DebtClosedPayload{}, err
	}
	return DebtClosedPayload{
		DebtID: closed.ID(), Type: debtTypeWire(closed.Type()), CounterpartyID: closed.CounterpartyID(), OriginalAmount: original,
	}, nil
}

// publishDebtWrittenOff publishes finance.debt.written_off.v1 for the debt.
func publishDebtWrittenOff(ctx context.Context, closed debt.Debt) error {
	const op = serrors.Op("DebtService.publishDebtWrittenOff")
	p, err := debtClosedPayload(closed)
	if err != nil {
		return serrors.Wrap(op, err)
	}
	return serrors.Wrap(op, events.PublishInTx(ctx, closed.TenantID(), DebtWrittenOffEvent, ref(DebtResource, closed.ID()), p))
}

// publishDebtCancelled publishes finance.debt.cancelled.v1 for the debt.
func publishDebtCancelled(ctx context.Context, closed debt.Debt) error {
	const op = serrors.Op("DebtService.publishDebtCancelled")
	p, err := debtClosedPayload(closed)
	if err != nil {
		return serrors.Wrap(op, err)
	}
	return serrors.Wrap(op, events.PublishInTx(ctx, closed.TenantID(), DebtCancelledEvent, ref(DebtResource, closed.ID()), p))
}

// checkEntityTenant refuses an entity without a tenant, or of another tenant than
// the context's, before the repository normalizes the tenant: an event belongs to
// the tenant of the entity, so a missing or foreign tenant fails the change
// instead of being replaced.
func checkEntityTenant(ctx context.Context, op serrors.Op, entityTenant uuid.UUID) error {
	if entityTenant == uuid.Nil {
		return serrors.New(serrors.Invalid, "the entity has no tenant").WithOp(op)
	}
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return serrors.Wrap(op, err)
	}
	if entityTenant != tenantID {
		return serrors.New(serrors.Invalid, "the entity belongs to another tenant than the context").WithOp(op)
	}
	return nil
}

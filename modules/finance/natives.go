package finance

import (
	"github.com/iota-uz/iota-sdk/modules/finance/permissions"
	"github.com/iota-uz/iota-sdk/modules/finance/services"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/data"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/native"
)

func uuidField(optional bool) manifest.Type {
	return manifest.Type{Kind: manifest.KindUUID, Optional: optional, Nullable: optional}
}

func timeField() manifest.Type {
	return manifest.Type{Kind: manifest.KindDateTime, Optional: true, Nullable: true}
}

// PaymentEntity describes the finance.payment entity (table payments). The
// amount, account and date live in the payment's transaction and are not
// columns of payments.
func PaymentEntity() data.NativeEntity {
	return data.NativeEntity{
		ID: services.PaymentResource, Table: "payments", KeyColumn: "id", KeyType: data.KeyUUID, TenantColumn: "tenant_id",
		ReadPermission: permissions.PaymentRead.Name(),
		Label:          manifest.Label{"en": "Payment", "ru": "Платёж", "uz": "To'lov", "uz-Cyrl": "Тўлов"},
		Fields: []data.NativeField{
			{ID: "counterpartyId", Column: "counterparty_id", Type: uuidField(false)},
			{ID: "categoryId", Column: "payment_category_id", Type: uuidField(true)},
			{ID: "createdAt", Column: "created_at", Type: timeField()},
			{ID: "updatedAt", Column: "updated_at", Type: timeField()},
		},
	}
}

// ExpenseEntity describes the finance.expense entity (table expenses).
func ExpenseEntity() data.NativeEntity {
	return data.NativeEntity{
		ID: services.ExpenseResource, Table: "expenses", KeyColumn: "id", KeyType: data.KeyUUID, TenantColumn: "tenant_id",
		ReadPermission: permissions.ExpenseRead.Name(),
		Label:          manifest.Label{"en": "Expense", "ru": "Расход", "uz": "Xarajat", "uz-Cyrl": "Харажат"},
		Fields: []data.NativeField{
			{ID: "categoryId", Column: "category_id", Type: uuidField(false)},
			{ID: "createdAt", Column: "created_at", Type: timeField()},
			{ID: "updatedAt", Column: "updated_at", Type: timeField()},
		},
	}
}

// DebtEntity describes the finance.debt entity (table debts). Amounts are
// minor units: the entity exposes them as int64 next to their currency.
func DebtEntity() data.NativeEntity {
	return data.NativeEntity{
		ID: services.DebtResource, Table: "debts", KeyColumn: "id", KeyType: data.KeyUUID, TenantColumn: "tenant_id",
		ReadPermission: permissions.DebtRead.Name(),
		Label:          manifest.Label{"en": "Debt", "ru": "Долг", "uz": "Qarz", "uz-Cyrl": "Қарз"},
		Fields: []data.NativeField{
			{ID: "type", Column: "type", Type: manifest.Type{Kind: manifest.KindString, MaxLength: 20}},
			{ID: "status", Column: "status", Type: manifest.Type{Kind: manifest.KindString, MaxLength: 20}},
			{ID: "counterpartyId", Column: "counterparty_id", Type: uuidField(false)},
			{ID: "originalAmount", Column: "original_amount", Type: manifest.Type{Kind: manifest.KindInt64}},
			{ID: "originalCurrency", Column: "original_amount_currency_id", Type: manifest.Type{Kind: manifest.KindString, MaxLength: 3}},
			{ID: "outstandingAmount", Column: "outstanding_amount", Type: manifest.Type{Kind: manifest.KindInt64}},
			{ID: "outstandingCurrency", Column: "outstanding_currency_id", Type: manifest.Type{Kind: manifest.KindString, MaxLength: 3}},
			{ID: "dueDate", Column: "due_date", Type: manifest.Type{Kind: manifest.KindDate, Optional: true, Nullable: true}},
			{ID: "projectId", Column: "project_id", Type: uuidField(true)},
			{ID: "createdAt", Column: "created_at", Type: timeField()},
			{ID: "updatedAt", Column: "updated_at", Type: timeField()},
		},
	}
}

// Natives is the finance module's native registration: the finance.payment,
// finance.expense and finance.debt entities and the payment, expense and debt
// events. Applications compose it into their native.Set (modules.Natives).
func Natives(s *native.Set) error {
	return s.Add(append([]native.Item{
		native.Entity(PaymentEntity()), native.Entity(ExpenseEntity()), native.Entity(DebtEntity()),
	}, services.FinanceEvents()...)...)
}

package billing

import (
	"github.com/iota-uz/iota-sdk/modules/billing/permissions"
	"github.com/iota-uz/iota-sdk/modules/billing/services"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/data"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/native"
)

// TransactionEntity describes the billing.transaction entity (table
// billing_transactions). The details (gateway payloads) are not exposed.
func TransactionEntity() data.NativeEntity {
	return data.NativeEntity{
		ID: services.TransactionResource, Table: "billing_transactions", KeyColumn: "id", KeyType: data.KeyUUID, TenantColumn: "tenant_id",
		ReadPermission: permissions.BillingTransactionRead.Name(),
		Label: manifest.Label{
			"en": "Billing transaction", "ru": "Billing-транзакция", "uz": "Billing tranzaksiyasi", "uz-Cyrl": "Billing транзакцияси",
		},
		Fields: []data.NativeField{
			{ID: "status", Column: "status", Type: manifest.Type{Kind: manifest.KindString, MaxLength: 50}},
			{ID: "currency", Column: "currency", Type: manifest.Type{Kind: manifest.KindString, MaxLength: 3}},
			{ID: "gateway", Column: "gateway", Type: manifest.Type{Kind: manifest.KindString, MaxLength: 50}},
			{ID: "createdAt", Column: "created_at", Type: manifest.Type{Kind: manifest.KindDateTime}},
			{ID: "updatedAt", Column: "updated_at", Type: manifest.Type{Kind: manifest.KindDateTime}},
		},
	}
}

// Natives is the billing module's native registration: the billing.transaction
// entity and the status change event. Applications compose it into their
// native.Set (modules.Natives).
func Natives(s *native.Set) error {
	return s.Add(append([]native.Item{native.Entity(TransactionEntity())}, services.Events()...)...)
}

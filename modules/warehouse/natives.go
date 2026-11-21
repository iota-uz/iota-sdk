package warehouse

import (
	"github.com/iota-uz/iota-sdk/modules/warehouse/permissions"
	"github.com/iota-uz/iota-sdk/modules/warehouse/services/orderservice"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/data"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/native"
)

// OrderEntity describes the warehouse.order entity (table warehouse_orders).
func OrderEntity() data.NativeEntity {
	return data.NativeEntity{
		ID: orderservice.OrderResource, Table: "warehouse_orders", KeyColumn: "id", KeyType: data.KeyInt64, TenantColumn: "tenant_id",
		ReadPermission: permissions.OrderRead.Name(),
		Label: manifest.Label{
			"en": "Warehouse order", "ru": "Складской заказ", "uz": "Ombor buyurtmasi", "uz-Cyrl": "Омбор буюртмаси",
		},
		Fields: []data.NativeField{
			{ID: "type", Column: "type", Type: manifest.Type{Kind: manifest.KindString, MaxLength: 255}},
			{ID: "status", Column: "status", Type: manifest.Type{Kind: manifest.KindString, MaxLength: 255}},
			{ID: "createdAt", Column: "created_at", Type: manifest.Type{Kind: manifest.KindDateTime, Optional: true, Nullable: true}},
		},
	}
}

// Natives is the warehouse module's native registration: the warehouse.order
// entity and the order events. Applications compose it into their native.Set
// (modules.Natives).
func Natives(s *native.Set) error {
	return s.Add(append([]native.Item{native.Entity(OrderEntity())}, orderservice.Events()...)...)
}

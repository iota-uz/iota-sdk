package orderservice

import (
	"context"
	"strconv"

	"github.com/iota-uz/iota-sdk/modules/warehouse/domain/aggregates/order"
	"github.com/iota-uz/iota-sdk/modules/warehouse/permissions"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/events"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/native"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// Versioned IDs of the warehouse order events. Extension packages subscribe to
// them by these IDs. Each is published in the transaction of the change it
// reports, so it commits or rolls back with the order.
const (
	OrderCreatedEventID   = "warehouse.order.created.v1"
	OrderCompletedEventID = "warehouse.order.completed.v1"
)

// OrderCreatedPayload is the payload of warehouse.order.created.v1.
type OrderCreatedPayload struct {
	OrderID int64  `json:"orderId,string" wire:"int64"`
	Type    string `json:"type" wire:"enum=in|out"`
	Status  string `json:"status" wire:"enum=pending|complete"`
}

// OrderCompletedPayload is the payload of warehouse.order.completed.v1.
type OrderCompletedPayload struct {
	OrderID int64  `json:"orderId,string" wire:"int64"`
	Type    string `json:"type" wire:"enum=in|out"`
}

// OrderCreatedEvent is published by OrderService.Create.
var OrderCreatedEvent = native.Event[OrderCreatedPayload](OrderCreatedEventID, native.EventSpec{
	Subject: OrderResource,
	Label: manifest.Label{
		"en": "Warehouse order created", "ru": "Складской заказ создан",
		"uz": "Ombor buyurtmasi yaratildi", "uz-Cyrl": "Омбор буюртмаси яратилди",
	},
	Description: "A warehouse order was created. Published in the transaction that creates the order.",
	Descriptions: manifest.Label{
		"en":      "A warehouse order was created.",
		"ru":      "Создан складской заказ.",
		"uz":      "Ombor buyurtmasi yaratildi.",
		"uz-Cyrl": "Омбор буюртмаси яратилди.",
	},
	ReadPermission: permissions.OrderRead.Name(),
	Category:       "warehouse.orders",
})

// OrderCompletedEvent is published by OrderService.Complete.
var OrderCompletedEvent = native.Event[OrderCompletedPayload](OrderCompletedEventID, native.EventSpec{
	Subject: OrderResource,
	Label: manifest.Label{
		"en": "Warehouse order completed", "ru": "Складской заказ выполнен",
		"uz": "Ombor buyurtmasi bajarildi", "uz-Cyrl": "Омбор буюртмаси бажарилди",
	},
	Description: "A warehouse order was completed. Published in the transaction that completes the order.",
	Descriptions: manifest.Label{
		"en":      "A warehouse order was completed.",
		"ru":      "Складской заказ выполнен.",
		"uz":      "Ombor buyurtmasi bajarildi.",
		"uz-Cyrl": "Омбор буюртмаси бажарилди.",
	},
	ReadPermission: permissions.OrderRead.Name(),
	Category:       "warehouse.orders",
})

// Events are the warehouse order events the module contributes to the
// application's native set (warehouse.Natives).
func Events() []native.Item { return []native.Item{OrderCreatedEvent, OrderCompletedEvent} }

func orderRef(id uint) *spi.ObjectRef {
	return &spi.ObjectRef{Entity: OrderResource, Key: strconv.FormatUint(uint64(id), 10)}
}

func publishOrderCreated(ctx context.Context, created order.Order) error {
	const op = serrors.Op("OrderService.publishOrderCreated")
	return serrors.Wrap(op, events.PublishInTx(ctx, created.TenantID(), OrderCreatedEvent, orderRef(created.ID()), OrderCreatedPayload{
		OrderID: int64(created.ID()), Type: string(created.Type()), Status: string(created.Status()),
	}))
}

func publishOrderCompleted(ctx context.Context, completed order.Order) error {
	const op = serrors.Op("OrderService.publishOrderCompleted")
	return serrors.Wrap(op, events.PublishInTx(ctx, completed.TenantID(), OrderCompletedEvent, orderRef(completed.ID()), OrderCompletedPayload{
		OrderID: int64(completed.ID()), Type: string(completed.Type()),
	}))
}

// publishAggregateEvents publishes the events the aggregate accumulated on the
// eventbus once the transaction holding the change has committed: inside a
// caller's transaction that is when the caller commits, and a rollback
// publishes nothing.
func (s *OrderService) publishAggregateEvents(ctx context.Context, o order.Order) {
	if s.publisher == nil || o == nil {
		return
	}
	pending := o.Events()
	composables.AfterCommit(ctx, func(context.Context) {
		for _, event := range pending {
			s.publisher.Publish(event)
		}
	})
}

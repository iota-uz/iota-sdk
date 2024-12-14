package orders

import (
	"context"
	"github.com/iota-agency/iota-sdk/components/base/toggle"
	"github.com/iota-agency/iota-sdk/pkg/composables"
)

func OrderTypes(ctx context.Context) []toggle.ToggleOption {
	return []toggle.ToggleOption{
		{Value: "in", Label: composables.MustT(ctx, "WarehouseOrders.Single.Types.in")},
		{Value: "out", Label: composables.MustT(ctx, "WarehouseOrders.Single.Types.out")},
	}
}

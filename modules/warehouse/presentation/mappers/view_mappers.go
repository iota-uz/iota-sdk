package mappers

import (
	"strconv"
	"time"

	"github.com/iota-uz/iota-sdk/modules/core/presentation/mappers"
	coreviewmodels "github.com/iota-uz/iota-sdk/modules/core/presentation/viewmodels"
	"github.com/iota-uz/iota-sdk/modules/warehouse/domain/aggregates/order"
	"github.com/iota-uz/iota-sdk/modules/warehouse/domain/aggregates/position"
	"github.com/iota-uz/iota-sdk/modules/warehouse/domain/aggregates/product"
	"github.com/iota-uz/iota-sdk/modules/warehouse/domain/entities/inventory"
	"github.com/iota-uz/iota-sdk/modules/warehouse/domain/entities/unit"
	"github.com/iota-uz/iota-sdk/modules/warehouse/presentation/viewmodels"
	"github.com/iota-uz/iota-sdk/pkg/mapping"
)

func ProductToViewModel(entity product.Product) *viewmodels.Product {
	var pos *viewmodels.Position
	if entity.Position() != nil {
		pos = PositionToViewModel(entity.Position())
	}
	return &viewmodels.Product{
		ID:         strconv.FormatUint(uint64(entity.ID()), 10),
		Status:     string(entity.Status()),
		Rfid:       entity.Rfid(),
		PositionID: strconv.FormatUint(uint64(entity.PositionID()), 10),
		Position:   pos,
		CreatedAt:  entity.CreatedAt().Format(time.RFC3339),
		UpdatedAt:  entity.UpdatedAt().Format(time.RFC3339),
	}
}

func PositionToViewModel(entity position.Position) *viewmodels.Position {
	images := make([]*coreviewmodels.Upload, len(entity.Images()))
	for i, img := range entity.Images() {
		images[i] = mappers.UploadToViewModel(img)
	}
	return &viewmodels.Position{
		ID:        strconv.FormatUint(uint64(entity.ID()), 10),
		Title:     entity.Title(),
		Barcode:   entity.Barcode(),
		UnitID:    strconv.FormatUint(uint64(entity.UnitID()), 10),
		Unit:      *UnitToViewModel(entity.Unit()),
		Images:    images,
		CreatedAt: entity.CreatedAt().Format(time.RFC3339),
		UpdatedAt: entity.UpdatedAt().Format(time.RFC3339),
	}
}

func UnitToViewModel(entity *unit.Unit) *viewmodels.Unit {
	return &viewmodels.Unit{
		ID:         strconv.FormatUint(uint64(entity.ID), 10),
		Title:      entity.Title,
		ShortTitle: entity.ShortTitle,
		CreatedAt:  entity.CreatedAt.Format(time.RFC3339),
		UpdatedAt:  entity.UpdatedAt.Format(time.RFC3339),
	}
}

func OrderItemToViewModel(entity order.Item, inStock int) viewmodels.OrderItem {
	return viewmodels.OrderItem{
		InStock:  strconv.Itoa(inStock),
		Position: *PositionToViewModel(entity.Position()),
		Products: mapping.MapViewModels(entity.Products(), func(e product.Product) viewmodels.Product {
			return *ProductToViewModel(e)
		}),
	}
}

func OrderToViewModel(entity order.Order, inStockByPosition map[uint]int) *viewmodels.Order {
	return &viewmodels.Order{
		ID:     strconv.FormatUint(uint64(entity.ID()), 10),
		Type:   string(entity.Type()),
		Status: string(entity.Status()),
		Items: mapping.MapViewModels(entity.Items(), func(e order.Item) viewmodels.OrderItem {
			return OrderItemToViewModel(e, inStockByPosition[e.Position().ID()])
		}),
		CreatedAt: entity.CreatedAt().Format(time.RFC3339),
	}
}

func CheckToViewModel(entity *inventory.Check) *viewmodels.Check {
	var createdBy *coreviewmodels.User
	if entity.CreatedBy != nil {
		createdBy = mappers.UserToViewModel(entity.CreatedBy)
	}
	var finishedBy *coreviewmodels.User
	if entity.FinishedBy != nil {
		finishedBy = mappers.UserToViewModel(entity.FinishedBy)
	}
	return &viewmodels.Check{
		ID:         strconv.FormatUint(uint64(entity.ID), 10),
		Name:       entity.Name,
		Results:    mapping.MapViewModels(entity.Results, CheckResultToViewModel),
		Status:     string(entity.Status),
		CreatedAt:  entity.CreatedAt.Format(time.RFC3339),
		CreatedBy:  createdBy,
		FinishedBy: finishedBy,
	}
}

func CheckResultToViewModel(entity *inventory.CheckResult) *viewmodels.CheckResult {
	// Note: Position in CheckResult is likely still a struct, not interface
	// This would need to be updated if CheckResult is also refactored
	var pos *viewmodels.Position
	// if entity.Position != nil {
	// 	pos = PositionToViewModel(entity.Position)
	// }
	return &viewmodels.CheckResult{
		ID:               strconv.FormatUint(uint64(entity.ID), 10),
		PositionID:       strconv.FormatUint(uint64(entity.PositionID), 10),
		Position:         pos,
		ExpectedQuantity: strconv.FormatUint(uint64(entity.ExpectedQuantity), 10),
		ActualQuantity:   strconv.FormatUint(uint64(entity.ActualQuantity), 10),
		Difference:       strconv.FormatUint(uint64(entity.Difference), 10),
		CreatedAt:        entity.CreatedAt.Format(time.RFC3339),
	}
}

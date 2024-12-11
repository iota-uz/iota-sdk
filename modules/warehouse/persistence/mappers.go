package persistence

import (
	"github.com/iota-agency/iota-sdk/modules/warehouse/domain/aggregates/order"
	"github.com/iota-agency/iota-sdk/modules/warehouse/domain/aggregates/position"
	"github.com/iota-agency/iota-sdk/modules/warehouse/domain/aggregates/product"
	"github.com/iota-agency/iota-sdk/modules/warehouse/domain/entities/unit"
	"github.com/iota-agency/iota-sdk/modules/warehouse/persistence/models"
	"github.com/iota-agency/iota-sdk/pkg/domain/entities/upload"
	"github.com/iota-agency/iota-sdk/pkg/infrastructure/persistence"
	"github.com/iota-agency/iota-sdk/pkg/mapping"
)

func toDBUnit(unit *unit.Unit) *models.WarehouseUnit {
	return &models.WarehouseUnit{
		ID:         unit.ID,
		Title:      unit.Title,
		ShortTitle: unit.ShortTitle,
		CreatedAt:  unit.CreatedAt,
		UpdatedAt:  unit.UpdatedAt,
	}
}

func toDomainUnit(dbUnit *models.WarehouseUnit) *unit.Unit {
	return &unit.Unit{
		ID:         dbUnit.ID,
		Title:      dbUnit.Title,
		ShortTitle: dbUnit.ShortTitle,
		CreatedAt:  dbUnit.CreatedAt,
		UpdatedAt:  dbUnit.UpdatedAt,
	}
}

func toDBOrder(data *order.Order) (*models.WarehouseOrder, []*models.WarehouseOrderItem) {
	dbItems, _ := mapping.MapDbModels(data.Products, func(p *product.Product) (*models.WarehouseOrderItem, error) {
		return &models.WarehouseOrderItem{
			ProductID: p.ID,
			CreatedAt: data.CreatedAt,
		}, nil
	})
	return &models.WarehouseOrder{
		ID:        data.ID,
		Status:    string(data.Status),
		Type:      string(data.Type),
		CreatedAt: data.CreatedAt,
	}, dbItems
}

func toDomainOrder(dbOrder *models.WarehouseOrder) (*order.Order, error) {
	products, err := mapping.MapDbModels(dbOrder.Products, toDomainProduct)
	if err != nil {
		return nil, err
	}
	status, err := order.NewStatus(dbOrder.Status)
	if err != nil {
		return nil, err
	}
	typeEnum, err := order.NewType(dbOrder.Type)
	if err != nil {
		return nil, err
	}
	return &order.Order{
		ID:        dbOrder.ID,
		Status:    status,
		Type:      typeEnum,
		Products:  products,
		CreatedAt: dbOrder.CreatedAt,
	}, nil
}

func toDBProduct(entity *product.Product) (*models.WarehouseProduct, error) {
	return &models.WarehouseProduct{
		ID:         entity.ID,
		PositionID: entity.PositionID,
		Rfid:       mapping.Pointer(entity.Rfid),
		Status:     string(entity.Status),
		CreatedAt:  entity.CreatedAt,
		UpdatedAt:  entity.UpdatedAt,
	}, nil
}

func toDomainProduct(dbProduct *models.WarehouseProduct) (*product.Product, error) {
	status, err := product.NewStatus(dbProduct.Status)
	if err != nil {
		return nil, err
	}
	var pos *position.Position
	if dbProduct.Position != nil {
		pos, err = toDomainPosition(dbProduct.Position)
		if err != nil {
			return nil, err
		}
	}
	return &product.Product{
		ID:         dbProduct.ID,
		PositionID: dbProduct.PositionID,
		Rfid:       mapping.Value(dbProduct.Rfid),
		Position:   pos,
		Status:     status,
		CreatedAt:  dbProduct.CreatedAt,
		UpdatedAt:  dbProduct.UpdatedAt,
	}, nil
}

func toDomainPosition(dbPosition *models.WarehousePosition) (*position.Position, error) {
	u := unit.Unit{}
	if dbPosition.Unit != nil {
		u = *toDomainUnit(dbPosition.Unit)
	}
	images := make([]upload.Upload, len(dbPosition.Images))
	for i, img := range dbPosition.Images {
		images[i] = *persistence.ToDomainUpload(&img)
	}
	return &position.Position{
		ID:        dbPosition.ID,
		Title:     dbPosition.Title,
		Barcode:   dbPosition.Barcode,
		UnitID:    dbPosition.UnitID,
		Unit:      u,
		Images:    images,
		CreatedAt: dbPosition.CreatedAt,
		UpdatedAt: dbPosition.UpdatedAt,
	}, nil
}

func toDBPosition(entity *position.Position) (*models.WarehousePosition, []*models.WarehousePositionImage) {
	junctionRows := make([]*models.WarehousePositionImage, 0, len(entity.Images))
	for _, image := range entity.Images {
		junctionRows = append(
			junctionRows, &models.WarehousePositionImage{
				WarehousePositionID: entity.ID,
				UploadID:            image.ID,
			},
		)
	}
	return &models.WarehousePosition{
		ID:        entity.ID,
		Title:     entity.Title,
		Barcode:   entity.Barcode,
		UnitID:    entity.UnitID,
		CreatedAt: entity.CreatedAt,
		UpdatedAt: entity.UpdatedAt,
	}, junctionRows
}

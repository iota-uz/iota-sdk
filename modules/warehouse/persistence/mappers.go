package persistence

import (
	"github.com/iota-agency/iota-sdk/modules/core/infrastructure/persistence"
	"github.com/iota-agency/iota-sdk/modules/warehouse/domain/aggregates/order"
	"github.com/iota-agency/iota-sdk/modules/warehouse/domain/aggregates/position"
	"github.com/iota-agency/iota-sdk/modules/warehouse/domain/aggregates/product"
	"github.com/iota-agency/iota-sdk/modules/warehouse/domain/entities/inventory"
	"github.com/iota-agency/iota-sdk/modules/warehouse/domain/entities/unit"
	"github.com/iota-agency/iota-sdk/modules/warehouse/persistence/models"
	"github.com/iota-agency/iota-sdk/pkg/domain/entities/upload"
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

func ToDBOrder(data order.Order) (*models.WarehouseOrder, error) {
	var dbProducts []*models.WarehouseProduct
	for _, item := range data.Items() {
		for _, domainProduct := range item.Products() {
			dbProduct, err := toDBProduct(domainProduct)
			if err != nil {
				return nil, err
			}
			dbProducts = append(dbProducts, dbProduct)
		}
	}
	dbOrder := &models.WarehouseOrder{
		ID:        data.ID(),
		Status:    string(data.Status()),
		Type:      string(data.Type()),
		Products:  dbProducts,
		CreatedAt: data.CreatedAt(),
	}
	return dbOrder, nil
}

func ToDomainOrder(dbOrder *models.WarehouseOrder) (order.Order, error) {
	status, err := order.NewStatus(dbOrder.Status)
	if err != nil {
		return nil, err
	}
	orderType, err := order.NewType(dbOrder.Type)
	if err != nil {
		return nil, err
	}
	var idToPosition = make(map[uint]*models.WarehousePosition)
	var groupedByPositionID = make(map[uint][]*models.WarehouseProduct)
	for _, p := range dbOrder.Products {
		idToPosition[p.PositionID] = p.Position
		groupedByPositionID[p.PositionID] = append(groupedByPositionID[p.PositionID], p)
	}
	domainOrder := order.NewWithID(dbOrder.ID, orderType, status)
	for positionID, products := range groupedByPositionID {
		domainProducts, err := mapping.MapDbModels(products, toDomainProduct)
		if err != nil {
			return nil, err
		}
		positionEntity, err := toDomainPosition(idToPosition[positionID])
		if err != nil {
			return nil, err
		}
		if err := domainOrder.AddItem(*positionEntity, domainProducts...); err != nil {
			return nil, err
		}
	}
	return domainOrder, nil
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

func toDomainInventoryCheck(dbInventoryCheck *models.InventoryCheck) (*inventory.Check, error) {
	status, err := inventory.NewStatus(dbInventoryCheck.Status)
	if err != nil {
		return nil, err
	}
	typ, err := inventory.NewType(dbInventoryCheck.Type)
	if err != nil {
		return nil, err
	}
	results, err := mapping.MapDbModels(dbInventoryCheck.Results, toDomainInventoryCheckResult)
	if err != nil {
		return nil, err
	}
	check := &inventory.Check{
		ID:           dbInventoryCheck.ID,
		Status:       status,
		Type:         typ,
		Name:         dbInventoryCheck.Name,
		Results:      results,
		CreatedAt:    dbInventoryCheck.CreatedAt,
		FinishedAt:   mapping.Value(dbInventoryCheck.FinishedAt),
		FinishedByID: mapping.Value(dbInventoryCheck.FinishedByID),
		CreatedByID:  dbInventoryCheck.CreatedByID,
	}
	if dbInventoryCheck.CreatedBy != nil {
		check.CreatedBy, err = persistence.ToDomainUser(dbInventoryCheck.CreatedBy)
	}
	if dbInventoryCheck.FinishedBy != nil {
		check.FinishedBy, err = persistence.ToDomainUser(dbInventoryCheck.FinishedBy)
	}
	return check, nil
}

func toDBInventoryCheckResult(result *inventory.CheckResult) (*models.InventoryCheckResult, error) {
	return &models.InventoryCheckResult{
		ID:               result.ID,
		PositionID:       result.PositionID,
		ExpectedQuantity: result.ExpectedQuantity,
		ActualQuantity:   result.ActualQuantity,
		Difference:       result.Difference,
		CreatedAt:        result.CreatedAt,
	}, nil
}

func toDomainInventoryCheckResult(result *models.InventoryCheckResult) (*inventory.CheckResult, error) {
	// pos, err := toDomainPosition(result.Position)
	// if err != nil {
	// 	return nil, err
	// }
	return &inventory.CheckResult{
		ID:         result.ID,
		PositionID: result.PositionID,
		// Position:         pos,
		ExpectedQuantity: result.ExpectedQuantity,
		ActualQuantity:   result.ActualQuantity,
		Difference:       result.Difference,
		CreatedAt:        result.CreatedAt,
	}, nil
}

func toDBInventoryCheck(check *inventory.Check) (*models.InventoryCheck, error) {
	results, err := mapping.MapDbModels(check.Results, toDBInventoryCheckResult)
	if err != nil {
		return nil, err
	}
	return &models.InventoryCheck{
		ID:           check.ID,
		Status:       check.Status.String(),
		Type:         check.Type.String(),
		Name:         check.Name,
		Results:      results,
		CreatedAt:    check.CreatedAt,
		FinishedAt:   mapping.Pointer(check.FinishedAt),
		FinishedByID: mapping.Pointer(check.FinishedByID),
		CreatedByID:  check.CreatedByID,
	}, nil
}

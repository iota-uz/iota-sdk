package mappers

import (
	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/upload"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/warehouse/domain/aggregates/position"
	"github.com/iota-uz/iota-sdk/modules/warehouse/domain/aggregates/product"
	"github.com/iota-uz/iota-sdk/modules/warehouse/domain/entities/inventory"
	"github.com/iota-uz/iota-sdk/modules/warehouse/domain/entities/unit"
	"github.com/iota-uz/iota-sdk/modules/warehouse/infrastructure/persistence/models"
	"github.com/iota-uz/iota-sdk/pkg/mapping"
)

func ToDBUnit(unit *unit.Unit) *models.WarehouseUnit {
	return &models.WarehouseUnit{
		ID:         unit.ID,
		TenantID:   unit.TenantID,
		Title:      unit.Title,
		ShortTitle: unit.ShortTitle,
		CreatedAt:  unit.CreatedAt,
		UpdatedAt:  unit.UpdatedAt,
	}
}

func ToDomainUnit(dbUnit *models.WarehouseUnit) *unit.Unit {
	return &unit.Unit{
		ID:         dbUnit.ID,
		TenantID:   dbUnit.TenantID,
		Title:      dbUnit.Title,
		ShortTitle: dbUnit.ShortTitle,
		CreatedAt:  dbUnit.CreatedAt,
		UpdatedAt:  dbUnit.UpdatedAt,
	}
}

func ToDBProduct(entity *product.Product) (*models.WarehouseProduct, error) {
	return &models.WarehouseProduct{
		ID:         entity.ID,
		TenantID:   entity.TenantID,
		PositionID: entity.PositionID,
		Rfid:       mapping.ValueToSQLNullString(entity.Rfid),
		Status:     string(entity.Status),
		CreatedAt:  entity.CreatedAt,
		UpdatedAt:  entity.UpdatedAt,
	}, nil
}

func ToDomainProduct(
	dbProduct *models.WarehouseProduct,
	dbPosition *models.WarehousePosition,
	dbUnit *models.WarehouseUnit,
) (*product.Product, error) {
	status, err := product.NewStatus(dbProduct.Status)
	if err != nil {
		return nil, err
	}
	pos, err := ToDomainPosition(dbPosition, dbUnit)
	if err != nil {
		return nil, err
	}
	return &product.Product{
		ID:         dbProduct.ID,
		TenantID:   dbProduct.TenantID,
		PositionID: dbProduct.PositionID,
		Rfid:       dbProduct.Rfid.String,
		Position:   pos,
		Status:     status,
		CreatedAt:  dbProduct.CreatedAt,
		UpdatedAt:  dbProduct.UpdatedAt,
	}, nil
}

func ToDomainPosition(dbPosition *models.WarehousePosition, dbUnit *models.WarehouseUnit) (*position.Position, error) {
	// TODO: decouple
	images := make([]upload.Upload, len(dbPosition.Images))
	for i, img := range dbPosition.Images {
		images[i] = persistence.ToDomainUpload(&img)
	}
	return &position.Position{
		ID:        dbPosition.ID,
		TenantID:  dbPosition.TenantID,
		Title:     dbPosition.Title,
		Barcode:   dbPosition.Barcode,
		UnitID:    uint(dbPosition.UnitID.Int32),
		Unit:      ToDomainUnit(dbUnit),
		Images:    images,
		CreatedAt: dbPosition.CreatedAt,
		UpdatedAt: dbPosition.UpdatedAt,
	}, nil
}

func ToDBPosition(entity *position.Position) (*models.WarehousePosition, []*models.WarehousePositionImage) {
	junctionRows := make([]*models.WarehousePositionImage, 0, len(entity.Images))
	for _, image := range entity.Images {
		junctionRows = append(
			junctionRows, &models.WarehousePositionImage{
				WarehousePositionID: entity.ID,
				UploadID:            image.ID(),
			},
		)
	}
	dbPosition := &models.WarehousePosition{
		ID:        entity.ID,
		TenantID:  entity.TenantID,
		Title:     entity.Title,
		Barcode:   entity.Barcode,
		UnitID:    mapping.ValueToSQLNullInt32(int32(entity.UnitID)),
		CreatedAt: entity.CreatedAt,
		UpdatedAt: entity.UpdatedAt,
	}
	return dbPosition, junctionRows
}

func ToDomainInventoryPosition(dbPosition *models.InventoryPosition) (*inventory.Position, error) {
	return &inventory.Position{
		ID:       dbPosition.ID,
		Title:    dbPosition.Title,
		Quantity: dbPosition.Quantity,
		RfidTags: dbPosition.RfidTags,
	}, nil
}

func ToDomainInventoryCheck(dbInventoryCheck *models.InventoryCheck) (*inventory.Check, error) {
	status, err := inventory.NewStatus(dbInventoryCheck.Status)
	if err != nil {
		return nil, err
	}
	results, err := mapping.MapDBModels(dbInventoryCheck.Results, ToDomainInventoryCheckResult)
	if err != nil {
		return nil, err
	}
	check := &inventory.Check{
		ID:           dbInventoryCheck.ID,
		TenantID:     dbInventoryCheck.TenantID,
		Status:       status,
		Name:         dbInventoryCheck.Name,
		Results:      results,
		CreatedAt:    dbInventoryCheck.CreatedAt,
		FinishedAt:   mapping.Value(dbInventoryCheck.FinishedAt),
		FinishedByID: mapping.Value(dbInventoryCheck.FinishedByID),
		CreatedByID:  dbInventoryCheck.CreatedByID,
	}
	if dbInventoryCheck.CreatedBy != nil {
		check.CreatedBy, err = persistence.ToDomainUser(dbInventoryCheck.CreatedBy, nil, nil, []uuid.UUID{}, nil)
		if err != nil {
			return nil, err
		}
	}
	if dbInventoryCheck.FinishedBy != nil {
		check.FinishedBy, err = persistence.ToDomainUser(dbInventoryCheck.FinishedBy, nil, nil, []uuid.UUID{}, nil)
		if err != nil {
			return nil, err
		}
	}
	return check, nil
}

func ToDBInventoryCheckResult(result *inventory.CheckResult) (*models.InventoryCheckResult, error) {
	return &models.InventoryCheckResult{
		ID:               result.ID,
		TenantID:         result.TenantID,
		PositionID:       result.PositionID,
		ExpectedQuantity: result.ExpectedQuantity,
		ActualQuantity:   result.ActualQuantity,
		Difference:       result.Difference,
		CreatedAt:        result.CreatedAt,
	}, nil
}

func ToDomainInventoryCheckResult(result *models.InventoryCheckResult) (*inventory.CheckResult, error) {
	// pos, err := ToDomainPosition(result.Position)
	// if err != nil {
	// 	return nil, err
	// }
	return &inventory.CheckResult{
		ID:         result.ID,
		TenantID:   result.TenantID,
		PositionID: result.PositionID,
		// Position:         pos,
		ExpectedQuantity: result.ExpectedQuantity,
		ActualQuantity:   result.ActualQuantity,
		Difference:       result.Difference,
		CreatedAt:        result.CreatedAt,
	}, nil
}

func ToDBInventoryCheck(check *inventory.Check) (*models.InventoryCheck, error) {
	results, err := mapping.MapDBModels(check.Results, ToDBInventoryCheckResult)
	if err != nil {
		return nil, err
	}
	return &models.InventoryCheck{
		ID:           check.ID,
		TenantID:     check.TenantID,
		Status:       string(check.Status),
		Name:         check.Name,
		Results:      results,
		CreatedAt:    check.CreatedAt,
		FinishedAt:   mapping.Pointer(check.FinishedAt),
		FinishedByID: mapping.Pointer(check.FinishedByID),
		CreatedByID:  check.CreatedByID,
	}, nil
}

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
		TenantID:   unit.TenantID.String(),
		Title:      unit.Title,
		ShortTitle: unit.ShortTitle,
		CreatedAt:  unit.CreatedAt,
		UpdatedAt:  unit.UpdatedAt,
	}
}

func ToDomainUnit(dbUnit *models.WarehouseUnit) (*unit.Unit, error) {
	tenantID, err := uuid.Parse(dbUnit.TenantID)
	if err != nil {
		return nil, err
	}
	return &unit.Unit{
		ID:         dbUnit.ID,
		TenantID:   tenantID,
		Title:      dbUnit.Title,
		ShortTitle: dbUnit.ShortTitle,
		CreatedAt:  dbUnit.CreatedAt,
		UpdatedAt:  dbUnit.UpdatedAt,
	}, nil
}

func ToDBProduct(entity product.Product) (*models.WarehouseProduct, error) {
	return &models.WarehouseProduct{
		ID:         entity.ID(),
		TenantID:   entity.TenantID().String(),
		PositionID: entity.PositionID(),
		Rfid:       mapping.ValueToSQLNullString(entity.Rfid()),
		Status:     string(entity.Status()),
		CreatedAt:  entity.CreatedAt(),
		UpdatedAt:  entity.UpdatedAt(),
	}, nil
}

func ToDomainProduct(
	dbProduct *models.WarehouseProduct,
	dbPosition *models.WarehousePosition,
	dbUnit *models.WarehouseUnit,
) (product.Product, error) {
	status, err := product.NewStatus(dbProduct.Status)
	if err != nil {
		return nil, err
	}
	pos, err := ToDomainPosition(dbPosition, dbUnit)
	if err != nil {
		return nil, err
	}
	tenantID, err := uuid.Parse(dbProduct.TenantID)
	if err != nil {
		return nil, err
	}
	return product.New(dbProduct.Rfid.String, status,
		product.WithID(dbProduct.ID),
		product.WithTenantID(tenantID),
		product.WithPositionID(dbProduct.PositionID),
		product.WithPosition(pos),
		product.WithCreatedAt(dbProduct.CreatedAt),
		product.WithUpdatedAt(dbProduct.UpdatedAt),
	), nil
}

func ToDomainPosition(dbPosition *models.WarehousePosition, dbUnit *models.WarehouseUnit) (position.Position, error) {
	// TODO: decouple
	images := make([]upload.Upload, 0, len(dbPosition.Images))
	for _, img := range dbPosition.Images {
		domainUpload, err := persistence.ToDomainUpload(&img)
		if err != nil {
			return nil, err
		}
		images = append(images, domainUpload)
	}
	unit, err := ToDomainUnit(dbUnit)
	if err != nil {
		return nil, err
	}
	tenantID, err := uuid.Parse(dbPosition.TenantID)
	if err != nil {
		return nil, err
	}
	return position.New(dbPosition.Title, dbPosition.Barcode,
		position.WithID(dbPosition.ID),
		position.WithTenantID(tenantID),
		position.WithUnitID(uint(dbPosition.UnitID.Int32)),
		position.WithUnit(unit),
		position.WithImages(images),
		position.WithCreatedAt(dbPosition.CreatedAt),
		position.WithUpdatedAt(dbPosition.UpdatedAt),
	), nil
}

func ToDBPosition(entity position.Position) (*models.WarehousePosition, []*models.WarehousePositionImage) {
	junctionRows := make([]*models.WarehousePositionImage, 0, len(entity.Images()))
	for _, image := range entity.Images() {
		junctionRows = append(
			junctionRows, &models.WarehousePositionImage{
				WarehousePositionID: entity.ID(),
				UploadID:            image.ID(),
			},
		)
	}
	dbPosition := &models.WarehousePosition{
		ID:        entity.ID(),
		TenantID:  entity.TenantID().String(),
		Title:     entity.Title(),
		Barcode:   entity.Barcode(),
		UnitID:    mapping.ValueToSQLNullInt32(int32(entity.UnitID())),
		CreatedAt: entity.CreatedAt(),
		UpdatedAt: entity.UpdatedAt(),
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
	tenantID, err := uuid.Parse(dbInventoryCheck.TenantID)
	if err != nil {
		return nil, err
	}
	check := &inventory.Check{
		ID:           dbInventoryCheck.ID,
		TenantID:     tenantID,
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
		TenantID:         result.TenantID.String(),
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
	tenantID, err := uuid.Parse(result.TenantID)
	if err != nil {
		return nil, err
	}
	return &inventory.CheckResult{
		ID:         result.ID,
		TenantID:   tenantID,
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
		TenantID:     check.TenantID.String(),
		Status:       string(check.Status),
		Name:         check.Name,
		Results:      results,
		CreatedAt:    check.CreatedAt,
		FinishedAt:   mapping.Pointer(check.FinishedAt),
		FinishedByID: mapping.Pointer(check.FinishedByID),
		CreatedByID:  check.CreatedByID,
	}, nil
}

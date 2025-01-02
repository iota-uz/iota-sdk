package mappers

import (
	"github.com/iota-uz/iota-sdk/modules/warehouse/domain/aggregates/order"
	"github.com/iota-uz/iota-sdk/modules/warehouse/domain/aggregates/product"
	"github.com/iota-uz/iota-sdk/modules/warehouse/infrastructure/persistence/models"
)

func ToDBOrder(entity order.Order) (*models.WarehouseOrder, []*models.WarehouseOrderItem, error) {
	var dbOrderItems []*models.WarehouseOrderItem
	for _, item := range entity.Items() {
		for _, domainProduct := range item.Products() {
			dbOrderItems = append(dbOrderItems, &models.WarehouseOrderItem{
				WarehouseProductID: domainProduct.ID,
				WarehouseOrderID:   entity.ID(),
			})
		}
	}
	dbOrder := &models.WarehouseOrder{
		ID:        entity.ID(),
		Status:    string(entity.Status()),
		Type:      string(entity.Type()),
		CreatedAt: entity.CreatedAt(),
	}
	return dbOrder, dbOrderItems, nil
}

func ToDomainOrder(
	dbOrder *models.WarehouseOrder,
	dbProducts []*models.WarehouseProduct,
	dbPositions []*models.WarehousePosition,
	dbUnits []*models.WarehouseUnit,
) (order.Order, error) {
	status, err := order.NewStatus(dbOrder.Status)
	if err != nil {
		return nil, err
	}
	orderType, err := order.NewType(dbOrder.Type)
	if err != nil {
		return nil, err
	}
	var idPositionMap = make(map[uint]*models.WarehousePosition, len(dbPositions))
	for _, p := range dbPositions {
		idPositionMap[p.ID] = p
	}
	var idUnitMap = make(map[uint]*models.WarehouseUnit, len(dbUnits))
	for _, u := range dbUnits {
		idUnitMap[u.ID] = u
	}

	var groupedByPositionID = make(map[uint][]*models.WarehouseProduct, len(dbProducts))
	for _, p := range dbProducts {
		groupedByPositionID[p.PositionID] = append(groupedByPositionID[p.PositionID], p)
	}
	domainOrder := order.NewWithID(dbOrder.ID, orderType, status)
	for positionID, products := range groupedByPositionID {
		dbPos := idPositionMap[positionID]
		dbUnit := idUnitMap[uint(dbPos.UnitID.Int32)]
		domainProducts := make([]*product.Product, len(products))
		for i, p := range products {
			domainProduct, err := ToDomainProduct(p, dbPos, dbUnit)
			if err != nil {
				return nil, err
			}
			domainProducts[i] = domainProduct
		}

		domainPosition, err := ToDomainPosition(dbPos, dbUnit)
		if err != nil {
			return nil, err
		}
		if err := domainOrder.AddItem(domainPosition, domainProducts...); err != nil {
			return nil, err
		}
	}
	return domainOrder, nil
}

package persistence

import (
	"errors"
	"github.com/iota-agency/iota-erp/internal/domain/aggregates/order"
	"github.com/iota-agency/iota-erp/internal/domain/entities/currency"
	"github.com/iota-agency/iota-erp/internal/domain/entities/payment"
	"github.com/iota-agency/iota-erp/internal/domain/entities/product"
	"github.com/iota-agency/iota-erp/internal/domain/entities/transaction"
	"github.com/iota-agency/iota-erp/internal/domain/entities/unit"
	"github.com/iota-agency/iota-erp/internal/infrastracture/persistence/models"
)

func toDBUnit(unit *unit.Unit) *models.WarehouseUnit {
	return &models.WarehouseUnit{
		ID:        unit.ID,
		Name:      unit.Name,
		CreatedAt: unit.CreatedAt,
		UpdatedAt: unit.UpdatedAt,
	}
}

func toDomainUnit(dbUnit *models.WarehouseUnit) *unit.Unit {
	return &unit.Unit{
		ID:        dbUnit.ID,
		Name:      dbUnit.Name,
		CreatedAt: dbUnit.CreatedAt,
		UpdatedAt: dbUnit.UpdatedAt,
	}
}

func toDBOrder(data *order.Order) (*models.WarehouseOrder, []*models.OrderItem) {
	var dbItems []*models.OrderItem
	for _, item := range data.Items {
		dbItems = append(dbItems, &models.OrderItem{
			ProductID: item.Product.ID,
			OrderID:   data.ID,
		})
	}
	return &models.WarehouseOrder{
		ID:        data.ID,
		Status:    data.Status.String(),
		Type:      data.Type.String(),
		CreatedAt: data.CreatedAt,
	}, dbItems
}

func toDomainOrder(dbOrder *models.WarehouseOrder, dbItems []*models.OrderItem, dbProduct []*models.Product) (*order.Order, error) {
	var items []*order.Item
	for _, item := range dbItems {
		var orderProduct *models.Product
		for _, p := range dbProduct {
			if p.ID == item.ProductID {
				orderProduct = p
				break
			}
		}
		if orderProduct == nil {
			return nil, errors.New("product not found")
		}
		p, err := toDomainProduct(orderProduct)
		if err != nil {
			return nil, err
		}
		items = append(items, &order.Item{
			Product: p,
		})
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
		CreatedAt: dbOrder.CreatedAt,
		Items:     items,
	}, nil
}

func toDBProduct(entity *product.Product) *models.Product {
	return &models.Product{
		ID:         entity.ID,
		PositionID: entity.PositionID,
		Rfid:       entity.Rfid,
		Status:     entity.Status.String(),
		CreatedAt:  entity.CreatedAt,
		UpdatedAt:  entity.UpdatedAt,
	}
}

func toDomainProduct(dbProduct *models.Product) (*product.Product, error) {
	status, err := product.NewStatus(dbProduct.Status)
	if err != nil {
		return nil, err
	}
	return &product.Product{
		ID:         dbProduct.ID,
		PositionID: dbProduct.PositionID,
		Rfid:       dbProduct.Rfid,
		Status:     status,
		CreatedAt:  dbProduct.CreatedAt,
		UpdatedAt:  dbProduct.UpdatedAt,
	}, nil
}

func toDBTransaction(entity *transaction.Transaction) *models.Transaction {
	return &models.Transaction{
		ID:               entity.ID,
		Amount:           entity.Amount,
		Comment:          entity.Comment,
		AccountingPeriod: entity.AccountingPeriod,
		TransactionDate:  entity.TransactionDate,
		TransactionType:  entity.TransactionType.String(),
		CreatedAt:        entity.CreatedAt,
	}
}

func toDomainTransaction(dbTransaction *models.Transaction) (*transaction.Transaction, error) {
	_type, err := transaction.NewType(dbTransaction.TransactionType)
	if err != nil {
		return nil, err
	}
	return &transaction.Transaction{
		ID:               dbTransaction.ID,
		Amount:           dbTransaction.Amount,
		TransactionType:  _type,
		Comment:          dbTransaction.Comment,
		AccountingPeriod: dbTransaction.AccountingPeriod,
		TransactionDate:  dbTransaction.TransactionDate,
		CreatedAt:        dbTransaction.CreatedAt,
	}, nil
}

func toDBPayment(entity *payment.Payment) (*models.Payment, *models.Transaction) {
	dbPayment := &models.Payment{
		ID: entity.Id,
	}
	dbTransaction := &models.Transaction{
		Amount:           entity.Amount,
		Comment:          entity.Comment,
		AccountingPeriod: entity.AccountingPeriod,
		TransactionDate:  entity.TransactionDate,
		MoneyAccountID:   entity.MoneyAccountID,
		AmountCurrencyID: entity.AmountCurrencyID,
	}
	return dbPayment, dbTransaction
}

func toDomainPayment(dbPayment *models.Payment, dbTransaction *models.Transaction) (*payment.Payment, error) {
	t, err := toDomainTransaction(dbTransaction)
	if err != nil {
		return nil, err
	}
	return &payment.Payment{
		Id:               dbPayment.ID,
		Amount:           t.Amount,
		Comment:          t.Comment,
		TransactionDate:  t.TransactionDate,
		AccountingPeriod: t.AccountingPeriod,
		MoneyAccountID:   t.MoneyAccountID,
		AmountCurrencyID: t.AmountCurrencyID,
		CreatedAt:        dbPayment.CreatedAt,
		UpdatedAt:        dbPayment.UpdatedAt,
	}, nil
}

func toDbCurrency(entity *currency.Currency) *models.Currency {
	return &models.Currency{
		Code:   entity.Code,
		Name:   entity.Name,
		Symbol: entity.Symbol,
	}
}

func toDomainCurrency(dbCurrency *models.Currency) *currency.Currency {
	return &currency.Currency{
		Code:   dbCurrency.Code,
		Name:   dbCurrency.Name,
		Symbol: dbCurrency.Symbol,
	}
}

package persistence

import (
	"errors"
	"github.com/iota-agency/iota-erp/internal/domain/aggregates/expense"
	"github.com/iota-agency/iota-erp/internal/domain/aggregates/payment"
	"github.com/iota-agency/iota-erp/internal/domain/aggregates/role"
	"github.com/iota-agency/iota-erp/internal/domain/aggregates/user"
	"github.com/iota-agency/iota-erp/internal/domain/entities/permission"
	stage "github.com/iota-agency/iota-erp/internal/domain/entities/project_stages"
	"time"

	category "github.com/iota-agency/iota-erp/internal/domain/aggregates/expense_category"
	moneyAccount "github.com/iota-agency/iota-erp/internal/domain/aggregates/money_account"
	"github.com/iota-agency/iota-erp/internal/domain/aggregates/order"
	"github.com/iota-agency/iota-erp/internal/domain/aggregates/project"
	"github.com/iota-agency/iota-erp/internal/domain/entities/currency"
	"github.com/iota-agency/iota-erp/internal/domain/entities/product"
	"github.com/iota-agency/iota-erp/internal/domain/entities/transaction"
	"github.com/iota-agency/iota-erp/internal/domain/entities/unit"
	"github.com/iota-agency/iota-erp/internal/infrastructure/persistence/models"
)

func toDomainUser(dbUser *models.User) *user.User {
	roles := make([]*role.Role, len(dbUser.Roles))
	for i, r := range dbUser.Roles {
		roles[i] = toDomainRole(&r)
	}
	return &user.User{
		ID:         dbUser.ID,
		FirstName:  dbUser.FirstName,
		LastName:   dbUser.LastName,
		MiddleName: dbUser.MiddleName,
		Email:      dbUser.Email,
		Password:   dbUser.Password,
		AvatarID:   dbUser.AvatarID,
		EmployeeID: dbUser.EmployeeID,
		LastIP:     dbUser.LastIP,
		LastLogin:  dbUser.LastLogin,
		LastAction: dbUser.LastAction,
		CreatedAt:  dbUser.CreatedAt,
		UpdatedAt:  dbUser.UpdatedAt,
		Roles:      roles,
	}
}

func toDBUser(entity *user.User) *models.User {
	roles := make([]models.Role, len(entity.Roles))
	for i, r := range entity.Roles {
		roles[i] = *toDBRole(r)
	}
	return &models.User{
		ID:         entity.ID,
		FirstName:  entity.FirstName,
		LastName:   entity.LastName,
		MiddleName: entity.MiddleName,
		Email:      entity.Email,
		Password:   entity.Password,
		AvatarID:   entity.AvatarID,
		EmployeeID: entity.EmployeeID,
		LastIP:     entity.LastIP,
		LastLogin:  entity.LastLogin,
		LastAction: entity.LastAction,
		CreatedAt:  entity.CreatedAt,
		UpdatedAt:  entity.UpdatedAt,
		Roles:      roles,
	}
}

func toDomainRole(dbRole *models.Role) *role.Role {
	permissions := make([]permission.Permission, len(dbRole.Permissions))
	for i, p := range dbRole.Permissions {
		permissions[i] = *toDomainPermission(&p)
	}
	return &role.Role{
		ID:          dbRole.ID,
		Name:        dbRole.Name,
		Description: dbRole.Description,
		Permissions: permissions,
		CreatedAt:   dbRole.CreatedAt,
		UpdatedAt:   dbRole.UpdatedAt,
	}
}

func toDBRole(entity *role.Role) *models.Role {
	permissions := make([]models.Permission, len(entity.Permissions))
	for i, p := range entity.Permissions {
		permissions[i] = *toDBPermission(&p)
	}
	return &models.Role{
		ID:          entity.ID,
		Name:        entity.Name,
		Description: entity.Description,
		Permissions: permissions,
		CreatedAt:   entity.CreatedAt,
		UpdatedAt:   entity.UpdatedAt,
	}
}

func toDBPermission(entity *permission.Permission) *models.Permission {
	return &models.Permission{
		ID:       entity.ID,
		Name:     entity.Name,
		Resource: string(entity.Resource),
		Action:   string(entity.Action),
		Modifier: string(entity.Modifier),
	}
}

func toDomainPermission(dbPermission *models.Permission) *permission.Permission {
	return &permission.Permission{
		ID:       dbPermission.ID,
		Name:     dbPermission.Name,
		Resource: permission.Resource(dbPermission.Resource),
		Action:   permission.Action(dbPermission.Action),
		Modifier: permission.Modifier(dbPermission.Modifier),
	}
}

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
		ID:          dbUnit.ID,
		Name:        dbUnit.Name,
		Description: dbUnit.Description,
		CreatedAt:   dbUnit.CreatedAt,
		UpdatedAt:   dbUnit.UpdatedAt,
	}
}

func toDBOrder(data *order.Order) (*models.WarehouseOrder, []*models.OrderItem) {
	dbItems := make([]*models.OrderItem, 0, len(data.Items))
	for _, item := range data.Items {
		dbItems = append(
			dbItems, &models.OrderItem{
				ProductID: item.Product.ID,
				OrderID:   data.ID,
				CreatedAt: data.CreatedAt,
			},
		)
	}
	return &models.WarehouseOrder{
		ID:        data.ID,
		Status:    data.Status.String(),
		Type:      data.Type.String(),
		CreatedAt: data.CreatedAt,
	}, dbItems
}

func toDomainOrder(
	dbOrder *models.WarehouseOrder,
	dbItems []*models.OrderItem,
	dbProduct []*models.WarehouseProduct,
) (*order.Order, error) {
	items := make([]*order.Item, 0, len(dbItems))
	for _, item := range dbItems {
		var orderProduct *models.WarehouseProduct
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
		items = append(
			items, &order.Item{
				Product:   p,
				CreatedAt: item.CreatedAt,
			},
		)
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

func toDBProduct(entity *product.Product) *models.WarehouseProduct {
	return &models.WarehouseProduct{
		ID:         entity.ID,
		PositionID: entity.PositionID,
		Rfid:       entity.Rfid,
		Status:     entity.Status.String(),
		CreatedAt:  entity.CreatedAt,
		UpdatedAt:  entity.UpdatedAt,
	}
}

func toDomainProduct(dbProduct *models.WarehouseProduct) (*product.Product, error) {
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
		ID:                   entity.ID,
		Amount:               entity.Amount,
		Comment:              entity.Comment,
		AccountingPeriod:     entity.AccountingPeriod,
		TransactionDate:      entity.TransactionDate,
		DestinationAccountID: entity.DestinationAccountID,
		OriginAccountID:      entity.OriginAccountID,
		TransactionType:      entity.TransactionType.String(),
		CreatedAt:            entity.CreatedAt,
	}
}

func toDomainTransaction(dbTransaction *models.Transaction) (*transaction.Transaction, error) {
	_type, err := transaction.NewType(dbTransaction.TransactionType)
	if err != nil {
		return nil, err
	}

	return &transaction.Transaction{
		ID:                   dbTransaction.ID,
		Amount:               dbTransaction.Amount,
		TransactionType:      _type,
		Comment:              dbTransaction.Comment,
		AccountingPeriod:     dbTransaction.AccountingPeriod,
		TransactionDate:      dbTransaction.TransactionDate,
		DestinationAccountID: dbTransaction.DestinationAccountID,
		OriginAccountID:      dbTransaction.OriginAccountID,
		CreatedAt:            dbTransaction.CreatedAt,
	}, nil
}

func toDBPayment(entity *payment.Payment) (*models.Payment, *models.Transaction) {
	dbTransaction := &models.Transaction{
		ID:                   entity.TransactionID,
		Amount:               entity.Amount,
		Comment:              entity.Comment,
		AccountingPeriod:     entity.AccountingPeriod,
		TransactionDate:      entity.TransactionDate,
		OriginAccountID:      nil,
		DestinationAccountID: &entity.Account.ID,
		TransactionType:      transaction.Income.String(),
		CreatedAt:            entity.CreatedAt,
	}
	dbPayment := &models.Payment{
		ID:            entity.ID,
		StageID:       entity.StageID,
		TransactionID: entity.TransactionID,
		Transaction:   dbTransaction,
		CreatedAt:     entity.CreatedAt,
		UpdatedAt:     entity.UpdatedAt,
	}
	return dbPayment, dbTransaction
}

func toDomainPayment(dbPayment *models.Payment) (*payment.Payment, error) {
	if dbPayment.Transaction == nil {
		return nil, errors.New("transaction is nil")
	}
	t, err := toDomainTransaction(dbPayment.Transaction)
	if err != nil {
		return nil, err
	}
	return &payment.Payment{
		ID:               dbPayment.ID,
		Amount:           t.Amount,
		Comment:          t.Comment,
		TransactionDate:  t.TransactionDate,
		AccountingPeriod: t.AccountingPeriod,
		StageID:          dbPayment.StageID,
		User:             &user.User{},
		TransactionID:    dbPayment.TransactionID,
		Account:          moneyAccount.Account{ID: *t.DestinationAccountID},
		CreatedAt:        dbPayment.CreatedAt,
		UpdatedAt:        dbPayment.UpdatedAt,
	}, nil
}

func toDBCurrency(entity *currency.Currency) *models.Currency {
	return &models.Currency{
		Code:      string(entity.Code),
		Name:      entity.Name,
		Symbol:    string(entity.Symbol),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

func toDomainCurrency(dbCurrency *models.Currency) (*currency.Currency, error) {
	code, err := currency.NewCode(dbCurrency.Code)
	if err != nil {
		return nil, err
	}
	symbol, err := currency.NewSymbol(dbCurrency.Symbol)
	if err != nil {
		return nil, err
	}
	return &currency.Currency{
		Code:   code,
		Name:   dbCurrency.Name,
		Symbol: symbol,
	}, nil
}

func toDBExpenseCategory(entity *category.ExpenseCategory) *models.ExpenseCategory {
	return &models.ExpenseCategory{
		ID:               entity.ID,
		Name:             entity.Name,
		Description:      &entity.Description,
		Amount:           entity.Amount,
		AmountCurrencyID: string(entity.Currency.Code),
		CreatedAt:        entity.CreatedAt,
		UpdatedAt:        entity.UpdatedAt,
	}
}

func toDomainExpenseCategory(dbCategory *models.ExpenseCategory) (*category.ExpenseCategory, error) {
	currencyEntity, err := toDomainCurrency(&dbCategory.AmountCurrency)
	if err != nil {
		return nil, err
	}
	return &category.ExpenseCategory{
		ID:          dbCategory.ID,
		Name:        dbCategory.Name,
		Description: *dbCategory.Description,
		Amount:      dbCategory.Amount,
		Currency:    *currencyEntity,
		CreatedAt:   dbCategory.CreatedAt,
		UpdatedAt:   dbCategory.UpdatedAt,
	}, nil
}

func toDomainProject(dbProject *models.Project) *project.Project {
	return &project.Project{
		ID:          dbProject.ID,
		Name:        dbProject.Name,
		Description: dbProject.Description,
		CreatedAt:   dbProject.CreatedAt,
		UpdatedAt:   dbProject.UpdatedAt,
	}
}

func toDBProject(entity *project.Project) *models.Project {
	return &models.Project{
		ID:          entity.ID,
		Name:        entity.Name,
		Description: entity.Description,
		CreatedAt:   entity.CreatedAt,
		UpdatedAt:   entity.UpdatedAt,
	}
}

func toDomainMoneyAccount(dbAccount *models.MoneyAccount) (*moneyAccount.Account, error) {
	currencyEntity, err := toDomainCurrency(dbAccount.Currency)
	if err != nil {
		return nil, err
	}
	return &moneyAccount.Account{
		ID:            dbAccount.ID,
		Name:          dbAccount.Name,
		AccountNumber: dbAccount.AccountNumber,
		Balance:       dbAccount.Balance,
		Currency:      *currencyEntity,
		Description:   dbAccount.Description,
		CreatedAt:     dbAccount.CreatedAt,
		UpdatedAt:     dbAccount.UpdatedAt,
	}, nil
}

func toDBMoneyAccount(entity *moneyAccount.Account) *models.MoneyAccount {
	return &models.MoneyAccount{
		ID:                entity.ID,
		Name:              entity.Name,
		AccountNumber:     entity.AccountNumber,
		Balance:           entity.Balance,
		BalanceCurrencyID: string(entity.Currency.Code),
		Currency:          toDBCurrency(&entity.Currency),
		Description:       entity.Description,
		CreatedAt:         entity.CreatedAt,
		UpdatedAt:         entity.UpdatedAt,
	}
}

func toDomainProjectStage(dbStage *models.ProjectStage) *stage.ProjectStage {
	return &stage.ProjectStage{
		ID:        dbStage.ID,
		Name:      dbStage.Name,
		CreatedAt: dbStage.CreatedAt,
		UpdatedAt: dbStage.UpdatedAt,
	}
}

func toDBProjectStage(entity *stage.ProjectStage) *models.ProjectStage {
	return &models.ProjectStage{
		ID:        entity.ID,
		Name:      entity.Name,
		ProjectID: entity.ProjectID,
		CreatedAt: entity.CreatedAt,
		UpdatedAt: entity.UpdatedAt,
	}
}

func toDomainExpense(dbExpense *models.Expense) (*expense.Expense, error) {
	categoryEntity, err := toDomainExpenseCategory(dbExpense.Category)
	if err != nil {
		return nil, err
	}
	return &expense.Expense{
		ID:               dbExpense.ID,
		Amount:           -1 * dbExpense.Transaction.Amount,
		Category:         *categoryEntity,
		Account:          moneyAccount.Account{ID: *dbExpense.Transaction.OriginAccountID},
		Comment:          dbExpense.Transaction.Comment,
		TransactionID:    dbExpense.TransactionID,
		AccountingPeriod: dbExpense.Transaction.AccountingPeriod,
		Date:             dbExpense.Transaction.TransactionDate,
		CreatedAt:        dbExpense.CreatedAt,
		UpdatedAt:        dbExpense.UpdatedAt,
	}, nil
}

func toDBExpense(entity *expense.Expense) (*models.Expense, *models.Transaction) {
	dbTransaction := &models.Transaction{
		ID:                   entity.TransactionID,
		Amount:               -1 * entity.Amount,
		Comment:              entity.Comment,
		AccountingPeriod:     entity.AccountingPeriod,
		TransactionDate:      entity.Date,
		OriginAccountID:      &entity.Account.ID,
		DestinationAccountID: nil,
		TransactionType:      transaction.Expense.String(),
		CreatedAt:            entity.CreatedAt,
	}
	dbExpense := &models.Expense{
		ID:            entity.ID,
		CategoryID:    entity.Category.ID,
		TransactionID: entity.TransactionID,
		Transaction:   nil,
		CreatedAt:     entity.CreatedAt,
		UpdatedAt:     entity.UpdatedAt,
	}
	return dbExpense, dbTransaction
}

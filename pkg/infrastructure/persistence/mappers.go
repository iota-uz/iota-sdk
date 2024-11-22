package persistence

import (
	"errors"
	"github.com/iota-agency/iota-sdk/modules/finance/domain/aggregates/expense"
	"github.com/iota-agency/iota-sdk/modules/finance/domain/aggregates/expense_category"
	moneyAccount "github.com/iota-agency/iota-sdk/modules/finance/domain/aggregates/money_account"
	"github.com/iota-agency/iota-sdk/modules/finance/domain/aggregates/payment"
	currency2 "github.com/iota-agency/iota-sdk/modules/finance/domain/entities/currency"
	transaction2 "github.com/iota-agency/iota-sdk/modules/finance/domain/entities/transaction"
	"github.com/iota-agency/iota-sdk/pkg/domain/aggregates/role"
	"github.com/iota-agency/iota-sdk/pkg/domain/aggregates/user"
	"github.com/iota-agency/iota-sdk/pkg/domain/entities/employee"
	"github.com/iota-agency/iota-sdk/pkg/domain/entities/permission"
	stage "github.com/iota-agency/iota-sdk/pkg/domain/entities/project_stages"
	"time"

	"github.com/iota-agency/iota-sdk/pkg/domain/aggregates/project"
	"github.com/iota-agency/iota-sdk/pkg/infrastructure/persistence/models"
)

func toDomainUser(dbUser *models.User) *user.User {
	roles := make([]*role.Role, len(dbUser.Roles))
	for i, r := range dbUser.Roles {
		roles[i] = toDomainRole(&r)
	}
	var middleName string
	if dbUser.MiddleName != nil {
		middleName = *dbUser.MiddleName
	}
	var password string
	if dbUser.Password != nil {
		password = *dbUser.Password
	}
	return &user.User{
		ID:         dbUser.ID,
		FirstName:  dbUser.FirstName,
		LastName:   dbUser.LastName,
		MiddleName: middleName,
		Email:      dbUser.Email,
		Password:   password,
		AvatarID:   dbUser.AvatarID,
		EmployeeID: dbUser.EmployeeID,
		UILanguage: user.UILanguage(dbUser.UiLanguage),
		LastIP:     dbUser.LastIP,
		LastLogin:  dbUser.LastLogin,
		LastAction: dbUser.LastAction,
		CreatedAt:  dbUser.CreatedAt,
		UpdatedAt:  dbUser.UpdatedAt,
		Roles:      roles,
	}
}

func toDBUser(entity *user.User) (*models.User, []models.Role) {
	roles := make([]models.Role, len(entity.Roles))
	for i, r := range entity.Roles {
		dbRole, _ := toDBRole(r)
		roles[i] = *dbRole
	}
	return &models.User{
		ID:         entity.ID,
		FirstName:  entity.FirstName,
		LastName:   entity.LastName,
		MiddleName: &entity.MiddleName,
		Email:      entity.Email,
		UiLanguage: string(entity.UILanguage),
		Password:   &entity.Password,
		AvatarID:   entity.AvatarID,
		EmployeeID: entity.EmployeeID,
		LastIP:     entity.LastIP,
		LastLogin:  entity.LastLogin,
		LastAction: entity.LastAction,
		CreatedAt:  entity.CreatedAt,
		UpdatedAt:  entity.UpdatedAt,
		Roles:      nil,
	}, roles
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

func toDBRole(entity *role.Role) (*models.Role, []models.Permission) {
	permissions := make([]models.Permission, len(entity.Permissions))
	for i, p := range entity.Permissions {
		permissions[i] = *toDBPermission(&p)
	}
	return &models.Role{
		ID:          entity.ID,
		Name:        entity.Name,
		Description: entity.Description,
		Permissions: nil,
		CreatedAt:   entity.CreatedAt,
		UpdatedAt:   entity.UpdatedAt,
	}, permissions
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

func toDBTransaction(entity *transaction2.Transaction) *models.Transaction {
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

func toDomainTransaction(dbTransaction *models.Transaction) (*transaction2.Transaction, error) {
	_type, err := transaction2.NewType(dbTransaction.TransactionType)
	if err != nil {
		return nil, err
	}

	return &transaction2.Transaction{
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
		TransactionType:      transaction2.Income.String(),
		CreatedAt:            entity.CreatedAt,
	}
	dbPayment := &models.Payment{
		ID: entity.ID,
		//StageID:       entity.StageID,
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
		//StageID:          dbPayment.StageID,
		User:          &user.User{},
		TransactionID: dbPayment.TransactionID,
		Account:       moneyAccount.Account{ID: *t.DestinationAccountID},
		CreatedAt:     dbPayment.CreatedAt,
		UpdatedAt:     dbPayment.UpdatedAt,
	}, nil
}

func toDBCurrency(entity *currency2.Currency) *models.Currency {
	return &models.Currency{
		Code:      string(entity.Code),
		Name:      entity.Name,
		Symbol:    string(entity.Symbol),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

func toDomainCurrency(dbCurrency *models.Currency) (*currency2.Currency, error) {
	code, err := currency2.NewCode(dbCurrency.Code)
	if err != nil {
		return nil, err
	}
	symbol, err := currency2.NewSymbol(dbCurrency.Symbol)
	if err != nil {
		return nil, err
	}
	return &currency2.Currency{
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
		TransactionType:      transaction2.Expense.String(),
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

func toDomainEmployee(dbEmployee *models.Employee) *employee.Employee {
	return &employee.Employee{
		ID:        dbEmployee.ID,
		FirstName: dbEmployee.FirstName,
		LastName:  dbEmployee.LastName,
		Email:     dbEmployee.Email,
		Phone:     dbEmployee.Phone,
		CreatedAt: dbEmployee.CreatedAt,
		UpdatedAt: dbEmployee.UpdatedAt,
		Meta: &employee.Meta{
			EmployeeID: dbEmployee.ID,
		},
	}
}

func toDBEmployee(entity *employee.Employee) (*models.Employee, *models.EmployeeMeta) {
	dbEmployee := &models.Employee{
		ID:        entity.ID,
		FirstName: entity.FirstName,
		LastName:  entity.LastName,
		Email:     entity.Email,
		Phone:     entity.Phone,
		CreatedAt: entity.CreatedAt,
		UpdatedAt: entity.UpdatedAt,
	}
	dbEmployeeMeta := &models.EmployeeMeta{
		EmployeeID: entity.ID,
		UpdatedAt:  entity.UpdatedAt,
	}
	return dbEmployee, dbEmployeeMeta
}

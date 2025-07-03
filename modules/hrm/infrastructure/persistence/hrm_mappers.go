package persistence

import (
	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/currency"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/country"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	coremappers "github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/hrm/domain/aggregates/employee"
	"github.com/iota-uz/iota-sdk/modules/hrm/domain/entities/position"
	"github.com/iota-uz/iota-sdk/modules/hrm/infrastructure/persistence/models"
	"github.com/iota-uz/iota-sdk/pkg/mapping"
	"github.com/iota-uz/iota-sdk/pkg/money"
)

func toDomainEmployee(dbEmployee *models.Employee, dbMeta *models.EmployeeMeta) (employee.Employee, error) {
	tin, err := coremappers.ToDomainTin(dbMeta.Tin, country.Uzbekistan)
	if err != nil {
		return nil, err
	}
	pin, err := coremappers.ToDomainPin(dbMeta.Pin, country.Uzbekistan)
	if err != nil {
		return nil, err
	}
	mail, err := internet.NewEmail(dbEmployee.Email)
	if err != nil {
		return nil, err
	}
	var avatarID uint
	if dbEmployee.AvatarID != nil {
		avatarID = *dbEmployee.AvatarID
	}
	currencyCode, err := currency.NewCode(dbEmployee.SalaryCurrencyID.String)
	if err != nil {
		return nil, err
	}
	tenantID, err := uuid.Parse(dbEmployee.TenantID)
	if err != nil {
		return nil, err
	}
	return employee.NewWithID(
		dbEmployee.ID,
		tenantID,
		dbEmployee.FirstName,
		dbEmployee.LastName,
		dbEmployee.MiddleName.String,
		dbEmployee.Phone.String,
		mail,
		money.NewFromFloat(dbEmployee.Salary, string(currencyCode)),
		tin,
		pin,
		employee.NewLanguage(dbMeta.PrimaryLanguage.String, dbMeta.SecondaryLanguage.String),
		dbMeta.HireDate.Time,
		employee.WithAvatarID(avatarID),
		employee.WithBirthDate(dbMeta.BirthDate.Time),
		employee.WithResignationDate(mapping.SQLNullTimeToPointer(dbMeta.ResignationDate)),
		employee.WithNotes(dbMeta.Notes.String),
		employee.WithCreatedAt(dbEmployee.CreatedAt),
		employee.WithUpdatedAt(dbEmployee.UpdatedAt),
	), nil
}

func toDBEmployee(entity employee.Employee) (*models.Employee, *models.EmployeeMeta) {
	salary := entity.Salary()
	dbEmployee := &models.Employee{
		ID:               entity.ID(),
		TenantID:         entity.TenantID().String(),
		FirstName:        entity.FirstName(),
		LastName:         entity.LastName(),
		MiddleName:       mapping.ValueToSQLNullString(entity.MiddleName()),
		Salary:           float64(salary.Amount()) / 100,
		SalaryCurrencyID: mapping.ValueToSQLNullString(salary.Currency().Code),
		Email:            entity.Email().Value(),
		Phone:            mapping.ValueToSQLNullString(entity.Phone()),
		CreatedAt:        entity.CreatedAt(),
		UpdatedAt:        entity.UpdatedAt(),
	}
	lang := entity.Language()
	dbEmployeeMeta := &models.EmployeeMeta{
		PrimaryLanguage:   mapping.ValueToSQLNullString(lang.Primary()),
		SecondaryLanguage: mapping.ValueToSQLNullString(lang.Secondary()),
		Tin:               mapping.ValueToSQLNullString(entity.Tin().Value()),
		Pin:               mapping.ValueToSQLNullString(entity.Pin().Value()),
		Notes:             mapping.ValueToSQLNullString(entity.Notes()),
		BirthDate:         mapping.ValueToSQLNullTime(entity.BirthDate()),
		HireDate:          mapping.ValueToSQLNullTime(entity.HireDate()),
		ResignationDate:   mapping.PointerToSQLNullTime(entity.ResignationDate()),
	}
	return dbEmployee, dbEmployeeMeta
}

func toDomainPosition(dbPosition *models.Position) (*position.Position, error) {
	return &position.Position{
		ID:          dbPosition.ID,
		TenantID:    dbPosition.TenantID,
		Name:        dbPosition.Name,
		Description: dbPosition.Description.String,
		CreatedAt:   dbPosition.CreatedAt,
		UpdatedAt:   dbPosition.UpdatedAt,
	}, nil
}

func toDBPosition(position *position.Position) *models.Position {
	return &models.Position{
		ID:          position.ID,
		TenantID:    position.TenantID,
		Name:        position.Name,
		Description: mapping.ValueToSQLNullString(position.Description),
		CreatedAt:   position.CreatedAt,
		UpdatedAt:   position.UpdatedAt,
	}
}

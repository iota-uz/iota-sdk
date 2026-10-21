// Package persistence provides this package.
package persistence

import (
	"database/sql"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/projects/domain/aggregates/acceptance"
	"github.com/iota-uz/iota-sdk/modules/projects/domain/aggregates/project"
	projectstage "github.com/iota-uz/iota-sdk/modules/projects/domain/aggregates/project_stage"
	"github.com/iota-uz/iota-sdk/modules/projects/infrastructure/persistence/models"
	"github.com/iota-uz/iota-sdk/pkg/mapping"
	"github.com/iota-uz/iota-sdk/pkg/money"
)

func nullStringToString(ns sql.NullString) string {
	if ns.Valid {
		return ns.String
	}
	return ""
}

func ProjectModelToDomain(model models.Project) project.Project {
	return project.New(
		model.Name,
		uuid.MustParse(model.CounterpartyID),
		project.WithID(uuid.MustParse(model.ID)),
		project.WithTenantID(uuid.MustParse(model.TenantID)),
		project.WithDescription(nullStringToString(model.Description)),
		project.WithContract(contractToDomain(model.ContractAmount, model.ContractCurrency)),
		project.WithCreatedAt(model.CreatedAt),
		project.WithUpdatedAt(model.UpdatedAt),
	)
}

func ProjectDomainToModel(domain project.Project) models.Project {
	amount, currency := contractToModel(domain.Contract())
	return models.Project{
		ID:               domain.ID().String(),
		TenantID:         domain.TenantID().String(),
		CounterpartyID:   domain.CounterpartyID().String(),
		Name:             domain.Name(),
		Description:      mapping.ValueToSQLNullString(domain.Description()),
		ContractAmount:   amount,
		ContractCurrency: currency,
		CreatedAt:        domain.CreatedAt(),
		UpdatedAt:        domain.UpdatedAt(),
	}
}

func contractToDomain(amount sql.NullInt64, currency sql.NullString) *money.Money {
	if !amount.Valid || !currency.Valid {
		return nil
	}
	return money.New(amount.Int64, currency.String)
}

func contractToModel(contract *money.Money) (sql.NullInt64, sql.NullString) {
	if contract == nil {
		return sql.NullInt64{}, sql.NullString{}
	}
	return sql.NullInt64{Int64: contract.Amount(), Valid: true},
		sql.NullString{String: contract.Currency().Code, Valid: true}
}

func ProjectStageModelToDomain(model models.ProjectStage) projectstage.ProjectStage {
	return projectstage.New(
		uuid.MustParse(model.ProjectID),
		model.StageNumber,
		model.TotalAmount,
		projectstage.WithID(uuid.MustParse(model.ID)),
		projectstage.WithDescription(nullStringToString(model.Description)),
		projectstage.WithStartDate(mapping.SQLNullTimeToPointer(model.StartDate)),
		projectstage.WithPlannedEndDate(mapping.SQLNullTimeToPointer(model.PlannedEndDate)),
		projectstage.WithFactualEndDate(mapping.SQLNullTimeToPointer(model.FactualEndDate)),
		projectstage.WithCreatedAt(model.CreatedAt),
		projectstage.WithUpdatedAt(model.UpdatedAt),
	)
}

func ProjectStageDomainToModel(domain projectstage.ProjectStage) models.ProjectStage {
	return models.ProjectStage{
		ID:             domain.ID().String(),
		ProjectID:      domain.ProjectID().String(),
		StageNumber:    domain.StageNumber(),
		Description:    mapping.ValueToSQLNullString(domain.Description()),
		TotalAmount:    domain.TotalAmount(),
		StartDate:      mapping.PointerToSQLNullTime(domain.StartDate()),
		PlannedEndDate: mapping.PointerToSQLNullTime(domain.PlannedEndDate()),
		FactualEndDate: mapping.PointerToSQLNullTime(domain.FactualEndDate()),
		CreatedAt:      domain.CreatedAt(),
		UpdatedAt:      domain.UpdatedAt(),
	}
}

func AcceptanceModelToDomain(model models.AcceptanceDocument) acceptance.Document {
	return acceptance.New(
		uuid.MustParse(model.ProjectID),
		acceptance.Kind(model.Kind),
		model.Number,
		model.DocumentDate,
		money.New(model.Amount, model.CurrencyID),
		acceptance.WithID(uuid.MustParse(model.ID)),
		acceptance.WithTenantID(uuid.MustParse(model.TenantID)),
		acceptance.WithStatus(acceptance.Status(model.Status)),
		acceptance.WithDescription(nullStringToString(model.Description)),
		acceptance.WithCreatedAt(model.CreatedAt),
		acceptance.WithUpdatedAt(model.UpdatedAt),
	)
}

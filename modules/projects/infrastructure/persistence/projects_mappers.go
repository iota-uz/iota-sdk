package persistence

import (
	"database/sql"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/projects/domain/aggregates/project"
	projectstage "github.com/iota-uz/iota-sdk/modules/projects/domain/aggregates/project_stage"
	"github.com/iota-uz/iota-sdk/modules/projects/infrastructure/persistence/models"
	"github.com/iota-uz/iota-sdk/pkg/mapping"
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
		project.WithCreatedAt(model.CreatedAt),
		project.WithUpdatedAt(model.UpdatedAt),
	)
}

func ProjectDomainToModel(domain project.Project) models.Project {
	return models.Project{
		ID:             domain.ID().String(),
		TenantID:       domain.TenantID().String(),
		CounterpartyID: domain.CounterpartyID().String(),
		Name:           domain.Name(),
		Description:    mapping.ValueToSQLNullString(domain.Description()),
		CreatedAt:      domain.CreatedAt(),
		UpdatedAt:      domain.UpdatedAt(),
	}
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

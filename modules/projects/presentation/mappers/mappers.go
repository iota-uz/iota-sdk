package mappers

import (
	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/projects/domain/aggregates/project"
	projectstage "github.com/iota-uz/iota-sdk/modules/projects/domain/aggregates/project_stage"
	"github.com/iota-uz/iota-sdk/modules/projects/presentation/controllers/dtos"
	"github.com/iota-uz/iota-sdk/modules/projects/presentation/viewmodels"
	"github.com/iota-uz/iota-sdk/pkg/mapping"
)

// Project mappings
func ProjectDomainToViewModel(p project.Project) viewmodels.ProjectViewModel {
	return viewmodels.ProjectViewModel{
		ID:               p.ID().String(),
		TenantID:         p.TenantID().String(),
		CounterpartyID:   p.CounterpartyID().String(),
		CounterpartyName: "",
		Name:             p.Name(),
		Description:      p.Description(),
		CreatedAt:        p.CreatedAt().Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:        p.UpdatedAt().Format("2006-01-02T15:04:05Z07:00"),
	}
}

func ProjectDomainToViewModels(projects []project.Project) []viewmodels.ProjectViewModel {
	return mapping.MapViewModels(projects, ProjectDomainToViewModel)
}

func ProjectDomainToViewUpdateModel(p project.Project) dtos.ProjectUpdateDTO {
	return dtos.ProjectUpdateDTO{
		CounterpartyID: p.CounterpartyID(),
		Name:           p.Name(),
		Description:    p.Description(),
	}
}

func ProjectViewModelToUpdateDTO(vm viewmodels.ProjectViewModel) dtos.ProjectUpdateDTO {
	counterpartyID, _ := uuid.Parse(vm.CounterpartyID)
	return dtos.ProjectUpdateDTO{
		CounterpartyID: counterpartyID,
		Name:           vm.Name,
		Description:    vm.Description,
	}
}

// Project stage mappings
func ProjectStageDomainToViewModel(ps projectstage.ProjectStage) viewmodels.ProjectStageViewModel {
	return viewmodels.ProjectStageViewModel{
		ID:             ps.ID().String(),
		ProjectID:      ps.ProjectID().String(),
		StageNumber:    ps.StageNumber(),
		Description:    ps.Description(),
		TotalAmount:    ps.TotalAmount(),
		PaidAmount:     ps.PaidAmount(),
		StartDate:      ps.StartDate(),
		PlannedEndDate: ps.PlannedEndDate(),
		FactualEndDate: ps.FactualEndDate(),
		CreatedAt:      ps.CreatedAt(),
		UpdatedAt:      ps.UpdatedAt(),
	}
}

func ProjectStageDomainToViewModels(stages []projectstage.ProjectStage) []viewmodels.ProjectStageViewModel {
	return mapping.MapViewModels(stages, ProjectStageDomainToViewModel)
}

func ProjectStageViewModelToUpdateDTO(vm viewmodels.ProjectStageViewModel) dtos.ProjectStageUpdateDTO {
	return dtos.ProjectStageUpdateDTO{
		StageNumber:    vm.StageNumber,
		Description:    vm.Description,
		TotalAmount:    vm.TotalAmount,
		StartDate:      vm.StartDate,
		PlannedEndDate: vm.PlannedEndDate,
		FactualEndDate: vm.FactualEndDate,
	}
}

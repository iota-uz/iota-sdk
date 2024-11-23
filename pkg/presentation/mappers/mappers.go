package mappers

import (
	"fmt"
	"strconv"
	"time"

	"github.com/iota-agency/iota-sdk/pkg/domain/aggregates/project"
	"github.com/iota-agency/iota-sdk/pkg/domain/aggregates/user"
	"github.com/iota-agency/iota-sdk/pkg/domain/entities/employee"
	stage "github.com/iota-agency/iota-sdk/pkg/domain/entities/project_stages"
	"github.com/iota-agency/iota-sdk/pkg/domain/entities/upload"
	"github.com/iota-agency/iota-sdk/pkg/presentation/viewmodels"
)

func UserToViewModel(entity *user.User) *viewmodels.User {
	return &viewmodels.User{
		ID:         strconv.FormatUint(uint64(entity.ID), 10),
		FirstName:  entity.FirstName,
		LastName:   entity.LastName,
		MiddleName: entity.MiddleName,
		Email:      entity.Email,
		UILanguage: string(entity.UILanguage),
		CreatedAt:  entity.CreatedAt.Format(time.RFC3339),
		UpdatedAt:  entity.UpdatedAt.Format(time.RFC3339),
	}
}

func ProjectStageToViewModel(entity *stage.ProjectStage) *viewmodels.ProjectStage {
	return &viewmodels.ProjectStage{
		ID:        strconv.FormatUint(uint64(entity.ID), 10),
		Name:      entity.Name,
		ProjectID: strconv.FormatUint(uint64(entity.ProjectID), 10),
		Margin:    fmt.Sprintf("%.2f", entity.Margin),
		Risks:     fmt.Sprintf("%.2f", entity.Risks),
		UpdatedAt: entity.UpdatedAt.Format(time.RFC3339),
		CreatedAt: entity.CreatedAt.Format(time.RFC3339),
	}
}

func ProjectToViewModel(entity *project.Project) *viewmodels.Project {
	return &viewmodels.Project{
		ID:          strconv.FormatUint(uint64(entity.ID), 10),
		Name:        entity.Name,
		Description: entity.Description,
		UpdatedAt:   entity.UpdatedAt.Format(time.RFC3339),
		CreatedAt:   entity.CreatedAt.Format(time.RFC3339),
	}
}

func EmployeeToViewModel(entity *employee.Employee) *viewmodels.Employee {
	return &viewmodels.Employee{
		ID:        strconv.FormatUint(uint64(entity.ID), 10),
		FirstName: entity.FirstName,
		LastName:  entity.LastName,
		Email:     entity.Email,
		Phone:     entity.Phone,
		UpdatedAt: entity.UpdatedAt.Format(time.RFC3339),
		CreatedAt: entity.CreatedAt.Format(time.RFC3339),
	}
}

func UploadToViewModel(entity *upload.Upload) *viewmodels.Upload {
	return &viewmodels.Upload{
		ID:        entity.ID,
		URL:       entity.URL,
		Name:      entity.Name,
		Type:      entity.Type,
		Size:      strconv.Itoa(entity.Size),
		CreatedAt: entity.CreatedAt.Format(time.RFC3339),
		UpdatedAt: entity.UpdatedAt.Format(time.RFC3339),
	}
}

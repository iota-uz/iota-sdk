package mappers

import (
	"fmt"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/employee"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/project"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/currency"
	stage "github.com/iota-uz/iota-sdk/modules/core/domain/entities/project_stages"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/tab"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/upload"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/assets"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/viewmodels"
	"slices"
	"strconv"
	"time"
)

func UserToViewModel(entity *user.User) *viewmodels.User {
	var avatarId string
	if v := entity.AvatarID; v != nil {
		avatarId = strconv.Itoa(int(*v))
	}
	var avatar viewmodels.Upload
	if entity.Avatar != nil {
		avatar = *UploadToViewModel(entity.Avatar)
	}
	return &viewmodels.User{
		ID:         strconv.FormatUint(uint64(entity.ID), 10),
		FirstName:  entity.FirstName,
		LastName:   entity.LastName,
		MiddleName: entity.MiddleName,
		Email:      entity.Email,
		Avatar:     &avatar,
		UILanguage: string(entity.UILanguage),
		CreatedAt:  entity.CreatedAt.Format(time.RFC3339),
		UpdatedAt:  entity.UpdatedAt.Format(time.RFC3339),
		AvatarID:   avatarId,
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

func EmployeeToViewModel(entity employee.Employee) *viewmodels.Employee {
	return &viewmodels.Employee{
		ID:        strconv.FormatUint(uint64(entity.ID()), 10),
		FirstName: entity.FirstName(),
		LastName:  entity.LastName(),
		Email:     entity.Email().Value(),
		Phone:     entity.Phone(),
		UpdatedAt: entity.UpdatedAt().Format(time.RFC3339),
		CreatedAt: entity.CreatedAt().Format(time.RFC3339),
	}
}

func UploadToViewModel(entity *upload.Upload) *viewmodels.Upload {
	var url string
	// TODO: this is gotta be implemented better
	if slices.Contains([]string{".xls", ".xlsx"}, entity.Mimetype.Extension()) {
		url = "/assets/" + assets.HashFS.HashName("images/excel-logo.svg")
	} else {
		url = "/" + entity.Path
	}

	return &viewmodels.Upload{
		ID:        strconv.FormatUint(uint64(entity.ID), 10),
		Hash:      entity.Hash,
		URL:       url,
		Mimetype:  entity.Mimetype.String(),
		Size:      strconv.Itoa(entity.Size),
		CreatedAt: entity.CreatedAt.Format(time.RFC3339),
		UpdatedAt: entity.UpdatedAt.Format(time.RFC3339),
	}
}

func CurrencyToViewModel(entity *currency.Currency) *viewmodels.Currency {
	return &viewmodels.Currency{
		Code:   string(entity.Code),
		Name:   entity.Name,
		Symbol: string(entity.Symbol),
	}
}

func TabToViewModel(entity *tab.Tab) *viewmodels.Tab {
	return &viewmodels.Tab{
		ID:   strconv.FormatUint(uint64(entity.ID), 10),
		Href: entity.Href,
	}
}

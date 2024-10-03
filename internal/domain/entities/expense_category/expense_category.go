package category

import (
	model "github.com/iota-agency/iota-erp/internal/interfaces/graph/gqlmodels"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"strings"
	"time"
)

type ExpenseCategory struct {
	Id          uint
	Name        string
	Description *string
	Amount      float64
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type CreateDTO struct {
	Name        string  `schema:"name,required"`
	Amount      float64 `schema:"amount,required"`
	Description string  `schema:"description"`
}

type UpdateDTO struct {
	Name        string  `schema:"name"`
	Amount      float64 `schema:"amount"`
	Description string  `schema:"description"`
}

func (e *ExpenseCategory) Ok(l *i18n.Localizer) (map[string]string, bool) {
	errors := map[string]string{}
	if strings.TrimSpace(e.Name) == "" {
		errors["name"] = l.MustLocalize(&i18n.LocalizeConfig{MessageID: "Validations.Required"})
	}
	if e.Amount == 0 {
		errors["amount"] = l.MustLocalize(&i18n.LocalizeConfig{MessageID: "Validations.Required"})
	}
	return errors, len(errors) == 0
}

func (e *CreateDTO) Ok(l *i18n.Localizer) (map[string]string, bool) {
	errors := map[string]string{}
	if strings.TrimSpace(e.Name) == "" {
		errors["name"] = l.MustLocalize(&i18n.LocalizeConfig{MessageID: "Validations.Required"})
	}
	if e.Amount == 0 {
		errors["amount"] = l.MustLocalize(&i18n.LocalizeConfig{MessageID: "Validations.Required"})
	}

	return errors, len(errors) == 0
}

func (e *UpdateDTO) Ok(l *i18n.Localizer) (map[string]string, bool) {
	errors := map[string]string{}
	return errors, len(errors) == 0
}

func (e *CreateDTO) ToEntity() *ExpenseCategory {
	return &ExpenseCategory{
		Name:        e.Name,
		Amount:      e.Amount,
		Description: &e.Description,
	}
}

func (e *UpdateDTO) ToEntity(id uint) *ExpenseCategory {
	return &ExpenseCategory{
		Id:          id,
		Name:        e.Name,
		Amount:      e.Amount,
		Description: &e.Description,
	}
}

func (e *ExpenseCategory) ToGraph() *model.ExpenseCategory {
	return &model.ExpenseCategory{
		ID:          e.Id,
		Name:        e.Name,
		Amount:      e.Amount,
		Description: e.Description,
		CreatedAt:   e.CreatedAt,
		UpdatedAt:   e.UpdatedAt,
	}
}

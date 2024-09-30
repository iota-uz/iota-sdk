package category

import (
	model "github.com/iota-agency/iota-erp/internal/interfaces/graph/gqlmodels"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"strings"
	"time"
)

type ExpenseCategory struct {
	Id          int64
	Name        string
	Description *string
	Amount      float64
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type ExpenseCategoryUpdate struct {
	Name        string
	Description string
	Amount      float64
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

func (e *ExpenseCategoryUpdate) Ok(l *i18n.Localizer) (map[string]string, bool) {
	errors := map[string]string{}
	if strings.TrimSpace(e.Name) == "" {
		errors["name"] = l.MustLocalize(&i18n.LocalizeConfig{MessageID: "Validations.Required"})
	}
	if e.Amount == 0 {
		errors["amount"] = l.MustLocalize(&i18n.LocalizeConfig{MessageID: "Validations.Required"})
	}
	return errors, len(errors) == 0
}

func (e *ExpenseCategory) ToGraph() *model.ExpenseCategory {
	return &model.ExpenseCategory{
		ID:          e.Id,
		Name:        e.Name,
		Description: e.Description,
		Amount:      e.Amount,
		CreatedAt:   e.CreatedAt,
		UpdatedAt:   e.UpdatedAt,
	}
}

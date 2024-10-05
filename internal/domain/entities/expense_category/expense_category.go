package category

import (
	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	model "github.com/iota-agency/iota-erp/internal/interfaces/graph/gqlmodels"
	"github.com/iota-agency/iota-erp/pkg/middleware"
	"time"
)

type ExpenseCategory struct {
	Id           uint
	Name         string
	Description  string
	Amount       float64
	CurrencyCode string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type CreateDTO struct {
	Name         string  `validate:"required"`
	Amount       float64 `validate:"required,gt=0"`
	CurrencyCode string  `validate:"required,len=3"`
	Description  string
}

type UpdateDTO struct {
	Name         string
	Amount       float64 `validate:"gt=0"`
	CurrencyCode string  `validate:"len=3"`
	Description  string
}

func (e *ExpenseCategory) Ok(l ut.Translator) (map[string]string, bool) {
	errors := map[string]string{}
	err := middleware.Validate.Struct(e)
	if err == nil {
		return errors, true
	}
	for _, _err := range err.(validator.ValidationErrors) {
		errors[_err.Field()] = _err.Translate(l)
	}
	return errors, len(errors) == 0
}

func (e *CreateDTO) Ok(l ut.Translator) (map[string]string, bool) {
	errors := map[string]string{}
	errs := middleware.Validate.Struct(e)
	if errs == nil {
		return errors, true
	}

	for _, err := range errs.(validator.ValidationErrors) {
		errors[err.Field()] = err.Translate(l)
	}
	return errors, len(errors) == 0
}

func (e *UpdateDTO) Ok(l ut.Translator) (map[string]string, bool) {
	errors := map[string]string{}
	errs := middleware.Validate.Struct(e)
	if errs == nil {
		return errors, true
	}
	for _, err := range errs.(validator.ValidationErrors) {
		errors[err.Field()] = err.Translate(l)
	}
	return errors, len(errors) == 0
}

func (e *CreateDTO) ToEntity() *ExpenseCategory {
	return &ExpenseCategory{
		Name:         e.Name,
		Amount:       e.Amount,
		CurrencyCode: e.CurrencyCode,
		Description:  e.Description,
	}
}

func (e *UpdateDTO) ToEntity(id uint) *ExpenseCategory {
	return &ExpenseCategory{
		Id:           id,
		Name:         e.Name,
		Amount:       e.Amount,
		CurrencyCode: e.CurrencyCode,
		Description:  e.Description,
	}
}

func (e *ExpenseCategory) ToGraph() *model.ExpenseCategory {
	return &model.ExpenseCategory{
		ID:          e.Id,
		Name:        e.Name,
		Amount:      e.Amount,
		Description: &e.Description,
		CreatedAt:   e.CreatedAt,
		UpdatedAt:   e.UpdatedAt,
	}
}

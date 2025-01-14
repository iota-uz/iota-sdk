package category

import (
	"time"

	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"

	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/currency"
	"github.com/iota-uz/iota-sdk/pkg/constants"
)

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

func (e *CreateDTO) Ok(l ut.Translator) (map[string]string, bool) {
	errorMessages := map[string]string{}
	errs := constants.Validate.Struct(e)
	if errs == nil {
		return errorMessages, true
	}

	for _, err := range errs.(validator.ValidationErrors) {
		errorMessages[err.Field()] = err.Translate(l)
	}
	return errorMessages, len(errorMessages) == 0
}

func (e *UpdateDTO) Ok(l ut.Translator) (map[string]string, bool) {
	errors := map[string]string{}
	errs := constants.Validate.Struct(e)
	if errs == nil {
		return errors, true
	}
	for _, err := range errs.(validator.ValidationErrors) {
		errors[err.Field()] = err.Translate(l)
	}
	return errors, len(errors) == 0
}

func (e *CreateDTO) ToEntity() (ExpenseCategory, error) {
	code, err := currency.NewCode(e.CurrencyCode)
	if err != nil {
		return nil, err
	}

	return &expenseCategory{
		id:          0,
		name:        e.Name,
		amount:      e.Amount,
		currency:    &currency.Currency{Code: code},
		description: e.Description,
		createdAt:   time.Now(),
		updatedAt:   time.Now(),
	}, nil
}

func (e *UpdateDTO) ToEntity(id uint) (ExpenseCategory, error) {
	code, err := currency.NewCode(e.CurrencyCode)
	if err != nil {
		return nil, err
	}
	return &expenseCategory{
		id:          id,
		name:        e.Name,
		amount:      e.Amount,
		currency:    &currency.Currency{Code: code},
		description: e.Description,
		createdAt:   time.Now(),
		updatedAt:   time.Now(),
	}, nil
}

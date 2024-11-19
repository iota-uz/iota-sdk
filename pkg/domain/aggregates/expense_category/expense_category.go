package category

import (
	"time"

	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	"github.com/iota-agency/iota-sdk/pkg/constants"
	"github.com/iota-agency/iota-sdk/pkg/domain/entities/currency"
	model "github.com/iota-agency/iota-sdk/pkg/interfaces/graph/gqlmodels"
)

type ExpenseCategory struct {
	ID          uint
	Name        string
	Description string
	Amount      float64
	Currency    currency.Currency
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (e *ExpenseCategory) Ok(l ut.Translator) (map[string]string, bool) {
	errors := map[string]string{}
	err := constants.Validate.Struct(e)
	if err == nil {
		return errors, true
	}
	for _, _err := range err.(validator.ValidationErrors) {
		errors[_err.Field()] = _err.Translate(l)
	}
	return errors, len(errors) == 0
}

func (e *ExpenseCategory) ToGraph() *model.ExpenseCategory {
	return &model.ExpenseCategory{
		ID:          int64(e.ID),
		Name:        e.Name,
		Amount:      e.Amount,
		Description: &e.Description,
		CreatedAt:   e.CreatedAt,
		UpdatedAt:   e.UpdatedAt,
	}
}

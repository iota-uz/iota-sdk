package category

import (
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/currency"
	"time"

	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	"github.com/iota-uz/iota-sdk/pkg/constants"
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

package moneyaccount

import (
	"time"

	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/currency"
	"github.com/iota-uz/iota-sdk/pkg/constants"
)

type CreateDTO struct {
	Name          string  `validate:"required"`
	Balance       float64 `validate:"gte=0"`
	AccountNumber string
	CurrencyCode  string `validate:"required,len=3"`
	Description   string
}

type UpdateDTO struct {
	Name          string  `validate:"lte=255"`
	Balance       float64 `validate:"gte=0"`
	AccountNumber string
	CurrencyCode  string `validate:"len=3"`
	Description   string
}

func (p *CreateDTO) Ok(l ut.Translator) (map[string]string, bool) {
	errors := map[string]string{}
	errs := constants.Validate.Struct(p)
	if errs == nil {
		return errors, true
	}
	for _, err := range errs.(validator.ValidationErrors) {
		errors[err.Field()] = err.Translate(l)
	}
	return errors, len(errors) == 0
}

func (p *CreateDTO) ToEntity() (*Account, error) {
	c, err := currency.NewCode(p.CurrencyCode)
	if err != nil {
		return nil, err
	}
	return &Account{
		ID:            0,
		TenantID:      uuid.Nil,
		Name:          p.Name,
		AccountNumber: p.AccountNumber,
		Balance:       p.Balance,
		Currency: currency.Currency{
			Name:   "",
			Code:   c,
			Symbol: "",
		},
		Description: p.Description,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}, nil
}

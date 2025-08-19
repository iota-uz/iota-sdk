package dtos

import (
	"context"
	"fmt"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/iota-uz/go-i18n/v2/i18n"
	"github.com/iota-uz/iota-sdk/pkg/constants"
	"github.com/iota-uz/iota-sdk/pkg/intl"
)

type TransferDTO struct {
	DestinationAccountID string  `validate:"required,uuid"`
	Amount               float64 `validate:"required,gt=0"`
	Comment              string

	// Exchange rate fields for cross-currency transfers
	ExchangeRate      *float64 `validate:"omitempty,gt=0"`
	DestinationAmount *float64 `validate:"omitempty,gt=0"`
	IsExchange        bool     // Flag to indicate if this is a cross-currency transfer
}

func (p *TransferDTO) Ok(ctx context.Context) (map[string]string, bool) {
	l, ok := intl.UseLocalizer(ctx)
	if !ok {
		panic(intl.ErrNoLocalizer)
	}
	errorMessages := map[string]string{}
	errs := constants.Validate.Struct(p)
	if errs == nil {
		return errorMessages, true
	}
	for _, err := range errs.(validator.ValidationErrors) {
		translatedFieldName := l.MustLocalize(&i18n.LocalizeConfig{
			MessageID: fmt.Sprintf("Transfers.Single.%s", err.Field()),
		})
		errorMessages[err.Field()] = l.MustLocalize(&i18n.LocalizeConfig{
			MessageID: fmt.Sprintf("ValidationErrors.%s", err.Tag()),
			TemplateData: map[string]string{
				"Field": translatedFieldName,
			},
		})
	}
	return errorMessages, len(errorMessages) == 0
}

func (p *TransferDTO) GetDestinationAccountID() (uuid.UUID, error) {
	return uuid.Parse(p.DestinationAccountID)
}

func (p *TransferDTO) GetExchangeRate() float64 {
	if p.ExchangeRate != nil {
		return *p.ExchangeRate
	}
	return 1.0 // Default to 1:1 for same currency
}

func (p *TransferDTO) GetDestinationAmount() float64 {
	if p.DestinationAmount != nil {
		return *p.DestinationAmount
	}
	// If no destination amount specified, calculate from source amount and exchange rate
	return p.Amount * p.GetExchangeRate()
}

func (p *TransferDTO) SetExchangeRate(rate float64) {
	p.ExchangeRate = &rate
	p.IsExchange = true
}

func (p *TransferDTO) SetDestinationAmount(amount float64) {
	p.DestinationAmount = &amount
}

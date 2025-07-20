package dtos

import (
	"context"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/iota-uz/go-i18n/v2/i18n"
	"github.com/iota-uz/iota-sdk/pkg/constants"
	"github.com/iota-uz/iota-sdk/pkg/intl"
	"github.com/iota-uz/iota-sdk/pkg/shared"
)

// IncomeStatementRequestDTO represents the request for generating an income statement
type IncomeStatementRequestDTO struct {
	StartDate shared.DateOnly `json:"startDate" form:"start_date" validate:"required"`
	EndDate   shared.DateOnly `json:"endDate" form:"end_date" validate:"required"`
}

// Ok validates the income statement request and returns validation errors
func (dto *IncomeStatementRequestDTO) Ok(ctx context.Context) (map[string]string, bool) {
	l, ok := intl.UseLocalizer(ctx)
	if !ok {
		panic(intl.ErrNoLocalizer)
	}

	errorMessages := map[string]string{}
	errs := constants.Validate.Struct(dto)
	if errs == nil {
		// Additional validation for date range
		if time.Time(dto.StartDate).After(time.Time(dto.EndDate)) {
			errorMessages["start_date"] = l.MustLocalize(&i18n.LocalizeConfig{
				MessageID: "validation.start_date_before_end_date",
				DefaultMessage: &i18n.Message{
					ID:    "validation.start_date_before_end_date",
					Other: "Start date must be before end date",
				},
			})
			return errorMessages, false
		}
		return errorMessages, true
	}

	for _, err := range errs.(validator.ValidationErrors) {
		translatedFieldName := l.MustLocalize(&i18n.LocalizeConfig{
			MessageID: "fields." + err.Field(),
			DefaultMessage: &i18n.Message{
				ID:    "fields." + err.Field(),
				Other: err.Field(),
			},
		})

		switch err.Tag() {
		case "required":
			errorMessages[err.Field()] = l.MustLocalize(&i18n.LocalizeConfig{
				MessageID: "validation.required",
				DefaultMessage: &i18n.Message{
					ID:    "validation.required",
					Other: "{{.Field}} is required",
				},
				TemplateData: map[string]interface{}{
					"Field": translatedFieldName,
				},
			})
		case "min":
			errorMessages[err.Field()] = l.MustLocalize(&i18n.LocalizeConfig{
				MessageID: "validation.min",
				DefaultMessage: &i18n.Message{
					ID:    "validation.min",
					Other: "{{.Field}} must be at least {{.Min}}",
				},
				TemplateData: map[string]interface{}{
					"Field": translatedFieldName,
					"Min":   err.Param(),
				},
			})
		case "max":
			errorMessages[err.Field()] = l.MustLocalize(&i18n.LocalizeConfig{
				MessageID: "validation.max",
				DefaultMessage: &i18n.Message{
					ID:    "validation.max",
					Other: "{{.Field}} must be at most {{.Max}}",
				},
				TemplateData: map[string]interface{}{
					"Field": translatedFieldName,
					"Max":   err.Param(),
				},
			})
		default:
			errorMessages[err.Field()] = l.MustLocalize(&i18n.LocalizeConfig{
				MessageID: "validation.invalid",
				DefaultMessage: &i18n.Message{
					ID:    "validation.invalid",
					Other: "{{.Field}} is invalid",
				},
				TemplateData: map[string]interface{}{
					"Field": translatedFieldName,
				},
			})
		}
	}

	return errorMessages, false
}

// IncomeStatementLineItemDTO represents a line item in the income statement response
type IncomeStatementLineItemDTO struct {
	ID                 string  `json:"id"`
	Name               string  `json:"name"`
	Amount             string  `json:"amount"`
	AmountWithCurrency string  `json:"amountWithCurrency"`
	Percentage         float64 `json:"percentage"`
}

// IncomeStatementSectionDTO represents a section of the income statement
type IncomeStatementSectionDTO struct {
	Title                string                       `json:"title"`
	LineItems            []IncomeStatementLineItemDTO `json:"lineItems"`
	Subtotal             string                       `json:"subtotal"`
	SubtotalWithCurrency string                       `json:"subtotalWithCurrency"`
	Percentage           float64                      `json:"percentage"`
}

// IncomeStatementResponseDTO represents the complete income statement response
type IncomeStatementResponseDTO struct {
	ID                      string                    `json:"id"`
	Period                  string                    `json:"period"`
	StartDate               string                    `json:"startDate"`
	EndDate                 string                    `json:"endDate"`
	RevenueSection          IncomeStatementSectionDTO `json:"revenueSection"`
	ExpenseSection          IncomeStatementSectionDTO `json:"expenseSection"`
	GrossProfit             string                    `json:"grossProfit"`
	GrossProfitWithCurrency string                    `json:"grossProfitWithCurrency"`
	GrossProfitRatio        float64                   `json:"grossProfitRatio"`
	NetProfit               string                    `json:"netProfit"`
	NetProfitWithCurrency   string                    `json:"netProfitWithCurrency"`
	NetProfitRatio          float64                   `json:"netProfitRatio"`
	IsProfit                bool                      `json:"isProfit"`
	Currency                string                    `json:"currency"`
	GeneratedAt             string                    `json:"generatedAt"`
}

package dtos

import (
	"context"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/iota-uz/go-i18n/v2/i18n"
	"github.com/iota-uz/iota-sdk/pkg/constants"
	"github.com/iota-uz/iota-sdk/pkg/intl"
	"github.com/iota-uz/iota-sdk/pkg/shared"
)

// CashflowStatementRequestDTO represents the request for generating a cashflow statement
type CashflowStatementRequestDTO struct {
	AccountID uuid.UUID       `json:"accountId" form:"account_id" validate:"required"`
	StartDate shared.DateOnly `json:"startDate" form:"start_date" validate:"required"`
	EndDate   shared.DateOnly `json:"endDate" form:"end_date" validate:"required"`
}

// Ok validates the cashflow statement request and returns validation errors
func (dto *CashflowStatementRequestDTO) Ok(ctx context.Context) (map[string]string, bool) {
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

// CashflowLineItemDTO represents a line item in the cashflow statement
type CashflowLineItemDTO struct {
	ID                 string  `json:"id"`
	Name               string  `json:"name"`
	Amount             string  `json:"amount"`
	AmountWithCurrency string  `json:"amountWithCurrency"`
	Percentage         float64 `json:"percentage"`
	Count              int     `json:"count"`
}

// CashflowSectionDTO represents a section of the cashflow statement
type CashflowSectionDTO struct {
	Name                    string                `json:"name"`
	Inflows                 []CashflowLineItemDTO `json:"inflows"`
	Outflows                []CashflowLineItemDTO `json:"outflows"`
	NetCashFlow             string                `json:"netCashFlow"`
	NetCashFlowWithCurrency string                `json:"netCashFlowWithCurrency"`
}

// CashflowStatementResponseDTO represents the complete cashflow statement response
type CashflowStatementResponseDTO struct {
	ID                          string             `json:"id"`
	AccountID                   string             `json:"accountId"`
	AccountName                 string             `json:"accountName"`
	Period                      string             `json:"period"`
	StartDate                   string             `json:"startDate"`
	EndDate                     string             `json:"endDate"`
	StartingBalance             string             `json:"startingBalance"`
	StartingBalanceWithCurrency string             `json:"startingBalanceWithCurrency"`
	EndingBalance               string             `json:"endingBalance"`
	EndingBalanceWithCurrency   string             `json:"endingBalanceWithCurrency"`
	OperatingActivities         CashflowSectionDTO `json:"operatingActivities"`
	InvestingActivities         CashflowSectionDTO `json:"investingActivities"`
	FinancingActivities         CashflowSectionDTO `json:"financingActivities"`
	TotalInflows                string             `json:"totalInflows"`
	TotalInflowsWithCurrency    string             `json:"totalInflowsWithCurrency"`
	TotalOutflows               string             `json:"totalOutflows"`
	TotalOutflowsWithCurrency   string             `json:"totalOutflowsWithCurrency"`
	NetCashFlow                 string             `json:"netCashFlow"`
	NetCashFlowWithCurrency     string             `json:"netCashFlowWithCurrency"`
	IsPositive                  bool               `json:"isPositive"`
	Currency                    string             `json:"currency"`
	GeneratedAt                 string             `json:"generatedAt"`
}

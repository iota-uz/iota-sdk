package dtos

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/go-i18n/v2/i18n"
	"github.com/iota-uz/iota-sdk/modules/projects/domain/aggregates/acceptance"
	"github.com/iota-uz/iota-sdk/pkg/constants"
	"github.com/iota-uz/iota-sdk/pkg/intl"
	"github.com/iota-uz/iota-sdk/pkg/money"
	"github.com/iota-uz/iota-sdk/pkg/shared"
)

type AcceptanceCreateDTO struct {
	Kind         string `validate:"required,oneof=ACT DELIVERY_NOTE OTHER"`
	Number       string `validate:"required,max=64"`
	Date         shared.DateOnly
	Amount       shared.Amount
	CurrencyCode string `validate:"required,len=3"`
	Description  string
}

func (dto *AcceptanceCreateDTO) Ok(ctx context.Context) (map[string]string, bool) {
	l, ok := intl.UseLocalizer(ctx)
	if !ok {
		panic(intl.ErrNoLocalizer)
	}
	errorMessages := validationErrors(l, "Projects.Acceptance", constants.Validate.Struct(dto))
	switch {
	case dto.Amount.Err() != nil:
		errorMessages["Amount"] = l.MustLocalize(&i18n.LocalizeConfig{MessageID: "Projects.Errors.InvalidAmount"})
	case !dto.Amount.IsPositive():
		errorMessages["Amount"] = fieldError(l, "Projects.Acceptance", "Amount", "gt")
	}
	if time.Time(dto.Date).IsZero() {
		errorMessages["Date"] = fieldError(l, "Projects.Acceptance", "Date", "required")
	}
	return errorMessages, len(errorMessages) == 0
}

func (dto *AcceptanceCreateDTO) ToEntity(tenantID, projectID uuid.UUID) acceptance.Document {
	return acceptance.New(
		projectID,
		acceptance.Kind(dto.Kind),
		dto.Number,
		time.Time(dto.Date),
		money.NewFromFloat(dto.Amount.Float64(), dto.CurrencyCode),
		acceptance.WithTenantID(tenantID),
		acceptance.WithDescription(dto.Description),
	)
}

package dtos

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/go-i18n/v2/i18n"
	"github.com/iota-uz/iota-sdk/modules/projects/domain/aggregates/acceptance"
	"github.com/iota-uz/iota-sdk/pkg/constants"
	"github.com/iota-uz/iota-sdk/pkg/intl"
	"github.com/iota-uz/iota-sdk/pkg/money"
	"github.com/iota-uz/iota-sdk/pkg/shared"
)

const acceptanceFields = "Projects.Acceptance"

// AcceptanceCreateDTO is the form of an acceptance document, for a new
// document and for an edited one. SignedOn is optional: a day signs the
// document, no day leaves it a draft that does not count as revenue.
type AcceptanceCreateDTO struct {
	Kind         string `validate:"required,max=32"`
	Number       string `validate:"required,max=64"`
	Date         shared.DateOnly
	SignedOn     shared.DateOnly
	Amount       shared.Amount
	CurrencyCode string `validate:"required,len=3"`
	Description  string
}

// Ok validates the form against the built-in kinds.
func (dto *AcceptanceCreateDTO) Ok(ctx context.Context) (map[string]string, bool) {
	return dto.Check(ctx, nil)
}

// Check validates the form; kinds are the kinds the form offers, and none
// means the built-in ones.
func (dto *AcceptanceCreateDTO) Check(ctx context.Context, kinds []acceptance.Kind) (map[string]string, bool) {
	l, ok := intl.UseLocalizer(ctx)
	if !ok {
		panic(intl.ErrNoLocalizer)
	}
	dto.Number = strings.TrimSpace(dto.Number)
	errorMessages := validationErrors(l, acceptanceFields, constants.Validate.Struct(dto))
	if _, failed := errorMessages["Kind"]; !failed && !dto.kindAllowed(kinds) {
		errorMessages["Kind"] = fieldError(l, acceptanceFields, "Kind", "oneof")
	}
	switch {
	case dto.Amount.Err() != nil:
		errorMessages["Amount"] = l.MustLocalize(&i18n.LocalizeConfig{MessageID: "Projects.Errors.InvalidAmount"})
	case !dto.Amount.IsPositive():
		errorMessages["Amount"] = fieldError(l, acceptanceFields, "Amount", "gt")
	}
	if time.Time(dto.Date).IsZero() {
		errorMessages["Date"] = fieldError(l, acceptanceFields, "Date", "required")
	}
	return errorMessages, len(errorMessages) == 0
}

func (dto *AcceptanceCreateDTO) kindAllowed(kinds []acceptance.Kind) bool {
	if len(kinds) == 0 {
		for _, option := range acceptance.DefaultKinds() {
			kinds = append(kinds, option.Value)
		}
	}
	for _, kind := range kinds {
		if string(kind) == dto.Kind {
			return true
		}
	}
	return false
}

// ToEntity builds the new document; a signing day makes it signed.
func (dto *AcceptanceCreateDTO) ToEntity(tenantID, projectID uuid.UUID) acceptance.Document {
	return acceptance.New(
		projectID,
		acceptance.Kind(dto.Kind),
		dto.Number,
		time.Time(dto.Date),
		dto.money(),
		acceptance.WithTenantID(tenantID),
		acceptance.WithSignedOn(time.Time(dto.SignedOn)),
		acceptance.WithDescription(dto.Description),
	)
}

// ToRevision is what an edit changes in an existing document.
func (dto *AcceptanceCreateDTO) ToRevision() acceptance.Revision {
	return acceptance.Revision{
		Kind:        acceptance.Kind(dto.Kind),
		Number:      dto.Number,
		Date:        time.Time(dto.Date),
		SignedOn:    time.Time(dto.SignedOn),
		Amount:      dto.money(),
		Description: dto.Description,
	}
}

// money is the exact amount typed into the form in the minor units of the
// currency; it never passes through float64, which would round a long amount.
func (dto *AcceptanceCreateDTO) money() *money.Money {
	fraction := money.New(0, dto.CurrencyCode).Currency().Fraction
	return money.NewFromBigInt(dto.Amount.Decimal().Shift(int32(fraction)).BigInt(), dto.CurrencyCode)
}

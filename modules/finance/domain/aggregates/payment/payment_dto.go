package payment

import (
	"time"

	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	moneyaccount "github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/money_account"
	"github.com/iota-uz/iota-sdk/pkg/shared"
)

var validate = validator.New(validator.WithRequiredStructEnabled())

type CreateDTO struct {
	Amount           float64         `validate:"required,gt=0"`
	AccountID        uint            `validate:"required"`
	TransactionDate  shared.DateOnly `validate:"required"`
	AccountingPeriod shared.DateOnly `validate:"required"`
	CounterpartyID   uint            `validate:"required"`
	UserID           uint            `validate:"required"`
	Comment          string
}

type UpdateDTO struct {
	Amount           float64 `validate:"gt=0"`
	AccountID        uint
	CounterpartyID   uint
	TransactionDate  shared.DateOnly
	AccountingPeriod shared.DateOnly
	Comment          string
	UserID           uint
}

func (p *CreateDTO) Ok(l ut.Translator) (map[string]string, bool) {
	errors := map[string]string{}
	err := validate.Struct(p)
	if err == nil {
		return errors, true
	}
	for _, _err := range err.(validator.ValidationErrors) {
		errors[_err.Field()] = _err.Translate(l)
	}
	return errors, len(errors) == 0
}

func (p *CreateDTO) ToEntity() Payment {
	email, err := internet.NewEmail("payment@system.internal")
	if err != nil {
		// This should never happen with a hardcoded valid email
		panic(err)
	}

	return New(
		p.Amount,
		0,
		p.CounterpartyID,
		p.Comment,
		&moneyaccount.Account{ID: p.AccountID},
		user.New(
			"", // firstName
			"", // lastName
			email,
			"", // uiLanguage
			user.WithID(p.UserID),
		),
		time.Time(p.TransactionDate),
		time.Time(p.AccountingPeriod),
	)
}

func (p *UpdateDTO) Ok(l ut.Translator) (map[string]string, bool) {
	errors := map[string]string{}
	errs := validate.Struct(p)
	if errs == nil {
		return errors, true
	}
	for _, err := range errs.(validator.ValidationErrors) {
		errors[err.Field()] = err.Translate(l)
	}
	return errors, len(errors) == 0
}

func (p *UpdateDTO) ToEntity(id uint) Payment {
	email, err := internet.NewEmail("payment@system.internal")
	if err != nil {
		// This should never happen with a hardcoded valid email
		panic(err)
	}

	return NewWithID(
		id,
		p.Amount,
		0,
		p.CounterpartyID,
		p.Comment,
		&moneyaccount.Account{ID: p.AccountID},
		user.New(
			"", // firstName
			"", // lastName
			email,
			"", // uiLanguage
			user.WithID(p.UserID),
		),
		time.Time(p.TransactionDate),
		time.Time(p.AccountingPeriod),
		time.Now(),
		time.Now(),
	)
}

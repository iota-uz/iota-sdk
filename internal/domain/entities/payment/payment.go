package payment

import (
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/iota-agency/iota-erp/internal/domain/entities/user"
	model "github.com/iota-agency/iota-erp/internal/interfaces/graph/gqlmodels"
	"github.com/nicksnyder/go-i18n/v2/i18n"
)

var validate = validator.New(validator.WithRequiredStructEnabled())

type Payment struct {
	Id               uint
	StageId          uint
	Amount           float64
	CurrencyCode     string
	AccountId        uint
	TransactionId    uint
	TransactionDate  time.Time
	AccountingPeriod time.Time
	Comment          string
	User             *user.User
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type CreateDTO struct {
	Amount           float64   `validate:"required,gt=0"`
	CurrencyCode     string    `validate:"required,len=3"`
	AccountId        uint      `validate:"required"`
	TransactionDate  time.Time `validate:"required"`
	AccountingPeriod time.Time `validate:"required"`
	Comment          string
	UserId           uint `validate:"required"`
	StageId          uint `validate:"required"`
}

type UpdateDTO struct {
	Amount           float64 `validate:"gt=0"`
	CurrencyCode     string  `validate:"len=3"`
	AccountID        uint
	TransactionDate  time.Time
	AccountingPeriod time.Time
	Comment          string
	UserId           uint
	StageId          uint
}

// TODO: translate error messages

func (p *CreateDTO) Ok(l *i18n.Localizer) (map[string]string, bool) {
	errors := map[string]string{}
	err := validate.Struct(p)
	if err == nil {
		return errors, true
	}
	for _, _err := range err.(validator.ValidationErrors) {
		errors[_err.Field()] = _err.Error()
	}
	return errors, len(errors) == 0
}

func (p *CreateDTO) ToEntity() *Payment {
	return &Payment{
		StageId:          p.StageId,
		Amount:           p.Amount,
		CurrencyCode:     p.CurrencyCode,
		AccountId:        p.AccountId,
		TransactionDate:  p.TransactionDate,
		AccountingPeriod: p.AccountingPeriod,
		Comment:          p.Comment,
	}
}

func (p *Payment) Ok(l *i18n.Localizer) (map[string]string, bool) {
	errors := map[string]string{}
	err := validate.Struct(p)
	if err == nil {
		return errors, true
	}
	for _, _err := range err.(validator.ValidationErrors) {
		errors[_err.Field()] = _err.Error()
	}
	return errors, len(errors) == 0
}

func (p *UpdateDTO) Ok(l *i18n.Localizer) (map[string]string, bool) {
	errors := map[string]string{}
	err := validate.Struct(p)
	if err == nil {
		return errors, true
	}
	for _, _err := range err.(validator.ValidationErrors) {
		errors[_err.Field()] = _err.Error()
	}
	return errors, len(errors) == 0
}

func (p *UpdateDTO) ToEntity(id uint) *Payment {
	return &Payment{
		Id:               id,
		StageId:          p.StageId,
		Amount:           p.Amount,
		CurrencyCode:     p.CurrencyCode,
		AccountId:        p.AccountID,
		TransactionDate:  p.TransactionDate,
		AccountingPeriod: p.AccountingPeriod,
		Comment:          p.Comment,
	}
}

func (p *Payment) ToGraph() *model.Payment {
	return &model.Payment{
		ID:        int64(p.Id),
		StageID:   int64(p.StageId),
		CreatedAt: p.CreatedAt,
		UpdatedAt: p.UpdatedAt,
	}
}

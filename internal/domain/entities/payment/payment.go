package payment

import (
	"github.com/iota-agency/iota-erp/internal/domain/entities/user"
	"time"

	model "github.com/iota-agency/iota-erp/internal/interfaces/graph/gqlmodels"
	"github.com/nicksnyder/go-i18n/v2/i18n"
)

type Payment struct {
	Id               uint
	StageId          uint
	Amount           float64
	AmountCurrencyID string
	MoneyAccountID   uint
	TransactionDate  time.Time
	AccountingPeriod time.Time
	Comment          string
	User             *user.User
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type CreateDTO struct {
	Amount           float64   `schema:"amount,required"`
	AmountCurrencyID string    `schema:"amount_currency_id,required"`
	MoneyAccountID   uint      `schema:"money_account_id,required"`
	TransactionDate  time.Time `schema:"transaction_date,required"`
	AccountingPeriod time.Time `schema:"accounting_period,required"`
	Comment          string    `schema:"comment"`
	UserId           uint      `schema:"user_id,required"`
	StageId          uint      `schema:"stage_id,required"`
}

type UpdateDTO struct {
	Amount           float64   `schema:"amount"`
	AmountCurrencyID string    `schema:"amount_currency_id"`
	MoneyAccountID   uint      `schema:"money_account_id"`
	TransactionDate  time.Time `schema:"transaction_date"`
	AccountingPeriod time.Time `schema:"accounting_period"`
	Comment          string    `schema:"comment"`
	UserId           uint      `schema:"user_id"`
	StageId          uint      `schema:"stage_id"`
}

func (p *CreateDTO) Ok(l *i18n.Localizer) (map[string]string, bool) {
	errors := map[string]string{}
	if p.Amount <= 0 {
		errors["amount"] = l.MustLocalize(&i18n.LocalizeConfig{MessageID: "Validations.PositiveAmount"})
	}
	return errors, len(errors) == 0
}

func (p *CreateDTO) ToEntity() *Payment {
	return &Payment{
		StageId:          p.StageId,
		Amount:           p.Amount,
		AmountCurrencyID: p.AmountCurrencyID,
		MoneyAccountID:   p.MoneyAccountID,
		TransactionDate:  p.TransactionDate,
		AccountingPeriod: p.AccountingPeriod,
		Comment:          p.Comment,
	}
}

func (p *Payment) Ok(l *i18n.Localizer) (map[string]string, bool) {
	errors := map[string]string{}
	if p.Amount <= 0 {
		errors["amount"] = l.MustLocalize(&i18n.LocalizeConfig{MessageID: "Validations.PositiveAmount"})
	}
	return errors, len(errors) == 0
}

func (p *UpdateDTO) Ok(l *i18n.Localizer) (map[string]string, bool) {
	errors := map[string]string{}
	if p.Amount <= 0 {
		errors["amount"] = l.MustLocalize(&i18n.LocalizeConfig{MessageID: "Validations.PositiveAmount"})
	}
	return errors, len(errors) == 0
}

func (p *UpdateDTO) ToEntity(id uint) *Payment {
	return &Payment{
		Id:               id,
		StageId:          p.StageId,
		Amount:           p.Amount,
		AmountCurrencyID: p.AmountCurrencyID,
		MoneyAccountID:   p.MoneyAccountID,
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

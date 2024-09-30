package payment

import (
	"github.com/iota-agency/iota-erp/internal/domain/entities/transaction"
	"github.com/iota-agency/iota-erp/internal/domain/entities/user"
	"time"

	model "github.com/iota-agency/iota-erp/internal/interfaces/graph/gqlmodels"
	"github.com/nicksnyder/go-i18n/v2/i18n"
)

type Payment struct {
	Id          uint
	StageId     int64
	Amount      float64
	Comment     string
	Transaction *transaction.Transaction
	User        *user.User
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type PaymentUpdate struct {
	Amount      float64
	Description string
}

func (p *Payment) Ok(l *i18n.Localizer) (map[string]string, bool) {
	errors := map[string]string{}
	if p.Amount <= 0 {
		errors["amount"] = l.MustLocalize(&i18n.LocalizeConfig{MessageID: "Validations.PositiveAmount"})
	}
	return errors, len(errors) == 0
}

func (p *PaymentUpdate) Ok(l *i18n.Localizer) (map[string]string, bool) {
	errors := map[string]string{}
	if p.Amount <= 0 {
		errors["amount"] = l.MustLocalize(&i18n.LocalizeConfig{MessageID: "Validations.PositiveAmount"})
	}
	return errors, len(errors) == 0
}

func (p *Payment) ToGraph() *model.Payment {
	return &model.Payment{
		ID:        int64(p.Id),
		CreatedAt: p.CreatedAt,
		UpdatedAt: p.UpdatedAt,
	}
}

package order

import (
	"fmt"
	"github.com/iota-agency/iota-sdk/modules/warehouse/domain/aggregates/product"
	"github.com/iota-agency/iota-sdk/pkg/serrors"
	"github.com/nicksnyder/go-i18n/v2/i18n"
)

type ErrOrderIsComplete struct {
	serrors.Base
	Current Status
}

func NewErrOrderIsComplete(current Status) *ErrOrderIsComplete {
	return &ErrOrderIsComplete{
		Base: serrors.Base{
			Code:    "ERR_ORDER_IS_ALREADY_COMPLETED",
			Message: "order is already complete",
		},
		Current: current,
	}
}

func (e *ErrOrderIsComplete) Localize(l *i18n.Localizer) string {
	return l.MustLocalize(&i18n.LocalizeConfig{
		DefaultMessage: &i18n.Message{
			ID: fmt.Sprintf("Errors.%s", e.Code),
		},
		TemplateData: map[string]interface{}{
			"Current": e.Current,
		},
	})
}

type ErrProductIsShipped struct {
	serrors.Base
	Current product.Status
}

func NewErrProductIsShipped(current product.Status) *ErrProductIsShipped {
	return &ErrProductIsShipped{
		Base: serrors.Base{
			Code:    "ERR_PRODUCT_IS_SHIPPED",
			Message: "product is already shipped",
		},
		Current: current,
	}
}

func (e *ErrProductIsShipped) Localize(l *i18n.Localizer) string {
	return l.MustLocalize(&i18n.LocalizeConfig{
		DefaultMessage: &i18n.Message{
			ID: fmt.Sprintf("Errors.%s", e.Code),
		},
		TemplateData: map[string]interface{}{
			"Current": e.Current,
		},
	})
}

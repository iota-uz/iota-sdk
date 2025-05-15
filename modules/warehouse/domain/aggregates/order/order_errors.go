package order

import (
	"github.com/iota-uz/go-i18n/v2/i18n"
	"github.com/iota-uz/iota-sdk/modules/warehouse/domain/aggregates/product"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

type OrderIsCompleteError struct {
	serrors.BaseError
	Current Status
}

func NewErrOrderIsComplete(current Status) *OrderIsCompleteError {
	return &OrderIsCompleteError{
		BaseError: serrors.BaseError{
			Code:    "ERR_ORDER_IS_ALREADY_COMPLETED",
			Message: "order is already complete",
		},
		Current: current,
	}
}

func (e *OrderIsCompleteError) Localize(l *i18n.Localizer) string {
	return l.MustLocalize(&i18n.LocalizeConfig{ //nolint:exhaustruct
		DefaultMessage: &i18n.Message{ //nolint:exhaustruct
			ID: "Errors." + e.Code,
		},
		TemplateData: map[string]interface{}{
			"Current": e.Current,
		},
	})
}

type ProductIsShippedError struct {
	serrors.BaseError
	Current product.Status
}

func NewErrProductIsShipped(current product.Status) *ProductIsShippedError {
	return &ProductIsShippedError{
		BaseError: serrors.BaseError{
			Code:    "ERR_PRODUCT_IS_SHIPPED",
			Message: "product is already shipped",
		},
		Current: current,
	}
}

func (e *ProductIsShippedError) Localize(l *i18n.Localizer) string {
	return l.MustLocalize(&i18n.LocalizeConfig{ //nolint:exhaustruct
		DefaultMessage: &i18n.Message{ //nolint:exhaustruct
			ID: "Errors." + e.Code,
		},
		TemplateData: map[string]interface{}{
			"Current": e.Current,
		},
	})
}

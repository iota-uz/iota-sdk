package order

import (
	"fmt"
	"github.com/iota-uz/go-i18n/v2/i18n"
	"github.com/iota-uz/iota-sdk/modules/warehouse/domain/aggregates/product"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

type OrderIsCompleteError struct {
	detail  *serrors.Error
	Current Status
}

func NewErrOrderIsComplete(current Status) *OrderIsCompleteError {
	return &OrderIsCompleteError{detail: serrors.NewFailedPrecondition("order is already complete").WithReason("ERR_ORDER_IS_ALREADY_COMPLETED").WithPublic(serrors.Message{ID: "Errors.ERR_ORDER_IS_ALREADY_COMPLETED", Args: map[string]serrors.Value{"Current": serrors.Text(fmt.Sprint(current))}}), Current: current}
}
func (e *OrderIsCompleteError) Unwrap() error { return e.detail }
func (e *OrderIsCompleteError) Localize(l *i18n.Localizer) string {
	return serrors.Public(e, l).Message
}

type ProductIsShippedError struct {
	detail  *serrors.Error
	Current product.Status
}

func NewErrProductIsShipped(current product.Status) *ProductIsShippedError {
	return &ProductIsShippedError{detail: serrors.NewFailedPrecondition("product is already shipped").WithReason("ERR_PRODUCT_IS_SHIPPED").WithPublic(serrors.Message{ID: "Errors.ERR_PRODUCT_IS_SHIPPED", Args: map[string]serrors.Value{"Current": serrors.Text(fmt.Sprint(current))}}), Current: current}
}
func (e *ProductIsShippedError) Unwrap() error { return e.detail }
func (e *ProductIsShippedError) Localize(l *i18n.Localizer) string {
	return serrors.Public(e, l).Message
}
func (e *OrderIsCompleteError) Error() string  { return e.detail.Error() }
func (e *ProductIsShippedError) Error() string { return e.detail.Error() }

package money

import "github.com/iota-uz/iota-sdk/modules/core/domain/entities/currency"

type Amount interface {
	Value() float64
	Currency() currency.Code
}

package money

import "github.com/iota-uz/iota-sdk/modules/core/domain/entities/currency"

func New(v float64, code currency.Code) Amount {
	return &amount{
		value: v,
		code:  code,
	}
}

type amount struct {
	value float64
	code  currency.Code
}

func (a *amount) Value() float64 {
	return a.value
}

func (a *amount) Currency() currency.Code {
	return a.code
}

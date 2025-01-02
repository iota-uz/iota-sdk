package currency

import (
	"fmt"
	"slices"
)

// TODO: make this private
type Code string

const (
	UsdCode Code = "USD"
	EurCode Code = "EUR"
	GbpCode Code = "GBP"
	AudCode Code = "AUD"
	CadCode Code = "CAD"
	ChfCode Code = "CHF"
	CnyCode Code = "CNY"
	JpyCode Code = "JPY"
	RubCode Code = "RUB"
	TryCode Code = "TRY"
	SomCode Code = "SOM"
)

func (s Code) IsValid() bool {
	return slices.Contains(ValidCodes, s)
}

func NewCode(value string) (Code, error) {
	c := Code(value)
	if !c.IsValid() {
		return c, fmt.Errorf("invalid currency code: %s", value)
	}
	return c, nil
}

// TODO: make this private
type Symbol string

const (
	UsdSymbol Symbol = "$"
	EurSymbol Symbol = "€"
	GbpSymbol Symbol = "£"
	AudSymbol Symbol = "A$"
	CadSymbol Symbol = "C$"
	ChfSymbol Symbol = "CHF"
	CnySymbol Symbol = "¥"
	JpySymbol Symbol = "¥"
	RubSymbol Symbol = "₽"
	TrySymbol Symbol = "₺"
	SomSymbol Symbol = "S"
)

func (s Symbol) IsValid() bool {
	return slices.Contains(ValidSymbols, s)
}

func NewSymbol(value string) (Symbol, error) {
	s := Symbol(value)
	if !s.IsValid() {
		return s, fmt.Errorf("invalid currency symbol: %s", value)
	}
	return s, nil
}

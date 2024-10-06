package currency

import (
	"fmt"
	"slices"
)

type CodeEnum string

const (
	UsdCode CodeEnum = "USD"
	EurCode CodeEnum = "EUR"
	GbpCode CodeEnum = "GBP"
	AudCode CodeEnum = "AUD"
	CadCode CodeEnum = "CAD"
	ChfCode CodeEnum = "CHF"
	CnyCode CodeEnum = "CNY"
	JpyCode CodeEnum = "JPY"
	RubCode CodeEnum = "RUB"
	TryCode CodeEnum = "TRY"
	SomCode CodeEnum = "SOM"
)

func (s CodeEnum) IsValid() bool {
	return slices.Contains(ValidCodes, s)
}

func NewCode(value string) (Code, error) {
	c := CodeEnum(value)
	if !c.IsValid() {
		return Code{}, fmt.Errorf("invalid currency code: %s", value)
	}
	return Code{value: c}, nil
}

type Code struct {
	value CodeEnum
}

func (p Code) Get() CodeEnum {
	return p.value
}

func (p Code) String() string {
	return string(p.value)
}

type SymbolEnum string

const (
	UsdSymbol SymbolEnum = "$"
	EurSymbol SymbolEnum = "€"
	GbpSymbol SymbolEnum = "£"
	AudSymbol SymbolEnum = "A$"
	CadSymbol SymbolEnum = "C$"
	ChfSymbol SymbolEnum = "CHF"
	CnySymbol SymbolEnum = "¥"
	JpySymbol SymbolEnum = "¥"
	RubSymbol SymbolEnum = "₽"
	TrySymbol SymbolEnum = "₺"
	SomSymbol SymbolEnum = "S"
)

func (s SymbolEnum) IsValid() bool {
	return slices.Contains(ValidSymbols, s)
}

func NewSymbol(value string) (Symbol, error) {
	s := SymbolEnum(value)
	if !s.IsValid() {
		return Symbol{}, fmt.Errorf("invalid currency symbol: %s", value)
	}
	return Symbol{value: s}, nil
}

type Symbol struct {
	value SymbolEnum
}

func (p Symbol) Get() SymbolEnum {
	return p.value
}

func (p Symbol) String() string {
	return string(p.value)
}

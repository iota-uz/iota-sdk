package currency

import (
	"slices"
)

type CodeEnum string

const (
	UsdCode CodeEnum = "USD"
	EurCode          = "EUR"
	GbpCode          = "GBP"
	AudCode          = "AUD"
	CadCode          = "CAD"
	ChfCode          = "CHF"
	CnyCode          = "CNY"
	JpyCode          = "JPY"
	RubCode          = "RUB"
	TryCode          = "TRY"
	SomCode          = "SOM"
)

func (s CodeEnum) IsValid() bool {
	return slices.Contains(ValidCodes, s)
}

func NewCode(value string) (Code, error) {
	c := CodeEnum(value)
	if !c.IsValid() {
		return Code{}, InvalidCode
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
	EurSymbol            = "€"
	GbpSymbol            = "£"
	AudSymbol            = "A$"
	CadSymbol            = "C$"
	ChfSymbol            = "CHF"
	CnySymbol            = "¥"
	JpySymbol            = "¥"
	RubSymbol            = "₽"
	TrySymbol            = "₺"
	SomSymbol            = "S"
)

func (s SymbolEnum) IsValid() bool {
	return slices.Contains(ValidSymbols, s)
}

func NewSymbol(value string) (Symbol, error) {
	s := SymbolEnum(value)
	if !s.IsValid() {
		return Symbol{}, InvalidSymbol
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

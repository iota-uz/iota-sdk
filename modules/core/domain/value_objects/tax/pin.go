package tax

import (
	"errors"
	"strings"

	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/country"
	"github.com/iota-uz/utils/sequence"
)

var (
	ErrInvalidPin = errors.New("invalid PIN")
)

var (
	NilPin Pin = &pin{
		v:       "",
		country: country.NilCountry,
	}
)

func NewPin(v string, c country.Country) (Pin, error) {
	if !IsValidPin(v, c) {
		return nil, ErrInvalidPin
	}
	return pin{v: v, country: c}, nil
}

type pin struct {
	v       string
	country country.Country
}

func (p pin) Value() string {
	return p.v
}

func (p pin) Country() country.Country {
	return p.country
}

func IsValidPin(v string, c country.Country) bool {
	v = strings.Trim(v, " ")
	// TODO: Implement PIN validation for other countries
	switch c {
	case country.Uzbekistan:
		return len(v) == 14 && sequence.IsNumeric(v)
	case country.Kazakhstan:
		return len(v) == 12 && sequence.IsNumeric(v)
	}
	return true
}

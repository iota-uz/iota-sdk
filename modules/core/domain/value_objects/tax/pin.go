package tax

import (
	"errors"
	country2 "github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/country"
	"github.com/iota-uz/utils/sequence"
	"strings"
)

var (
	ErrInvalidPin = errors.New("invalid PIN")
)

var (
	NilPin Pin = &pin{
		v:       "",
		country: country2.NilCountry,
	}
)

func NewPin(v string, c country2.Country) (Pin, error) {
	if !IsValidPin(v, c) {
		return nil, ErrInvalidPin
	}
	return pin{v: v, country: c}, nil
}

type pin struct {
	v       string
	country country2.Country
}

func (p pin) Value() string {
	return p.v
}

func (p pin) Country() country2.Country {
	return p.country
}

func IsValidPin(v string, c country2.Country) bool {
	v = strings.Trim(v, " ")
	// TODO: Implement PIN validation for other countries
	switch c {
	case country2.Uzbekistan:
		return len(v) == 14 && sequence.IsNumeric(v)
	case country2.Kazakhstan:
		return len(v) == 12 && sequence.IsNumeric(v)
	}
	return false
}

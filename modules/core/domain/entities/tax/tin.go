package tax

import (
	"errors"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/country"
	"github.com/iota-uz/iota-sdk/pkg/utils/sequence"
	"strings"
)

var (
	ErrInvalidTin = errors.New("invalid TIN")
)

var (
	NilTin Tin = &tin{
		v:      "",
		county: country.NilCountry,
	}
)

func NewTin(t string, c country.Country) (Tin, error) {
	if !IsValidTin(t, c) {
		return nil, ErrInvalidTin
	}
	return tin{v: t, county: c}, nil
}

type tin struct {
	v      string
	county country.Country
}

func (t tin) Country() country.Country {
	return t.county
}

func (t tin) Value() string {
	return t.v
}

func IsValidTin(t string, c country.Country) bool {
	t = strings.Trim(t, " ")
	// TODO: Implement TIN validation for other countries
	switch c {
	case country.Uzbekistan:
		return len(t) == 9 && sequence.IsNumeric(t)
	case country.Kazakhstan:
		return len(t) == 12 && sequence.IsNumeric(t)
	}
	return false
}

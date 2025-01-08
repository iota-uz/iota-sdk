package tax

import (
	"github.com/go-faster/errors"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/country"
	"github.com/iota-uz/utils/sequence"
	"strings"
)

var (
	ErrInvalidTin = errors.New("invalid Tin")
)

var (
	NilTin Tin = &tin{
		v:      "",
		county: country.NilCountry,
	}
)

func NewTin(t string, c country.Country) (Tin, error) {
	if err := ValidateTin(t, c); err != nil {
		return nil, err
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

func ValidateTin(t string, c country.Country) error {
	t = strings.Trim(t, " ")
	// TODO: Implement Tin validation for other countries
	switch c {
	case country.Uzbekistan:
		if !sequence.IsNumeric(t) {
			return errors.Wrap(ErrInvalidTin, "Uzbekistan Tin must be numeric")
		}
		if len(t) != 9 {
			return errors.Wrap(ErrInvalidTin, "Uzbekistan Tin length must be 9")
		}
		return nil
	case country.Kazakhstan:
		if !sequence.IsNumeric(t) {
			return errors.Wrap(ErrInvalidTin, "Kazakhstan Tin must be numeric")
		}
		if len(t) != 12 {
			return errors.Wrap(ErrInvalidTin, "Kazakhstan Tin length must be 12")
		}
	}
	return nil
}

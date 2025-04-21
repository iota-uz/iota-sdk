package tax

import (
	"strings"

	"github.com/go-faster/errors"
	country2 "github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/country"
	"github.com/iota-uz/utils/sequence"
)

var (
	ErrInvalidTin = errors.New("invalid Tin")
)

var (
	NilTin Tin = &tin{
		v:      "",
		county: country2.NilCountry,
	}
)

func NewTin(t string, c country2.Country) (Tin, error) {
	if err := ValidateTin(t, c); err != nil {
		return nil, err
	}
	return tin{v: t, county: c}, nil
}

type tin struct {
	v      string
	county country2.Country
}

func (t tin) Country() country2.Country {
	return t.county
}

func (t tin) Value() string {
	return t.v
}

func ValidateTin(t string, c country2.Country) error {
	t = strings.Trim(t, " ")
	// TODO: Implement Tin validation for other countries
	switch c {
	case country2.Uzbekistan:
		if !sequence.IsNumeric(t) {
			return errors.Wrap(ErrInvalidTin, "Uzbekistan Tin must be numeric")
		}
		if len(t) != 9 {
			return errors.Wrap(ErrInvalidTin, "Uzbekistan Tin length must be 9")
		}
		return nil
	case country2.Kazakhstan:
		if !sequence.IsNumeric(t) {
			return errors.Wrap(ErrInvalidTin, "Kazakhstan Tin must be numeric")
		}
		if len(t) != 12 {
			return errors.Wrap(ErrInvalidTin, "Kazakhstan Tin length must be 12")
		}
	}
	return nil
}

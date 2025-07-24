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

// Specific TIN validation error types for better error handling
type TinValidationError struct {
	Country country2.Country
	Reason  string
	Message string
}

func (e *TinValidationError) Error() string {
	return e.Message
}

func newTinValidationError(country country2.Country, reason, message string) *TinValidationError {
	return &TinValidationError{
		Country: country,
		Reason:  reason,
		Message: message,
	}
}

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
			return newTinValidationError(c, "non_numeric", "TIN must contain only numbers")
		}
		if len(t) != 9 {
			return newTinValidationError(c, "invalid_length", "TIN must be exactly 9 digits")
		}
		return nil
	case country2.Kazakhstan:
		if !sequence.IsNumeric(t) {
			return newTinValidationError(c, "non_numeric", "TIN must contain only numbers")
		}
		if len(t) != 12 {
			return newTinValidationError(c, "invalid_length", "TIN must be exactly 12 digits")
		}
	}
	return nil
}

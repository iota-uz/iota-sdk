package phone

import (
	"errors"
	"unicode"

	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/country"
)

var (
	ErrInvalidPhoneNumber = errors.New("invalid phone number")
)

type Phone interface {
	Value() string
}

func Strip(v string) string {
	result := ""
	for _, char := range v {
		if unicode.IsDigit(char) {
			result += string(char)
		}
	}
	return result
}

func New(v string, c country.Country) (Phone, error) {
	if v == "" {
		return phone(""), ErrInvalidPhoneNumber
	}
	stripped := Strip(v)
	if !IsValidPhoneNumber(stripped, c) {
		return phone(""), ErrInvalidPhoneNumber
	}
	return phone(v), nil
}

type phone string

func (p phone) Value() string {
	return string(p)
}

func IsValidUSPhoneNumber(v string) bool {
	if len(v) == 11 && v[0] == '1' {
		v = v[1:] // Remove country code
	}

	if len(v) != 10 {
		return false
	}

	areaCode := v[:3]
	exchangeCode := v[3:6]

	// Area code and exchange code must start with 2-9
	if areaCode[0] < '2' || areaCode[0] > '9' {
		return false
	}

	if exchangeCode[0] < '2' || exchangeCode[0] > '9' {
		return false
	}

	// Prevent reserved/invalid numbers
	if areaCode == "911" || areaCode == "555" {
		return false
	}

	return true
}

func IsValidGlobalPhoneNumber(v string) bool {
	if len(v) < 7 || len(v) > 15 {
		return false
	}

	// Allow leading zero for local dialing in some countries
	if v[0] == '0' && len(v) > 7 {
		return true
	}

	// Must start with a valid country code (1-9)
	if v[0] < '1' || v[0] > '9' {
		return false
	}

	return true
}

func IsValidPhoneNumber(v string, c country.Country) bool {
	switch c {
	case country.UnitedStates:
		return IsValidUSPhoneNumber(v)
	default:
		return IsValidGlobalPhoneNumber(v)
	}
}

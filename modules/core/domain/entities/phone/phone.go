package phone

import (
	"unicode"

	"github.com/go-faster/errors"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/country"
)

var (
	ErrInvalidPhoneNumber = errors.New("invalid phone number")
	ErrUnknownCountry     = errors.New("unknown country from phone number")
)

// AreaCode represents the mapping between area codes and countries
type AreaCode struct {
	Country    country.Country
	AreaCodes  []string
	CodeLength int // Expected length of phone number for this country
}

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

// ParseCountry attempts to determine the country from a phone number
func ParseCountry(phoneNumber string) (country.Country, error) {
	cleaned := Strip(phoneNumber)
	if cleaned == "" {
		return country.NilCountry, errors.Wrap(ErrUnknownCountry, "phone number is empty")
	}

	// Try different prefix lengths (from longest to shortest)
	for i := 4; i >= 1; i-- {
		if len(cleaned) >= i {
			prefix := cleaned[:i]
			if countryCodes, ok := PhoneCodeToCountry[prefix]; ok {
				// If there's only one country for this prefix, return it
				if len(countryCodes) == 1 && len(countryCodes[0].AreaCodes) == 0 {
					return countryCodes[0].Country, nil
				}

				// For shared codes (like +1), we need to check area codes
				for _, countryCode := range countryCodes {
					if len(countryCode.AreaCodes) > 0 && len(cleaned) >= 3 {
						areaCode := cleaned[len(prefix) : len(prefix)+3]
						for _, validAreaCode := range countryCode.AreaCodes {
							if areaCode == validAreaCode {
								return countryCode.Country, nil
							}
						}
					}
				}
			}
		}
	}

	return country.NilCountry, errors.Wrapf(ErrUnknownCountry, "could not determine country for phone number: %s", phoneNumber)
}

func New(v string, c country.Country) (Phone, error) {
	if v == "" {
		return phone(""), errors.Wrap(ErrInvalidPhoneNumber, "phone number is empty")
	}

	stripped := Strip(v)
	if !IsValidPhoneNumber(stripped, c) {
		return phone(""), errors.Wrapf(ErrInvalidPhoneNumber, "phone number %s is invalid", v)
	}

	return phone(v), nil
}

// NewFromE164 creates a new Phone from an E.164 formatted number, automatically detecting the country
func NewFromE164(v string) (Phone, error) {
	if v == "" {
		return phone(""), errors.Wrap(ErrInvalidPhoneNumber, "phone number is empty")
	}

	stripped := Strip(v)
	detectedCountry, err := ParseCountry(stripped)
	if err != nil {
		return phone(""), err
	}

	return New(v, detectedCountry)
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

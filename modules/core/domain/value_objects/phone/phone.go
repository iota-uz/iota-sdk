package phone

import (
	"unicode"

	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/country"

	"github.com/go-faster/errors"
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
	E164() string
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
					if len(countryCode.AreaCodes) > 0 {
						// Check if we have enough digits for the area code
						minAreaCodeLen := len(countryCode.AreaCodes[0])
						if len(cleaned) >= len(prefix)+minAreaCodeLen {
							areaCode := cleaned[len(prefix) : len(prefix)+minAreaCodeLen]
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

	return phone(stripped), nil
}

func Parse(v string, c country.Country) (Phone, error) {
	if v == "" {
		return phone(""), errors.Wrap(ErrInvalidPhoneNumber, "phone number is empty")
	}

	e164 := ToE164Format(v, c)
	if !IsValidPhoneNumber(e164, c) {
		return phone(""), errors.Wrapf(ErrInvalidPhoneNumber, "phone number %s is invalid", v)
	}
	return phone(e164), nil
}

// NewFromE164 creates a new Phone from an E.164 formatted number, automatically detecting the country
func NewFromE164(v string) (Phone, error) {
	stripped := Strip(v)
	if stripped == "" {
		return phone(""), errors.Wrap(ErrInvalidPhoneNumber, "phone number is empty")
	}
	//	detectedCountry, err := ParseCountry(stripped)
	//	if err != nil {
	//		return phone(""), err
	//	}

	return phone("+" + stripped), nil
}

type phone string

// TODO: rewrite this, kept for backward compatibility
func (p phone) Value() string {
	return Strip(string(p))
}

func (p phone) E164() string {
	return string(p)
}

func IsValidUSPhoneNumber(v string) bool {
	return len(v) == 12 && v[:2] == "+1"
}

func IsValidUZPhoneNumber(v string) bool {
	return len(v) == 13 && v[:4] == "+998"
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
	case country.Uzbekistan:
		return IsValidUZPhoneNumber(v)
	case country.UnitedStates:
		return IsValidUSPhoneNumber(v)
	default:
		return IsValidGlobalPhoneNumber(v)
	}
}

// ToE164Format converts a phone number to E.164 format based on the country
func ToE164Format(phone string, c country.Country) string {
	// If already in E.164 format, just return it
	stripped := Strip(phone)

	switch c {
	case country.Uzbekistan:
		if len(stripped) == 9 {
			// Local format, add country code
			return "+998" + stripped
		} else if len(stripped) == 12 && stripped[:3] == "998" {
			// Already has country code, just add + prefix
			return "+" + stripped
		}
	case country.UnitedStates:
		if len(stripped) == 10 {
			// Local format, add country code
			return "+1" + stripped
		} else if len(stripped) == 11 && stripped[0] == '1' {
			// Already has country code, just add + prefix
			return "+" + stripped
		}
	default:
		// For other countries, use the existing number but ensure E.164 format
		// This is a simple implementation - in a real system you'd handle more specific cases
		return "+" + stripped
	}

	// Default case - just add + prefix to stripped number
	return "+" + stripped
}

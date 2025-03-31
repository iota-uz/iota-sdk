package internet

import (
	"errors"
	"strings"
)

var (
	ErrInvalidEmail = errors.New("invalid email")
)

func NewEmail(v string) (Email, error) {
	if !IsValidEmail(v) {
		return nil, ErrInvalidEmail
	}
	return email(v), nil
}

func MustParseEmail(v string) Email {
	e, err := NewEmail(v)
	if err != nil {
		panic(err)
	}
	return e
}

type email string

func (e email) Value() string {
	return string(e)
}

func (e email) Domain() string {
	parts := strings.Split(e.Value(), "@")
	if len(parts) < 2 {
		return ""
	}
	return parts[1]
}

func (e email) Username() string {
	parts := strings.Split(e.Value(), "@")
	if len(parts) < 1 {
		return ""
	}
	return parts[0]
}

func IsValidEmail(v string) bool {
	// Empty check
	if len(v) == 0 {
		return false
	}

	// Find @ symbol
	atIndex := strings.LastIndex(v, "@")
	if atIndex <= 0 || atIndex == len(v)-1 {
		return false
	}

	// Split into local and domain parts
	local := v[:atIndex]
	domain := v[atIndex+1:]

	// Validate local part
	if !isValidLocalPart(local) {
		return false
	}

	// Validate domain
	if !isValidDomain(domain) {
		return false
	}

	return true
}

func isValidLocalPart(s string) bool {
	if len(s) == 0 {
		return false
	}

	// Check each character
	for i, c := range s {
		isValid := (c >= 'a' && c <= 'z') ||
			(c >= 'A' && c <= 'Z') ||
			(c >= '0' && c <= '9') ||
			c == '.' || c == '_' || c == '%' ||
			c == '+' || c == '-'

		if !isValid {
			return false
		}

		// Don't allow consecutive dots
		if c == '.' && i > 0 && s[i-1] == '.' {
			return false
		}
	}

	// Don't allow starting or ending with dot
	return s[0] != '.' && s[len(s)-1] != '.'
}

func isValidDomain(s string) bool {
	// Find last dot for TLD
	lastDot := strings.LastIndex(s, ".")
	if lastDot <= 0 || lastDot >= len(s)-2 {
		return false
	}

	// Check TLD (needs at least 2 chars after last dot)
	tld := s[lastDot+1:]
	for _, c := range tld {
		if c < 'a' || c > 'z' {
			return false
		}
	}

	// Check rest of domain
	domain := s[:lastDot]
	for i, c := range domain {
		isValid := (c >= 'a' && c <= 'z') ||
			(c >= 'A' && c <= 'Z') ||
			(c >= '0' && c <= '9') ||
			c == '.' || c == '-'

		if !isValid {
			return false
		}

		// Don't allow consecutive dots or hyphens
		if (c == '.' || c == '-') && i > 0 && (domain[i-1] == '.' || domain[i-1] == '-') {
			return false
		}
	}

	// Don't allow starting or ending with dot or hyphen
	return domain[0] != '.' && domain[0] != '-' &&
		domain[len(domain)-1] != '.' && domain[len(domain)-1] != '-'
}

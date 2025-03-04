package general

import (
	"errors"
)

type Gender interface {
	String() string
}

var (
	Male   Gender = GenderEnum("male")
	Female Gender = GenderEnum("female")
	Other  Gender = GenderEnum("other")
)

type GenderEnum string

var (
	ErrInvalidGender = errors.New("invalid gender")
	NilGender        = GenderEnum("")
)

func (g GenderEnum) String() string {
	return string(g)
}

// IsValid checks if a given country code is valid.
func IsValid(c string) bool {
	switch c {
	case "male":
		return true
	case "female":
		return true
	case "other":
		return true
	default:
		return false
	}
}

func NewGender(c string) (Gender, error) {
	if IsValid(c) {
		return GenderEnum(c), nil
	}
	return nil, ErrInvalidGender
}

package general

import (
	"errors"
)

type Gender interface {
	String() string
}

var (
	Male   Gender = gender("male")
	Female Gender = gender("female")
	Other  Gender = gender("other")
)

type gender string

var (
	ErrInvalidGender = errors.New("invalid gender")
	NilGender        = gender("")
)

func (g gender) String() string {
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
		return gender(c), nil
	}
	return nil, ErrInvalidGender
}

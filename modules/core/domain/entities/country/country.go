package country

import (
	"errors"
	"slices"
)

type country string

var (
	ErrInvalidCountry = errors.New("invalid country")
	NilCountry        = country("")
)

func (c country) String() string {
	return string(c)
}

// IsValid checks if a given country code is valid.
func IsValid(c string) bool {
	return slices.Contains(AllCountries, country(c))
}

func New(c string) (Country, error) {
	if IsValid(c) {
		return country(c), nil
	}
	return nil, ErrInvalidCountry
}

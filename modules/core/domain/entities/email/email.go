package email

import (
	"errors"
	"regexp"
)

var (
	ErrInvalidEmail = errors.New("invalid email")
)

func New(v string) (Email, error) {
	if !IsValid(v) {
		return nil, ErrInvalidEmail
	}
	return email(v), nil
}

type email string

func (e email) Value() string {
	return string(e)
}

func IsValid(v string) bool {
	re := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	return re.MatchString(v)
}

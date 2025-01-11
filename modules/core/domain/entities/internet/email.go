package internet

import (
	"errors"
	"regexp"
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

type email string

func (e email) Value() string {
	return string(e)
}

func IsValidEmail(v string) bool {
	re := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	return re.MatchString(v)
}

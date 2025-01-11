package internet

import (
	"errors"
	"regexp"
)

var (
	ErrInvalidIP = errors.New("invalid email")
)

func NewIP(v string) (IP, error) {
	if !IsValidIP(v) {
		return nil, ErrInvalidIP
	}
	return ip(v), nil
}

type ip string

func (i ip) Value() string {
	return string(i)
}

func IsValidIP(v string) bool {
	re := regexp.MustCompile(`\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}`)
	return re.MatchString(v)
}

package composables

import (
	"errors"
)

var (
	ErrAppNotFound     = errors.New("app not found")
	ErrInvalidPassword = errors.New("invalid password")
	ErrNotFound        = errors.New("not found")
	ErrUnauthorized    = errors.New("unauthorized")
	ErrForbidden       = errors.New("forbidden")
	ErrInternal        = errors.New("internal error")
)

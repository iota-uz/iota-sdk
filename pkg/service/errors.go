package service

import "errors"

var (
	ErrNoTx            = errors.New("no transaction found in context")
	ErrInvalidPassword = errors.New("invalid password")
	ErrNotFound        = errors.New("not found")
	ErrUnauthorized    = errors.New("unauthorized")
	ErrForbidden       = errors.New("forbidden")
	ErrInternal        = errors.New("internal error")
)

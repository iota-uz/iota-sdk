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

type FindParams struct {
	Offset int
	Limit  int
	SortBy []string
	Joins  []string
}

type GetParams[T comparable] struct {
	Id    T
	Joins []string
}

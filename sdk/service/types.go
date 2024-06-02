package service

import "errors"

var (
	ErrNoTx = errors.New("no transaction found in context")
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

package service

type FindParams struct {
	Offset int
	Limit  int
	SortBy []string
	Joins  []string
}

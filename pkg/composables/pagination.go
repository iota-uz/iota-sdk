package composables

import (
	"net/http"
	"strconv"
)

// Query parameter constants to avoid circular import
const (
	QueryParamLimit = "limit"
	QueryParamPage  = "page"

	// DefaultPageSize is the default number of items returned per page.
	// Mirrors the legacy PAGE_SIZE env default (25).
	DefaultPageSize = 25

	// DefaultMaxPageSize is the upper bound for client-supplied page sizes.
	// Mirrors the legacy MAX_PAGE_SIZE env default (100).
	DefaultMaxPageSize = 100
)

type PaginationParams struct {
	Limit  int
	Offset int
	Page   int
}

func UsePaginated(r *http.Request) PaginationParams {
	limit, err := strconv.Atoi(r.URL.Query().Get(QueryParamLimit))
	if err != nil || limit > DefaultMaxPageSize {
		limit = DefaultPageSize
	}

	page, err := strconv.Atoi(r.URL.Query().Get(QueryParamPage))
	if err != nil || page < 1 {
		page = 1
	}

	return PaginationParams{
		Limit:  limit,
		Offset: (page - 1) * limit,
		Page:   page,
	}
}

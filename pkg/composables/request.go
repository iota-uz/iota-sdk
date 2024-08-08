package composables

import (
	"net/http"
	"strconv"
)

const pageSize = 25
const maxPageSize = 100

type PaginationParams struct {
	Limit  int
	Offset int
}

func UsePaginated(r *http.Request) PaginationParams {
	limit, err := strconv.Atoi(r.URL.Query().Get("limit"))
	if err != nil || limit > maxPageSize {
		limit = pageSize
	}

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))

	return PaginationParams{
		Limit:  limit,
		Offset: page * limit,
	}
}

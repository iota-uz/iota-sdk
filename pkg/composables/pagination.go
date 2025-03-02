package composables

import (
	"net/http"
	"strconv"

	"github.com/iota-uz/iota-sdk/pkg/configuration"
)

type PaginationParams struct {
	Limit  int
	Offset int
	Page   int
}

func UsePaginated(r *http.Request) PaginationParams {
	config := configuration.Use()
	limit, err := strconv.Atoi(r.URL.Query().Get("limit"))
	if err != nil || limit > config.MaxPageSize {
		limit = config.PageSize
	}

	page, err := strconv.Atoi(r.URL.Query().Get("page"))
	if err != nil || page < 1 {
		page = 1
	}

	return PaginationParams{
		Limit:  limit,
		Offset: (page - 1) * limit,
		Page:   page,
	}
}

package filters

import (
	"context"
	"net/url"
	"slices"
	"strconv"

	"github.com/iota-uz/iota-sdk/pkg/composables"
)

// pageSizes are the choices of PageSize; 25 is the default.
var pageSizes = []int{15, 25, 50, 100}

const defaultPageSize = 25

// currentQuery is the request's query, so the list's controls render with the
// values a reload or a page link carries; empty without a request.
func currentQuery(ctx context.Context) url.Values {
	if r, ok := composables.UseRequest(ctx); ok && r != nil && r.URL != nil {
		return r.URL.Query()
	}
	return url.Values{}
}

// pageSize is the ?limit= of the request (any positive size, so a link with
// another size keeps it), otherwise the default; options are the choices plus
// that size, in order.
func pageSize(ctx context.Context) (int, []int) {
	n, err := strconv.Atoi(currentQuery(ctx).Get("limit"))
	if err != nil || n <= 0 {
		return defaultPageSize, pageSizes
	}
	if slices.Contains(pageSizes, n) {
		return n, pageSizes
	}
	options := append(slices.Clone(pageSizes), n)
	slices.Sort(options)
	return n, options
}

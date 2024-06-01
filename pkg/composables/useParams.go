package composables

import (
	"context"
	"github.com/iota-agency/iota-erp/pkg/middleware"
)

func UseParams(ctx context.Context) (*middleware.RequestParams, bool) {
	params, ok := ctx.Value("params").(*middleware.RequestParams)
	return params, ok
}

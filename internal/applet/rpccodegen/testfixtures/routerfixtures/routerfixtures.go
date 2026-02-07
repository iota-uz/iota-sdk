package routerfixtures

import (
	"context"
	"io"

	"github.com/iota-uz/iota-sdk/pkg/applet"
)

type pingParams struct {
	Msg string `json:"msg"`
}

type pingResult struct {
	OK bool `json:"ok"`
}

func Router() *applet.TypedRPCRouter {
	r := applet.NewTypedRPCRouter()
	applet.AddProcedure(r, "fixtures.ping", applet.Procedure[pingParams, pingResult]{
		Handler: func(ctx context.Context, params pingParams) (pingResult, error) {
			return pingResult{OK: params.Msg != ""}, nil
		},
	})
	return r
}

func RouterWithDeps(_ io.Reader, _ *int) *applet.TypedRPCRouter {
	return Router()
}

func RouterBadReturn() int {
	return 42
}

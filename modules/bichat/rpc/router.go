package rpc

import (
	"context"

	"github.com/iota-uz/iota-sdk/pkg/applet"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

type PingParams struct{}

type PingResult struct {
	Ok       bool   `json:"ok"`
	TenantID string `json:"tenantId"`
}

func Router() *applet.TypedRPCRouter {
	r := applet.NewTypedRPCRouter()
	applet.AddProcedure(r, "bichat.ping", applet.Procedure[PingParams, PingResult]{
		RequirePermissions: []string{"bichat.access"},
		Handler: func(ctx context.Context, _ PingParams) (PingResult, error) {
			const op serrors.Op = "bichat.rpc.ping"

			tenantID, err := composables.UseTenantID(ctx)
			if err != nil {
				return PingResult{}, serrors.E(op, serrors.Internal, err)
			}

			return PingResult{
				Ok:       true,
				TenantID: tenantID.String(),
			}, nil
		},
	})

	return r
}

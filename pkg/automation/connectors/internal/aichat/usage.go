package aichat

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
)

// Units of the usage an AI call records.
const (
	UnitTokensIn  = "token_in"
	UnitTokensOut = "token_out"
)

// RecordUsage reports the tokens of one provider answer to the usage accounting of the call: two entries,
// token_in and token_out, on the connection the call runs with, a tenant's own or the platform's alike.
// Every answer is recorded, a repeated call's too (it is paid again), and so is one the node then
// refuses (a refusal, an answer outside the schema): the tokens were spent. The answer is paid for
// whatever happens here, so a failing recorder is logged and does not fail the node.
func RecordUsage(ctx context.Context, log *slog.Logger, in catalog.CallInput, u Usage, at time.Time) {
	if in.Usage == nil || in.Connection == nil {
		return
	}
	var run *uuid.UUID // a test of a node has no run
	if in.RunID != uuid.Nil {
		id := in.RunID
		run = &id
	}
	for _, e := range []struct {
		unit string
		n    int
	}{{UnitTokensIn, u.InputTokens}, {UnitTokensOut, u.OutputTokens}} {
		if e.n <= 0 {
			continue
		}
		err := in.Usage.Record(context.WithoutCancel(ctx), catalog.UsageEntry{
			TenantID: in.TenantID, Kind: in.Connection.Kind, ConnectionID: in.Connection.ID,
			Unit: e.unit, Quantity: int64(e.n), RunID: run, At: at,
		})
		if err != nil {
			if log == nil {
				log = slog.Default()
			}
			log.Warn("automation: AI usage was not recorded", "kind", string(in.Connection.Kind), "unit", e.unit, "error", err)
		}
	}
}

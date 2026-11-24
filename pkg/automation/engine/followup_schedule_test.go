package engine_test

// Regression tests of the follow-up to the engine's review (#1395).

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/automation/engine"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/events"
)

func TestTimeZonePrefixedCronIsRefusedNotAPanic(t *testing.T) {
	e := newEnv(t)
	p := e.proc()
	ctx := context.Background()
	for _, cron := range []string{"CRON_TZ=UTC", "TZ=Asia/Tashkent", "CRON_TZ=UTC 0 3 * *", "TZ=UTC 0 3 * * *"} {
		_, _, err := p.rt.Service().Create(ctx, e.human(), engine.CreateInput{Graph: json.RawMessage(`{"name": "Zoned",
		 "triggers": [{"id": "z", "event": "automation.schedule.v1", "params": {"cron": "` + cron + `"}}],
		 "nodes": [{"id": "fin", "type": "end", "params": {}}], "edges": [{"from": {"node": "z", "outcome": "fired"}, "to": "fin"}]}`)})
		var verr *engine.ValidationError
		require.ErrorAs(t, err, &verr, cron)
		assert.Equal(t, "triggers[0].params.cron", verr.Errors[0].Path, cron)
	}
	// A schedule stored before the rule: listing and running its slots refuse it as well.
	e.create(p, e.human(), `{"name": "Nightly", "triggers": [{"id": "night", "event": "automation.schedule.v1", "params": {"cron": "0 3 * * *"}}],
	 "nodes": [{"id": "fin", "type": "end", "params": {}}], "edges": [{"from": {"node": "night", "outcome": "fired"}, "to": "fin"}]}`)
	listed, err := p.rt.Schedules().ActiveSchedules(ctx)
	require.NoError(t, err)
	require.Len(t, listed, 1)
	_, err = e.db.Exec(ctx, `UPDATE automation.triggers SET schedule = '{"cron": "CRON_TZ=UTC"}' WHERE kind = 'schedule'`)
	require.NoError(t, err)
	now, err := p.rt.Schedules().ActiveSchedules(ctx)
	require.NoError(t, err)
	assert.Empty(t, now)
	tick := time.Now().UTC().Truncate(24 * time.Hour).Add(27 * time.Hour)
	require.NoError(t, pgx.BeginFunc(ctx, e.db, func(tx pgx.Tx) error {
		return listed[0].Handler.RunSlot(ctx, tx, events.Slot{ScheduleID: listed[0].ID, TenantID: e.tenant, At: tick})
	}))
	assert.Zero(t, e.count(`SELECT count(*) FROM automation.runs`))
}

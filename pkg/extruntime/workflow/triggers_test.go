package workflow_test

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/events"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/host"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/host/hosttest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/workflow"
)

type subSource struct {
	mu   sync.Mutex
	list []events.Subscription
}

func (s *subSource) ActiveSubscriptions(context.Context) ([]events.Subscription, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]events.Subscription{}, s.list...), nil
}

type schedSource struct{ defs []events.ScheduleDef }

func (s schedSource) ActiveSchedules(context.Context) ([]events.ScheduleDef, error) {
	return s.defs, nil
}

func instanceCount(t *testing.T, env *hosttest.Env) int {
	t.Helper()
	var n int
	require.NoError(t, env.DB.QueryRow(context.Background(), `SELECT count(*) FROM extruntime.workflow_instances WHERE tenant_id = $1`, env.Tenant).Scan(&n))
	return n
}

func TestSubscriptionStartIntentCommitsWithTheReceiptAndNeverDuplicates(t *testing.T) {
	env := hosttest.New(t)
	im := newImpls()
	publishFlow(t, env, im, "acme.a", "1.0.0", `export const flow = async (wf, input) => ({ started: input.who });`)
	im.Activate(flowID, "acme.a", "1.0.0")
	env.Subjects.Grant(7, "p")
	env.Catalog.Add(spi.ResolvedResource{Package: "acme.a", Version: "1.0.0", Resource: manifest.Resource{
		ID: "acme.test.thing.v1", Kind: manifest.ResEvent, Payload: hosttest.Obj("name", hosttest.Str()),
	}})
	env.Artifacts.Add(t, "acme.emit", "1.0.0", `
export const emit = async (ctx, input) => ctx.transaction(async (tx) => {
  await tx.emit({ id: "acme.test.thing.v1" }, { payload: { name: input.name } });
  if (input.rollback) throw new Error("rolled back");
  return {};
});`)
	env.Catalog.Add(spi.ResolvedResource{Package: "acme.emit", Version: "1.0.0", Resource: hosttest.Command("acme.test.emit", false,
		hosttest.Obj("name", hosttest.Str(), "rollback", hosttest.Bool()), hosttest.Obj())})
	_, err := env.Bindings.Activate(context.Background(), "acme.test.emit", "acme.emit", "1.0.0", "emit", nil)
	require.NoError(t, err)
	subArt := env.Artifacts.Add(t, "acme.sub", "1.0.0", `
export const onThing = async (tx, e) => { await tx.start({ id: "acme.test.flow" }, { who: e.payload.name }); };`)
	sub := events.Subscription{ID: "acme.sub.start", Package: "acme.sub", Version: "1.0.0", Export: "onThing", Event: "acme.test.thing.v1", Artifact: subArt}

	st := newStack(t, env, im)
	disp, err := events.NewDispatcher(events.Config{
		DB: env.DB, Runner: st.host, Subscriptions: &subSource{list: []events.Subscription{sub}}, Subjects: env.Subjects,
		BackoffBase: 10 * time.Millisecond, Logger: env.Logger,
	})
	require.NoError(t, err)
	emit := func(name string, rollback bool) error {
		in, err := json.Marshal(struct {
			Name     string `json:"name"`
			Rollback bool   `json:"rollback"`
		}{Name: name, Rollback: rollback})
		if err != nil {
			return err
		}
		_, err = st.host.Invoke(context.Background(), host.Request{Contract: "acme.test.emit", Input: in, Subject: env.Subject(7, "p"), Trigger: host.Trigger{Kind: host.TriggerUI}})
		return err
	}
	deliver := func(want int) {
		env.Eventually(20*time.Second, func() bool {
			_, err := disp.RunOnce(context.Background())
			require.NoError(t, err)
			return instanceCount(t, env) >= want
		}, "dispatch")
	}

	require.Error(t, emit("ghost", true))
	n, err := disp.RunOnce(context.Background())
	require.NoError(t, err)
	require.Zero(t, n)
	require.Zero(t, instanceCount(t, env), "a rolled back event starts nothing")

	require.NoError(t, emit("real", false))
	deliver(1)
	list, err := st.engine.List(context.Background(), workflow.ListFilter{TenantID: env.Tenant})
	require.NoError(t, err)
	require.Len(t, list, 1)
	inst := list[0]
	require.EqualValues(t, 7, inst.SubjectUserID)
	require.Equal(t, spi.SubjectInitiator, inst.SubjectKind, "the workflow acts as the event initiator")
	require.NotNil(t, inst.StartKey)
	require.Regexp(t, `^acme\.sub\.start:[0-9a-f-]{36}#1$`, *inst.StartKey, "start key is subscription:event")

	// The delivery is repeated after commit (receipt lost, second node, replay):
	// the start intent is keyed by (subscription, event), so no second instance.
	_, err = env.DB.Exec(context.Background(), `DELETE FROM extruntime.subscription_receipts`)
	require.NoError(t, err)
	n, err = disp.RunOnce(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, n, "the handler ran again")
	require.Equal(t, 1, instanceCount(t, env), "but the start was deduped")

	require.Equal(t, 1, st.drain())
	require.Equal(t, workflow.StatusCompleted, st.status(inst.ID))
	require.JSONEq(t, `{"started":"real"}`, string(st.instance(inst.ID).Result))
}

func TestScheduleStartsAWorkflowAsTheOwnerOncePerSlot(t *testing.T) {
	env := hosttest.New(t)
	im := newImpls()
	publishFlow(t, env, im, "acme.a", "1.0.0", `export const flow = async (wf, input) => ({ by: wf.user.kind + ":" + wf.user.userId, who: input.who });`)
	im.Activate(flowID, "acme.a", "1.0.0")
	env.Subjects.Grant(42, "p")
	art := env.Artifacts.Add(t, "acme.sched", "1.0.0", `
export const handler = async (ctx) => {
  await ctx.transaction(async (tx) => { await tx.start({ id: "acme.test.flow" }, { who: ctx.slot }); });
};`)
	def := events.ScheduleDef{ID: "acme.sched.daily", Package: "acme.sched", Version: "1.0.0", Export: "handler", Cron: "* * * * *", Artifact: art}
	_, err := env.DB.Exec(context.Background(), `INSERT INTO extruntime.trigger_owners (trigger_id, tenant_id, user_id) VALUES ($1, $2, 42)`, def.ID, env.Tenant)
	require.NoError(t, err)
	st := newStack(t, env, im)
	now := time.Date(2026, 6, 1, 10, 30, 20, 0, time.UTC)
	sched, err := events.NewScheduler(events.SchedulerConfig{
		DB: env.DB, Runner: st.host, Schedules: schedSource{defs: []events.ScheduleDef{def}}, Subjects: env.Subjects,
		Lookback: time.Minute, Logger: env.Logger, Now: func() time.Time { return now },
	})
	require.NoError(t, err)

	for i := 0; i < 3; i++ {
		_, err := sched.RunOnce(context.Background())
		require.NoError(t, err)
	}
	require.Equal(t, 1, instanceCount(t, env), "the slot started one workflow however often the scheduler ran")
	require.Equal(t, 1, st.drain())
	list, err := st.engine.List(context.Background(), workflow.ListFilter{TenantID: env.Tenant})
	require.NoError(t, err)
	require.Equal(t, spi.SubjectOwner, list[0].SubjectKind)
	require.JSONEq(t, `{"by":"owner:42","who":"2026-06-01T10:30:00Z"}`, string(list[0].Result))
	require.Regexp(t, `^schedule:acme\.sched\.daily:2026-06-01T10:30:00Z#1$`, *list[0].StartKey)

	now = now.Add(time.Minute)
	_, err = sched.RunOnce(context.Background())
	require.NoError(t, err)
	require.Equal(t, 2, instanceCount(t, env))
}

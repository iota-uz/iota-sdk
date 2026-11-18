package e2e_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/manage"
)

// Scenario 8: a live workflow instance issues wf.call after the binding of the
// called command changed; the new invocation uses the new implementation, and
// a replay of the instance returns the earlier receipt instead of calling
// again.
func TestScenario08WorkflowCallFollowsBindingAndReplaysReceipts(t *testing.T) {
	e := newEnv(t)
	admin := e.admin
	user := e.newActor("clerk")
	e.deploy(admin, "e2e.pricing", "1.0.0", loadTree(t, "pricing"))
	e.deploy(admin, "e2e.promo", "1.0.0", loadTree(t, "promo"))
	e.deploy(admin, "e2e.caller", "1.0.0", loadTree(t, "caller"))

	o1 := e.newOrder(user, "W-1", 10000)
	o2 := e.newOrder(user, "W-2", 10000)
	var started struct {
		Instance string `json:"instance"`
	}
	e.mustRPC(user, "command", "e2e.caller.start", map[string]any{"first": o1, "second": o2, "percent": 10}, &started)
	id := uuid.MustParse(started.Instance)
	e.waitStatus(admin, id, "waiting")

	// The binding changes while the instance is alive.
	e.bind(admin, manage.SlotCommand, "e2e.pricing.checkout", "e2e.promo", "1.0.0", "checkout", 201)
	e.signal(admin, id, "go1", map[string]any{}, "g1")
	e.eventually("the instance moved on to its second wait", 30e9, func() bool {
		for _, o := range e.workflow(admin, id).Occurrences {
			if o.Name == "go2" && o.Status == "waiting" {
				return true
			}
		}
		return false
	})
	require.Equal(t, checkoutOut{Cents: 8500, Status: "open"}, e.readOrder(user, o1), "the replacement ran: 10% + 5% bonus; it cannot mark the order paid")

	// The binding changes back; the replay for the next wait must not call the
	// first checkout again, and the second call uses the original.
	e.unbind(admin, manage.SlotCommand, "e2e.pricing.checkout")
	e.signal(admin, id, "go2", map[string]any{}, "g2")
	done := e.waitStatus(admin, id, "completed")
	require.JSONEq(t, `{"first":"promo:8500","second":"paid:9000"}`, string(done.Result))
	require.Equal(t, checkoutOut{Cents: 8500, Status: "open"}, e.readOrder(user, o1), "replay did not discount the first order again")
	require.Equal(t, checkoutOut{Cents: 9000, Status: "paid"}, e.readOrder(user, o2))

	// Each call is an invocation of the workflow trigger that recorded its binding.
	rows, err := e.pool.Query(e.ctx, `SELECT binding_package FROM extruntime.invocations
		WHERE contract = 'e2e.pricing.checkout' AND trigger_kind = 'workflow' ORDER BY started_at`)
	require.NoError(t, err)
	defer rows.Close()
	var got []string
	for rows.Next() {
		var p string
		require.NoError(t, rows.Scan(&p))
		got = append(got, p)
	}
	require.Equal(t, []string{"e2e.promo", "e2e.pricing"}, got)

	h := e.workflow(admin, id)
	var calls int
	for _, o := range h.Occurrences {
		if o.Name == "checkout" {
			calls++
			require.Equal(t, "completed", o.Status)
		}
	}
	require.Equal(t, 2, calls, "two occurrences of the same name, counted n=0 and n=1")
}

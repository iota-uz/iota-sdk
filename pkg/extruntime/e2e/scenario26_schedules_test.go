package e2e_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/events"
)

// Scenario 26: the process has one scheduler, the core's. The runtime serves
// no scheduler of its own: its package schedules are one source of the core's,
// next to the Go schedules (wiring.ContributeSchedules; see the wiring tests),
// and run through the runtime's host.
func TestScenario26OneSchedulerServesGoAndPackageSchedules(t *testing.T) {
	e := newEnv(t)
	core := resolve[*events.Scheduler](t, e.scope.Container)
	require.Same(t, core, e.rt.Scheduler, "the runtime reuses the core scheduler")

	const ns = "e2e.scheduled"
	e.deploy(e.admin, ns, "1.0.0", loadTree(t, "scheduled"))
	e.assignOwner(e.admin, ns+".nightly", e.admin)
	e.eventually("the package schedule fires through the core scheduler", 90*time.Second, func() bool {
		return e.scalarInt(`SELECT count(*) FROM extruntime.schedule_runs WHERE schedule = $1 AND status = 'done'`, ns+".nightly") >= 1
	})
}

package e2e_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/events"
)

// With the runtime composed next to the always-on core, the process runs one
// outbox dispatcher: the core's. The runtime builds none of its own; its
// package subscriptions are consumers of the core dispatcher (the other
// scenarios deliver through it).
func TestScenario25_OneDispatcherServesCoreAndPackages(t *testing.T) {
	e := newEnv(t)
	core := resolve[*events.Dispatcher](t, e.scope.Container)
	require.Same(t, core, e.rt.Dispatcher, "the runtime reuses the core dispatcher")
	require.Equal(t, 1, events.Running(), "one dispatcher serves the process")
}

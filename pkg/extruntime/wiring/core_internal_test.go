package wiring

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/events"
)

// Options.Events wins; settings it leaves zero come from the runtime
// configuration a Tune produced.
func TestMergeEventsKeepsExplicitSettingsAndFillsTheRest(t *testing.T) {
	got := mergeEvents(
		events.Config{PollInterval: time.Second, MaxAttempts: 9},
		events.Config{PollInterval: time.Minute, BatchSize: 7, MaxAttempts: 2, Retention: time.Hour, Concurrency: 3},
	)
	require.Equal(t, time.Second, got.PollInterval)
	require.Equal(t, 9, got.MaxAttempts)
	require.Equal(t, 7, got.BatchSize)
	require.Equal(t, 3, got.Concurrency)
	require.Equal(t, time.Hour, got.Retention)
}

// Two contributions of one native workflow ID are a composition error.
func TestMergeNativeWorkflowsRejectsDuplicates(t *testing.T) {
	a := nativeWorkflows{"test.a": {}, "test.b": {}}
	got, err := mergeNativeWorkflows([]nativeWorkflows{a, {"test.c": {}}})
	require.NoError(t, err)
	require.Len(t, got, 3)
	_, err = mergeNativeWorkflows([]nativeWorkflows{a, {"test.b": {}}})
	require.ErrorContains(t, err, "test.b is contributed twice")
}

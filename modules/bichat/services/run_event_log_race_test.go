package services

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRunEventLog_ConcurrentAppendsTailReceivesTerminal spawns multiple
// goroutines appending content events and one appending a terminal "done".
// It asserts the Tail consumer sees at least one terminal event and that
// the channel closes cleanly. Run with -race to validate no data races.
func TestRunEventLog_ConcurrentAppendsTailReceivesTerminal(t *testing.T) {
	t.Parallel()

	log, _ := newTestRunEventLog(t)
	tenant := uuid.New()
	run := uuid.New()

	// Pre-seed one event so Tail has something to read immediately, avoiding
	// XREAD BLOCK flakiness on empty streams in miniredis.
	appendEvent(t, log, tenant, run, "content", map[string]string{"text": "seed"})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ch, err := log.Tail(ctx, tenant, run, RunEventStreamStart)
	require.NoError(t, err)

	const writers = 4
	const eventsPerWriter = 50

	var wg sync.WaitGroup
	// Spawn content writers.
	for i := 0; i < writers; i++ {
		wg.Add(1)
		go func(writerIdx int) {
			defer wg.Done()
			for j := 0; j < eventsPerWriter; j++ {
				body, _ := json.Marshal(map[string]any{"writer": writerIdx, "seq": j}) //nolint:errchkjson // map[string]any with int values cannot fail
				_, _ = log.Append(context.Background(), tenant, run, RunEvent{
					Type:    "content",
					Payload: body,
				})
			}
		}(i)
	}

	// One goroutine appends the terminal event after all writers have queued.
	go func() {
		wg.Wait()
		body, _ := json.Marshal(map[string]string{}) //nolint:errchkjson // empty map cannot fail
		_, _ = log.Append(context.Background(), tenant, run, RunEvent{
			Type:    "done",
			Payload: body,
		})
	}()

	// Consumer drains the channel; channel must close after a terminal event.
	var sawTerminal bool
	for evt := range ch {
		if IsRunEventTerminal(evt.Type) {
			sawTerminal = true
		}
	}

	assert.True(t, sawTerminal, "consumer must see a terminal event before channel closes")
}

// TestRunEventLog_TailCancelStopsGoroutine verifies that cancelling the context
// closes the Tail channel without leaking the XREAD goroutine. The test uses a
// very short blockTime so the goroutine exits quickly on context cancellation.
func TestRunEventLog_TailCancelStopsGoroutine(t *testing.T) {
	t.Parallel()

	// newTestRunEventLog already uses 30ms blockTime — suitable for this test.
	log, _ := newTestRunEventLog(t)

	tenant := uuid.New()
	run := uuid.New()

	// Seed one event so the first XREAD returns immediately.
	appendEvent(t, log, tenant, run, "content", map[string]string{"text": "hi"})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ch, err := log.Tail(ctx, tenant, run, RunEventStreamStart)
	require.NoError(t, err)

	// Drain the seeded event.
	select {
	case _, ok := <-ch:
		if !ok {
			// Channel already closed — that's fine.
			return
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for seeded event")
	}

	// Cancel and verify the channel closes within the block timeout.
	cancel()
	select {
	case _, ok := <-ch:
		assert.False(t, ok, "channel must close after context cancel")
	case <-time.After(500 * time.Millisecond):
		t.Fatal("goroutine did not exit after context cancel")
	}
}

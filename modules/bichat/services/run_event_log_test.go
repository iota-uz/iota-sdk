package services

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestRunEventLog(t *testing.T) (*RedisRunEventLog, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	log, err := NewRedisRunEventLog(RedisRunEventLogConfig{
		RedisURL:  mr.Addr(),
		BlockTime: 30 * time.Millisecond, // short block so Tail exits quickly in tests
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = log.Close() })
	return log, mr
}

func appendEvent(t *testing.T, log *RedisRunEventLog, tenant, run uuid.UUID, typ string, payload any) string {
	t.Helper()
	body, err := json.Marshal(payload)
	require.NoError(t, err)
	id, err := log.Append(context.Background(), tenant, run, RunEvent{Type: typ, Payload: body})
	require.NoError(t, err)
	return id
}

func TestRedisRunEventLog_AppendRefreshesTTL(t *testing.T) {
	t.Parallel()

	log, mr := newTestRunEventLog(t)
	tenant, run := uuid.New(), uuid.New()

	id1 := appendEvent(t, log, tenant, run, "content", map[string]string{"text": "hi"})
	require.NotEmpty(t, id1)

	key := log.streamKey(tenant, run)
	ttl := mr.TTL(key)
	assert.Greater(t, ttl, time.Duration(0), "TTL must be positive after append")

	// Fast-forward time to just under the configured TTL; a second append
	// should refresh the TTL back to ~2h.
	mr.FastForward(time.Hour)
	beforeSecond := mr.TTL(key)
	appendEvent(t, log, tenant, run, "content", map[string]string{"text": "again"})
	afterSecond := mr.TTL(key)

	assert.Greater(t, afterSecond, beforeSecond, "Append must refresh TTL")
}

func TestRedisRunEventLog_ReplayReturnsEventsInOrder(t *testing.T) {
	t.Parallel()

	log, _ := newTestRunEventLog(t)
	tenant, run := uuid.New(), uuid.New()

	appendEvent(t, log, tenant, run, "content", map[string]string{"text": "one"})
	appendEvent(t, log, tenant, run, "content", map[string]string{"text": "two"})
	appendEvent(t, log, tenant, run, "done", map[string]string{})

	events, err := log.Replay(context.Background(), tenant, run, RunEventStreamStart)
	require.NoError(t, err)
	require.Len(t, events, 3)
	assert.Equal(t, "content", events[0].Type)
	assert.Equal(t, "content", events[1].Type)
	assert.Equal(t, "done", events[2].Type)
}

func TestRedisRunEventLog_ReplayFromCursorIsExclusive(t *testing.T) {
	t.Parallel()

	log, _ := newTestRunEventLog(t)
	tenant, run := uuid.New(), uuid.New()

	first := appendEvent(t, log, tenant, run, "content", map[string]string{"text": "one"})
	appendEvent(t, log, tenant, run, "content", map[string]string{"text": "two"})
	appendEvent(t, log, tenant, run, "done", map[string]string{})

	// Replaying from first must skip the first event (exclusive semantics).
	events, err := log.Replay(context.Background(), tenant, run, first)
	require.NoError(t, err)
	require.Len(t, events, 2)
	assert.Equal(t, "content", events[0].Type)
	assert.Equal(t, "done", events[1].Type)
}

func TestRedisRunEventLog_TailClosesOnTerminal(t *testing.T) {
	t.Parallel()

	log, _ := newTestRunEventLog(t)
	tenant, run := uuid.New(), uuid.New()

	// Pre-seed so Tail has something to read immediately; avoids flakes
	// from miniredis not supporting XREAD BLOCK with zero entries.
	appendEvent(t, log, tenant, run, "content", map[string]string{"text": "one"})
	appendEvent(t, log, tenant, run, "content", map[string]string{"text": "two"})
	appendEvent(t, log, tenant, run, "done", map[string]string{})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	ch, err := log.Tail(ctx, tenant, run, RunEventStreamStart)
	require.NoError(t, err)

	seen := make([]string, 0, 3) // 2 content + 1 done seeded above
	for evt := range ch {
		seen = append(seen, evt.Type)
	}
	assert.Equal(t, []string{"content", "content", "done"}, seen, "tail must stop at the terminal done event")
}

func TestRedisRunEventLog_TailHonoursContextCancellation(t *testing.T) {
	t.Parallel()

	log, _ := newTestRunEventLog(t)
	tenant, run := uuid.New(), uuid.New()

	appendEvent(t, log, tenant, run, "content", map[string]string{"text": "ping"})

	ctx, cancel := context.WithCancel(context.Background())
	ch, err := log.Tail(ctx, tenant, run, RunEventStreamStart)
	require.NoError(t, err)

	// Drain the seeded event, then cancel — channel must close promptly.
	select {
	case evt, ok := <-ch:
		require.True(t, ok)
		assert.Equal(t, "content", evt.Type)
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for seeded event")
	}
	cancel()
	select {
	case _, ok := <-ch:
		assert.False(t, ok, "channel must close after context cancel")
	case <-time.After(500 * time.Millisecond):
		t.Fatal("tail did not exit on context cancel")
	}
}

func TestRedisRunEventLog_DropAfterTerminalShortensTTL(t *testing.T) {
	t.Parallel()

	log, mr := newTestRunEventLog(t)
	tenant, run := uuid.New(), uuid.New()

	appendEvent(t, log, tenant, run, "done", map[string]string{})

	before := mr.TTL(log.streamKey(tenant, run))
	require.NoError(t, log.DropAfterTerminal(context.Background(), tenant, run, 30*time.Second))
	after := mr.TTL(log.streamKey(tenant, run))

	assert.Less(t, after, before, "terminal TTL must be shorter than the live TTL")
	assert.Greater(t, after, time.Duration(0))
}

func TestIsRunEventTerminal(t *testing.T) {
	t.Parallel()

	cases := map[string]bool{
		"content":   false,
		"tool_end":  false,
		"done":      true,
		"cancelled": true,
		"error":     true,
		"failed":    true,
	}
	for typ, want := range cases {
		got := IsRunEventTerminal(typ)
		assert.Equal(t, want, got, "IsRunEventTerminal(%q)", typ)
	}
}

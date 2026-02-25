package streaming

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	bichatservices "github.com/iota-uz/iota-sdk/pkg/bichat/services"
	"github.com/iota-uz/iota-sdk/pkg/bichat/types"
	"github.com/stretchr/testify/require"
)

func TestRunRegistry_AddGetRemove(t *testing.T) {
	reg := NewRunRegistry()
	run := NewActiveRun(uuid.New(), uuid.New(), context.CancelFunc(func() {}), time.Now())

	reg.Add(run)
	require.Equal(t, run, reg.GetByRun(run.RunID))
	require.Equal(t, run, reg.GetBySession(run.SessionID))

	reg.Remove(run.RunID)
	require.Nil(t, reg.GetByRun(run.RunID))
	require.Nil(t, reg.GetBySession(run.SessionID))
}

func TestRunRegistry_AddReplacesExistingSessionRun(t *testing.T) {
	reg := NewRunRegistry()
	sessionID := uuid.New()
	first := NewActiveRun(uuid.New(), sessionID, context.CancelFunc(func() {}), time.Now())
	second := NewActiveRun(uuid.New(), sessionID, context.CancelFunc(func() {}), time.Now())

	reg.Add(first)
	reg.Add(second)

	require.Equal(t, second, reg.GetBySession(sessionID))
	require.Nil(t, reg.GetByRun(first.RunID))
	require.Equal(t, second, reg.GetByRun(second.RunID))
}

func TestActiveRun_SubscriberLifecycleAndSnapshot(t *testing.T) {
	run := NewActiveRun(uuid.New(), uuid.New(), context.CancelFunc(func() {}), time.Now())
	ch := make(chan bichatservices.StreamChunk, 1)
	run.AddSubscriber(ch)

	run.Broadcast(bichatservices.StreamChunk{Type: bichatservices.ChunkTypeContent, Content: "x", Timestamp: time.Now()})
	select {
	case <-ch:
	case <-time.After(time.Second):
		t.Fatal("expected broadcast chunk")
	}

	run.Mu.Lock()
	run.ToolCalls["c1"] = types.ToolCall{ID: "c1", Name: "tool"}
	run.ToolOrder = append(run.ToolOrder, "c1")
	run.Mu.Unlock()
	meta := run.SnapshotMetadata()
	require.Contains(t, meta, "tool_calls")

	run.CloseAllSubscribers()
	_, ok := <-ch
	require.False(t, ok, "CloseAllSubscribers should close subscriber channel")
}

func TestActiveRun_ConcurrentAddRemove(t *testing.T) {
	run := NewActiveRun(uuid.New(), uuid.New(), context.CancelFunc(func() {}), time.Now())
	var wg sync.WaitGroup

	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ch := make(chan bichatservices.StreamChunk, 1)
			run.AddSubscriber(ch)
			run.RemoveSubscriber(ch)
		}()
	}

	wg.Wait()
}

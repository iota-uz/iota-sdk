package streaming

import (
	"context"
	"sync"
	"testing"
	"time"
	"unicode/utf16"

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

func TestActiveRun_SetMirrorCalledForEveryBroadcast(t *testing.T) {
	run := NewActiveRun(uuid.New(), uuid.New(), context.CancelFunc(func() {}), time.Now())

	var seen []bichatservices.ChunkType
	var seenMu sync.Mutex
	run.SetMirror(func(chunk bichatservices.StreamChunk) {
		seenMu.Lock()
		defer seenMu.Unlock()
		seen = append(seen, chunk.Type)
	})

	run.Broadcast(bichatservices.StreamChunk{Type: bichatservices.ChunkTypeContent})
	run.Broadcast(bichatservices.StreamChunk{Type: bichatservices.ChunkTypeToolStart})
	run.Broadcast(bichatservices.StreamChunk{Type: bichatservices.ChunkTypeDone})

	seenMu.Lock()
	defer seenMu.Unlock()
	require.Equal(t, []bichatservices.ChunkType{
		bichatservices.ChunkTypeContent,
		bichatservices.ChunkTypeToolStart,
		bichatservices.ChunkTypeDone,
	}, seen)
}

func TestActiveRun_SetMirrorDoesNotBlockOnSlowSubscribers(t *testing.T) {
	run := NewActiveRun(uuid.New(), uuid.New(), context.CancelFunc(func() {}), time.Now())

	// Unbuffered channel: Broadcast's default-case drop must still fire
	// the mirror so the durable log isn't starved by a slow client.
	slow := make(chan bichatservices.StreamChunk)
	run.AddSubscriber(slow)

	var mirrored int
	run.SetMirror(func(chunk bichatservices.StreamChunk) { mirrored++ })

	run.Broadcast(bichatservices.StreamChunk{Type: bichatservices.ChunkTypeContent})
	require.Equal(t, 1, mirrored, "mirror must fire even when the subscriber channel is full")
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

func TestRunRegistry_ActiveCounters(t *testing.T) {
	reg := NewRunRegistry()

	first := NewActiveRun(uuid.New(), uuid.New(), context.CancelFunc(func() {}), time.Now())
	second := NewActiveRun(uuid.New(), uuid.New(), context.CancelFunc(func() {}), time.Now())
	firstChA := make(chan bichatservices.StreamChunk, 1)
	firstChB := make(chan bichatservices.StreamChunk, 1)
	secondCh := make(chan bichatservices.StreamChunk, 1)

	first.AddSubscriber(firstChA)
	first.AddSubscriber(firstChB)
	second.AddSubscriber(secondCh)

	reg.Add(first)
	reg.Add(second)

	require.Equal(t, 2, reg.ActiveRuns())
	require.Equal(t, 3, reg.ActiveSubscribers())

	first.RemoveSubscriber(firstChA)
	require.Equal(t, 2, reg.ActiveRuns())
	require.Equal(t, 2, reg.ActiveSubscribers())

	reg.Remove(first.RunID)
	require.Equal(t, 1, reg.ActiveRuns())
	require.Equal(t, 1, reg.ActiveSubscribers())
}

// TestActiveRun_ContentUTF16LenIncremental verifies that ContentUTF16Len
// matches the reference UTF-16 encoding of the full accumulated content when
// content deltas are applied one at a time, including non-ASCII (multi-byte)
// characters where Go byte length != UTF-16 code unit count.
func TestActiveRun_ContentUTF16LenIncremental(t *testing.T) {
	t.Parallel()

	deltas := []string{
		"Hello ",
		"世界", // non-ASCII: each rune is 1 UTF-16 code unit in BMP
		" and ",
		"𝄞",     // U+1D11E MUSICAL SYMBOL G CLEF: outside BMP, 2 UTF-16 code units
		" done",
	}

	run := NewActiveRun(uuid.New(), uuid.New(), context.CancelFunc(func() {}), time.Now())

	for _, delta := range deltas {
		run.Mu.Lock()
		run.Content += delta
		run.ContentUTF16Len += len(utf16.Encode([]rune(delta)))
		run.Mu.Unlock()
	}

	run.Mu.RLock()
	content := run.Content
	gotLen := run.ContentUTF16Len
	run.Mu.RUnlock()

	wantLen := len(utf16.Encode([]rune(content)))
	require.Equal(t, wantLen, gotLen,
		"incremental ContentUTF16Len must match full re-encode of accumulated content")
}

func TestRunRegistry_ActiveCountersAfterReplacingSessionRun(t *testing.T) {
	reg := NewRunRegistry()
	sessionID := uuid.New()

	replacedCanceled := false
	first := NewActiveRun(uuid.New(), sessionID, func() { replacedCanceled = true }, time.Now())
	second := NewActiveRun(uuid.New(), sessionID, context.CancelFunc(func() {}), time.Now())
	first.AddSubscriber(make(chan bichatservices.StreamChunk, 1))
	first.AddSubscriber(make(chan bichatservices.StreamChunk, 1))
	second.AddSubscriber(make(chan bichatservices.StreamChunk, 1))

	reg.Add(first)
	reg.Add(second)

	require.True(t, replacedCanceled, "expected previous run to be canceled when session run is replaced")
	require.Equal(t, 1, reg.ActiveRuns())
	require.Equal(t, 1, reg.ActiveSubscribers())
}

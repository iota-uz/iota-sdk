// Package streaming provides this package.
package streaming

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
	bichatservices "github.com/iota-uz/iota-sdk/pkg/bichat/services"
	"github.com/iota-uz/iota-sdk/pkg/bichat/types"
)

// ActiveRun holds in-memory state for a streaming run so clients can resume.
type ActiveRun struct {
	RunID       uuid.UUID
	SessionID   uuid.UUID
	Cancel      context.CancelFunc
	StartedAt   time.Time
	Content     string
	ToolCalls   map[string]types.ToolCall
	ToolOrder   []string
	ArtifactMap map[string]types.ToolArtifact
	LastPersist time.Time
	// TextBlockOffsets are byte offsets into Content marking the end of each
	// completed assistant text segment. The first entry corresponds to seq 0,
	// the second to seq 1, etc. The trailing un-closed segment (if any) is
	// implicit and runs from the last offset to len(Content).
	TextBlockOffsets []int

	subscribers map[chan bichatservices.StreamChunk]struct{}
	// mirrorFn is an optional side-effect hook invoked by Broadcast AFTER
	// in-memory subscribers are notified. It lets callers dual-write the
	// chunk to a durable store (e.g. RunEventLog on Redis) without the
	// runStreamLoop having to care where the event ends up. Installed via
	// SetMirror at run creation time.
	mirrorFn func(bichatservices.StreamChunk)
	Mu       sync.RWMutex
}

func NewActiveRun(runID, sessionID uuid.UUID, cancel context.CancelFunc, startedAt time.Time) *ActiveRun {
	return &ActiveRun{
		RunID:       runID,
		SessionID:   sessionID,
		Cancel:      cancel,
		StartedAt:   startedAt,
		ToolCalls:   make(map[string]types.ToolCall),
		ToolOrder:   make([]string, 0),
		ArtifactMap: make(map[string]types.ToolArtifact),
		subscribers: make(map[chan bichatservices.StreamChunk]struct{}),
	}
}

func (r *ActiveRun) AddSubscriber(ch chan bichatservices.StreamChunk) {
	r.Mu.Lock()
	defer r.Mu.Unlock()
	if r.subscribers == nil {
		r.subscribers = make(map[chan bichatservices.StreamChunk]struct{})
	}
	r.subscribers[ch] = struct{}{}
}

func (r *ActiveRun) RemoveSubscriber(ch chan bichatservices.StreamChunk) {
	r.Mu.Lock()
	defer r.Mu.Unlock()
	delete(r.subscribers, ch)
}

func (r *ActiveRun) SubscriberCount() int {
	r.Mu.RLock()
	defer r.Mu.RUnlock()
	return len(r.subscribers)
}

func (r *ActiveRun) Broadcast(chunk bichatservices.StreamChunk) {
	r.Mu.RLock()
	mirror := r.mirrorFn
	for ch := range r.subscribers {
		select {
		case ch <- chunk:
		default:
		}
	}
	r.Mu.RUnlock()
	if mirror != nil {
		mirror(chunk)
	}
}

// SetMirror installs a side-effect hook invoked after every Broadcast.
// Passing nil clears the hook. Safe to call concurrently with Broadcast
// — the hook is captured under the same RLock that the broadcast uses.
func (r *ActiveRun) SetMirror(fn func(bichatservices.StreamChunk)) {
	r.Mu.Lock()
	r.mirrorFn = fn
	r.Mu.Unlock()
}

func (r *ActiveRun) CloseAllSubscribers() {
	r.Mu.Lock()
	defer r.Mu.Unlock()
	for ch := range r.subscribers {
		close(ch)
	}
	r.subscribers = nil
}

func (r *ActiveRun) SnapshotMetadata() map[string]any {
	r.Mu.RLock()
	defer r.Mu.RUnlock()
	ordered := orderedToolCalls(r.ToolCalls, r.ToolOrder)
	meta := map[string]any{"tool_calls": ordered}
	if len(r.TextBlockOffsets) > 0 {
		offsets := make([]int, len(r.TextBlockOffsets))
		copy(offsets, r.TextBlockOffsets)
		meta["text_block_offsets"] = offsets
	}
	return meta
}

func orderedToolCalls(toolCalls map[string]types.ToolCall, toolOrder []string) []types.ToolCall {
	if len(toolOrder) == 0 {
		return nil
	}

	result := make([]types.ToolCall, 0, len(toolOrder))
	for _, key := range toolOrder {
		call, ok := toolCalls[key]
		if !ok {
			continue
		}
		result = append(result, call)
	}

	return result
}

// RunRegistry maps session and run ID to active runs.
type RunRegistry struct {
	mu        sync.RWMutex
	bySession map[uuid.UUID]*ActiveRun
	byRun     map[uuid.UUID]*ActiveRun
}

func NewRunRegistry() *RunRegistry {
	return &RunRegistry{
		bySession: make(map[uuid.UUID]*ActiveRun),
		byRun:     make(map[uuid.UUID]*ActiveRun),
	}
}

func (reg *RunRegistry) Add(run *ActiveRun) {
	reg.mu.Lock()
	defer reg.mu.Unlock()
	if previous, ok := reg.bySession[run.SessionID]; ok {
		previous.Cancel()
		previous.CloseAllSubscribers()
		delete(reg.byRun, previous.RunID)
	}
	reg.bySession[run.SessionID] = run
	reg.byRun[run.RunID] = run
}

func (reg *RunRegistry) Remove(runID uuid.UUID) {
	reg.mu.Lock()
	defer reg.mu.Unlock()
	if r, ok := reg.byRun[runID]; ok {
		delete(reg.bySession, r.SessionID)
		delete(reg.byRun, runID)
	}
}

func (reg *RunRegistry) GetBySession(sessionID uuid.UUID) *ActiveRun {
	reg.mu.RLock()
	defer reg.mu.RUnlock()
	return reg.bySession[sessionID]
}

func (reg *RunRegistry) GetByRun(runID uuid.UUID) *ActiveRun {
	reg.mu.RLock()
	defer reg.mu.RUnlock()
	return reg.byRun[runID]
}

func (reg *RunRegistry) ActiveRuns() int {
	reg.mu.RLock()
	defer reg.mu.RUnlock()
	return len(reg.byRun)
}

func (reg *RunRegistry) ActiveSubscribers() int {
	reg.mu.RLock()
	defer reg.mu.RUnlock()

	total := 0
	for _, run := range reg.byRun {
		total += run.SubscriberCount()
	}

	return total
}

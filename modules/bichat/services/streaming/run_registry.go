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

	subscribers map[chan bichatservices.StreamChunk]struct{}
	Mu          sync.RWMutex
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

func (r *ActiveRun) Broadcast(chunk bichatservices.StreamChunk) {
	r.Mu.RLock()
	defer r.Mu.RUnlock()
	for ch := range r.subscribers {
		select {
		case ch <- chunk:
		default:
		}
	}
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
	return map[string]any{"tool_calls": ordered}
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

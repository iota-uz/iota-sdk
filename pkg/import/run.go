// Package importpkg provides this package.
package importpkg

import (
	"context"
	"sort"
	"sync"
	"time"
)

// RunStatus describes the lifecycle state of an import run.
type RunStatus string

const (
	// RunQueued means the run has been created but not yet started.
	RunQueued RunStatus = "queued"
	// RunRunning means the run is actively executing.
	RunRunning RunStatus = "running"
	// RunDone means the run finished successfully.
	RunDone RunStatus = "done"
	// RunFailed means the run terminated with an error.
	RunFailed RunStatus = "failed"
)

// RunState is an immutable snapshot of an import run's progress and outcome.
// It is always copied in and out of a RunStore, so it carries no exported
// synchronisation primitives and is safe to pass by value.
type RunState struct {
	// ID uniquely identifies the run.
	ID string
	// Phase is the current human-readable stage label.
	Phase string
	// Done is the number of completed work units.
	Done int
	// Total is the expected number of work units (0 when unknown).
	Total int
	// Status is the current lifecycle status.
	Status RunStatus
	// Result holds the processor result once the run is done. It is nil until
	// the run completes successfully.
	Result *ImportResult
	// Err holds the error message when Status is RunFailed.
	Err string
	// StartedAt is the time the run transitioned to RunRunning.
	StartedAt time.Time
	// FinishedAt is the time the run reached a terminal status.
	FinishedAt time.Time
}

// RunStore persists RunState snapshots keyed by run id. Implementations must be
// safe for concurrent use.
type RunStore interface {
	// Create registers a new run with the given id in the RunQueued state and
	// returns its initial snapshot.
	Create(id string) *RunState
	// Get returns the current snapshot for id and whether it exists.
	Get(id string) (RunState, bool)
	// Update applies mutate to the stored state for id under the store's lock.
	// It is a no-op if id is unknown.
	Update(id string, mutate func(*RunState))
}

// DefaultRetentionCap is the default maximum number of terminal (RunDone or
// RunFailed) runs a MemoryRunStore retains. Older terminal runs beyond this
// cap are evicted oldest-first on each mutation.
const DefaultRetentionCap = 50

// DefaultRetentionTTL is the default maximum age of a terminal run before it
// becomes eligible for eviction, measured from its FinishedAt time.
const DefaultRetentionTTL = 30 * time.Minute

// MemoryRunStore is an in-memory, thread-safe RunStore.
//
// Retention: state lives in process memory only — it is not shared across
// processes/instances and is lost on restart. To bound memory growth, the store
// prunes terminal runs (RunDone or RunFailed) opportunistically on every
// Create/Update under the store lock, oldest-first by FinishedAt:
//
//   - A terminal run is evicted once its age (now - FinishedAt) exceeds the TTL.
//   - When the number of terminal runs still exceeds the cap, the oldest are
//     evicted until the count is within the cap.
//
// Non-terminal runs (RunQueued or RunRunning) are never evicted regardless of
// cap or TTL. The TTL sweep is skipped when the cap alone is satisfied, so a
// pure size cap is deterministic and does not depend on wall-clock time.
//
// Suitable for single-process deployments; back it with shared storage for
// horizontally scaled setups.
type MemoryRunStore struct {
	mu    sync.RWMutex
	state map[string]*RunState
	cap   int
	ttl   time.Duration
	now   func() time.Time // injectable clock for tests
}

// NewMemoryRunStore creates an empty in-memory RunStore using
// DefaultRetentionCap and DefaultRetentionTTL.
func NewMemoryRunStore() *MemoryRunStore {
	return NewMemoryRunStoreWithCap(DefaultRetentionCap)
}

// NewMemoryRunStoreWithCap creates an empty in-memory RunStore retaining at most
// n terminal runs (oldest evicted first) and using DefaultRetentionTTL. A
// non-positive n falls back to DefaultRetentionCap.
func NewMemoryRunStoreWithCap(n int) *MemoryRunStore {
	if n <= 0 {
		n = DefaultRetentionCap
	}
	return &MemoryRunStore{
		state: make(map[string]*RunState),
		cap:   n,
		ttl:   DefaultRetentionTTL,
		now:   time.Now,
	}
}

// isTerminal reports whether a status is a terminal (evictable) state.
func isTerminal(status RunStatus) bool {
	return status == RunDone || status == RunFailed
}

// prune evicts terminal runs that exceed the TTL and/or the cap, oldest-first by
// FinishedAt. Non-terminal runs are never evicted. Callers must hold s.mu.
func (s *MemoryRunStore) prune() {
	// Collect terminal runs (the only eviction candidates).
	terminal := make([]*RunState, 0, len(s.state))
	for _, st := range s.state {
		if isTerminal(st.Status) {
			terminal = append(terminal, st)
		}
	}
	if len(terminal) <= s.cap {
		// Within cap: skip the TTL sweep entirely so behavior is deterministic
		// and does not depend on wall-clock time.
		return
	}

	// Oldest-first by FinishedAt.
	sort.Slice(terminal, func(i, j int) bool {
		return terminal[i].FinishedAt.Before(terminal[j].FinishedAt)
	})

	// Evict by TTL first (oldest, expired entries), then by cap.
	now := s.now()
	excess := len(terminal) - s.cap
	for _, st := range terminal {
		expired := s.ttl > 0 && !st.FinishedAt.IsZero() && now.Sub(st.FinishedAt) > s.ttl
		if excess <= 0 && !expired {
			break
		}
		delete(s.state, st.ID)
		if excess > 0 {
			excess--
		}
	}
}

// Create registers a new queued run and returns a snapshot of it.
func (s *MemoryRunStore) Create(id string) *RunState {
	s.mu.Lock()
	defer s.mu.Unlock()
	st := &RunState{ID: id, Status: RunQueued}
	s.state[id] = st
	s.prune()
	snapshot := *st
	return &snapshot
}

// Get returns a snapshot of the run with the given id.
func (s *MemoryRunStore) Get(id string) (RunState, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	st, ok := s.state[id]
	if !ok {
		return RunState{}, false
	}
	return *st, true
}

// Update mutates the stored run under lock. Unknown ids are ignored.
func (s *MemoryRunStore) Update(id string, mutate func(*RunState)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	st, ok := s.state[id]
	if !ok {
		return
	}
	mutate(st)
	s.prune()
}

// storeSink is a ProgressSink that writes updates through a RunStore.
type storeSink struct {
	store RunStore
	id    string
}

func (s *storeSink) SetPhase(phase string) {
	s.store.Update(s.id, func(st *RunState) { st.Phase = phase })
}

func (s *storeSink) SetTotal(n int) {
	s.store.Update(s.id, func(st *RunState) { st.Total = n })
}

func (s *storeSink) SetDone(n int) {
	s.store.Update(s.id, func(st *RunState) { st.Done = n })
}

func (s *storeSink) Add(n int) {
	s.store.Update(s.id, func(st *RunState) { st.Done += n })
}

// ImportRunner orchestrates asynchronous BulkImportProcessor runs, tracking
// their progress in a RunStore.
type ImportRunner struct {
	store RunStore
}

// NewImportRunner creates a runner backed by the given store.
func NewImportRunner(store RunStore) *ImportRunner {
	return &ImportRunner{store: store}
}

// Start launches proc.Process for the run with the given id in a new goroutine
// and returns immediately. The run must already exist in the store (via
// Create); if it does not, Start creates it.
//
// The supplied ctx governs the spawned goroutine, so callers must pass a
// detached context (not a request context) to avoid cancellation when the HTTP
// handler returns. Progress is written to the store via a ProgressSink, and the
// terminal status (RunDone or RunFailed) plus Result/Err and FinishedAt are
// recorded when proc.Process returns.
func (r *ImportRunner) Start(ctx context.Context, id string, proc BulkImportProcessor, filePath string, opts ImportRunOptions) {
	if _, ok := r.store.Get(id); !ok {
		r.store.Create(id)
	}
	r.store.Update(id, func(st *RunState) {
		st.Status = RunRunning
		st.StartedAt = time.Now()
	})

	go func() {
		sink := &storeSink{store: r.store, id: id}
		result, err := proc.Process(ctx, filePath, opts, sink)
		r.store.Update(id, func(st *RunState) {
			st.FinishedAt = time.Now()
			if err != nil {
				st.Status = RunFailed
				st.Err = err.Error()
				return
			}
			res := result
			st.Result = &res
			st.Status = RunDone
			if st.Total > 0 {
				st.Done = st.Total
			}
		})
	}()
}

// Get returns the current snapshot for the run with the given id.
func (r *ImportRunner) Get(id string) (RunState, bool) {
	return r.store.Get(id)
}

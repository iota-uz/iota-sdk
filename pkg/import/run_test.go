package importpkg

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeProcessor is a configurable BulkImportProcessor for tests.
type fakeProcessor struct {
	total    int
	phase    string
	result   ImportResult
	err      error
	started  chan struct{} // closed when Process begins
	release  chan struct{} // Process blocks until closed (if non-nil)
	startOne sync.Once
}

func (p *fakeProcessor) Process(_ context.Context, _ string, _ ImportRunOptions, sink ProgressSink) (ImportResult, error) {
	if p.started != nil {
		p.startOne.Do(func() { close(p.started) })
	}
	if p.phase != "" {
		sink.SetPhase(p.phase)
	}
	if p.total > 0 {
		sink.SetTotal(p.total)
		for i := 0; i < p.total; i++ {
			sink.Add(1)
		}
	}
	if p.release != nil {
		<-p.release
	}
	if p.err != nil {
		return ImportResult{}, p.err
	}
	return p.result, nil
}

func waitForStatus(t *testing.T, store RunStore, id string, want RunStatus) RunState {
	t.Helper()
	deadline := time.After(2 * time.Second)
	for {
		select {
		case <-deadline:
			st, _ := store.Get(id)
			t.Fatalf("timeout waiting for status %q, last=%+v", want, st)
			return st
		default:
			if st, ok := store.Get(id); ok && st.Status == want {
				return st
			}
			time.Sleep(time.Millisecond)
		}
	}
}

func TestMemoryRunStore(t *testing.T) {
	t.Parallel()

	t.Run("Create returns queued snapshot", func(t *testing.T) {
		t.Parallel()
		store := NewMemoryRunStore()
		st := store.Create("a")
		require.NotNil(t, st)
		assert.Equal(t, "a", st.ID)
		assert.Equal(t, RunQueued, st.Status)
	})

	t.Run("Get missing id", func(t *testing.T) {
		t.Parallel()
		store := NewMemoryRunStore()
		_, ok := store.Get("missing")
		assert.False(t, ok)
	})

	t.Run("Update mutates and Get reflects copy", func(t *testing.T) {
		t.Parallel()
		store := NewMemoryRunStore()
		store.Create("b")
		store.Update("b", func(s *RunState) { s.Done = 7; s.Phase = "saving" })
		got, ok := store.Get("b")
		require.True(t, ok)
		assert.Equal(t, 7, got.Done)
		assert.Equal(t, "saving", got.Phase)
		// Mutating the returned snapshot must not affect the store.
		got.Done = 99
		again, _ := store.Get("b")
		assert.Equal(t, 7, again.Done)
	})

	t.Run("Update unknown id is no-op", func(t *testing.T) {
		t.Parallel()
		store := NewMemoryRunStore()
		assert.NotPanics(t, func() {
			store.Update("nope", func(s *RunState) { s.Done = 1 })
		})
	})
}

// finishedRun creates a terminal run and stamps a deterministic FinishedAt so
// eviction order (oldest-first) is predictable.
func finishedRun(store *MemoryRunStore, id string, finishedAt time.Time) {
	store.Create(id)
	store.Update(id, func(s *RunState) {
		s.Status = RunDone
		s.FinishedAt = finishedAt
	})
}

func TestMemoryRunStoreRetention(t *testing.T) {
	t.Parallel()

	t.Run("terminal runs beyond cap evicted oldest-first", func(t *testing.T) {
		t.Parallel()
		store := NewMemoryRunStoreWithCap(2)
		base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
		// Fix the clock so TTL never fires; this isolates the size-cap behavior.
		store.now = func() time.Time { return base.Add(5 * time.Minute) }
		// Insert 4 terminal runs with increasing FinishedAt; oldest two should go.
		finishedRun(store, "r1", base.Add(1*time.Minute))
		finishedRun(store, "r2", base.Add(2*time.Minute))
		finishedRun(store, "r3", base.Add(3*time.Minute))
		finishedRun(store, "r4", base.Add(4*time.Minute))

		_, ok1 := store.Get("r1")
		_, ok2 := store.Get("r2")
		_, ok3 := store.Get("r3")
		_, ok4 := store.Get("r4")
		assert.False(t, ok1, "oldest r1 should be evicted")
		assert.False(t, ok2, "r2 should be evicted")
		assert.True(t, ok3, "r3 within cap should be retained")
		assert.True(t, ok4, "newest r4 should be retained")
	})

	t.Run("running run never evicted even over cap", func(t *testing.T) {
		t.Parallel()
		store := NewMemoryRunStoreWithCap(1)
		base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
		store.now = func() time.Time { return base.Add(5 * time.Minute) }
		// A long-lived running run created first.
		store.Create("live")
		store.Update("live", func(s *RunState) { s.Status = RunRunning })
		// Pile on terminal runs beyond cap.
		finishedRun(store, "t1", base.Add(1*time.Minute))
		finishedRun(store, "t2", base.Add(2*time.Minute))
		finishedRun(store, "t3", base.Add(3*time.Minute))

		live, ok := store.Get("live")
		require.True(t, ok, "running run must never be evicted")
		assert.Equal(t, RunRunning, live.Status)
		// Only the newest terminal run survives under cap=1.
		_, okT1 := store.Get("t1")
		_, okT3 := store.Get("t3")
		assert.False(t, okT1, "oldest terminal evicted")
		assert.True(t, okT3, "newest terminal retained")
	})

	t.Run("TTL evicts expired terminal runs when over cap", func(t *testing.T) {
		t.Parallel()
		store := NewMemoryRunStoreWithCap(2)
		fixed := time.Date(2026, 1, 1, 1, 0, 0, 0, time.UTC)
		store.now = func() time.Time { return fixed }
		// r_old finished 31m ago (expired), others recent.
		finishedRun(store, "old", fixed.Add(-31*time.Minute))
		finishedRun(store, "mid", fixed.Add(-5*time.Minute))
		finishedRun(store, "new", fixed.Add(-1*time.Minute))

		// cap=2 with 3 terminal -> over cap; sweep runs. "old" is both excess
		// and TTL-expired, so it is evicted; the two recent ones remain.
		_, okOld := store.Get("old")
		_, okMid := store.Get("mid")
		_, okNew := store.Get("new")
		assert.False(t, okOld, "expired oldest evicted")
		assert.True(t, okMid)
		assert.True(t, okNew)
	})

	t.Run("under cap skips TTL sweep (deterministic)", func(t *testing.T) {
		t.Parallel()
		store := NewMemoryRunStoreWithCap(10)
		fixed := time.Date(2026, 1, 1, 1, 0, 0, 0, time.UTC)
		store.now = func() time.Time { return fixed }
		// Single expired terminal run, well under cap -> retained.
		finishedRun(store, "stale", fixed.Add(-2*time.Hour))
		_, ok := store.Get("stale")
		assert.True(t, ok, "under-cap expired run retained because TTL sweep is gated by cap")
	})
}

func TestImportRunner(t *testing.T) {
	t.Parallel()

	t.Run("success transitions queued->running->done and propagates progress", func(t *testing.T) {
		t.Parallel()
		store := NewMemoryRunStore()
		runner := NewImportRunner(store)
		created := store.Create("run1")
		require.Equal(t, RunQueued, created.Status)

		proc := &fakeProcessor{
			total: 3,
			phase: "saving",
			result: ImportResult{
				DryRun: true,
				Counts: []ImportCount{{Label: "imported", Value: 3}},
			},
		}
		runner.Start(context.Background(), "run1", proc, "/tmp/file.xlsx", ImportRunOptions{DryRun: true})

		st := waitForStatus(t, store, "run1", RunDone)
		assert.Equal(t, "saving", st.Phase)
		assert.Equal(t, 3, st.Total)
		assert.Equal(t, 3, st.Done)
		require.NotNil(t, st.Result)
		assert.True(t, st.Result.DryRun)
		require.Len(t, st.Result.Counts, 1)
		assert.Equal(t, 3, st.Result.Counts[0].Value)
		assert.False(t, st.StartedAt.IsZero())
		assert.False(t, st.FinishedAt.IsZero())
		assert.Empty(t, st.Err)
	})

	t.Run("failure sets RunFailed and Err", func(t *testing.T) {
		t.Parallel()
		store := NewMemoryRunStore()
		runner := NewImportRunner(store)
		proc := &fakeProcessor{err: errors.New("boom")}
		runner.Start(context.Background(), "run2", proc, "/tmp/f", ImportRunOptions{})

		st := waitForStatus(t, store, "run2", RunFailed)
		assert.Equal(t, "boom", st.Err)
		assert.Nil(t, st.Result)
		assert.False(t, st.FinishedAt.IsZero())
	})

	t.Run("Start creates run when absent", func(t *testing.T) {
		t.Parallel()
		store := NewMemoryRunStore()
		runner := NewImportRunner(store)
		proc := &fakeProcessor{result: ImportResult{}}
		runner.Start(context.Background(), "auto", proc, "/tmp/f", ImportRunOptions{})
		waitForStatus(t, store, "auto", RunDone)
		_, ok := runner.Get("auto")
		assert.True(t, ok)
	})

	t.Run("observes running before terminal", func(t *testing.T) {
		t.Parallel()
		store := NewMemoryRunStore()
		runner := NewImportRunner(store)
		proc := &fakeProcessor{
			started: make(chan struct{}),
			release: make(chan struct{}),
			result:  ImportResult{},
		}
		runner.Start(context.Background(), "run3", proc, "/tmp/f", ImportRunOptions{})
		<-proc.started
		st, ok := store.Get("run3")
		require.True(t, ok)
		assert.Equal(t, RunRunning, st.Status)
		assert.False(t, st.StartedAt.IsZero())
		close(proc.release)
		waitForStatus(t, store, "run3", RunDone)
	})
}

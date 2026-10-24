package jobs

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Store persists job snapshots and optional result bytes. Implementations
// must be safe for concurrent use and must evict terminal jobs (and their
// results) after the retention window — jobs are ephemeral by design.
type Store interface {
	// Create stores a new job.
	Create(ctx context.Context, j Job) error
	// Save overwrites an existing job snapshot and refreshes its TTL.
	Save(ctx context.Context, j Job) error
	// Get returns the job with the given id.
	Get(ctx context.Context, id uuid.UUID) (Job, bool, error)
	// ListByUser returns the user's jobs, newest first.
	ListByUser(ctx context.Context, tenantID uuid.UUID, userID uint, limit int) ([]Job, error)
	// Delete removes the job and its stored result.
	Delete(ctx context.Context, id uuid.UUID) error
	// SaveResult stores downloadable result bytes for the job.
	SaveResult(ctx context.Context, id uuid.UUID, name string, data []byte) error
	// GetResult returns the stored result bytes, if any.
	GetResult(ctx context.Context, id uuid.UUID) (name string, data []byte, ok bool, err error)
	// Touch refreshes the job's activity timestamp (heartbeat) and TTL.
	Touch(ctx context.Context, id uuid.UUID, at time.Time) error
}

const (
	// DefaultRetention is how long terminal jobs and their results live
	// after finishing.
	DefaultRetention = 24 * time.Hour
	// DefaultStoreCap bounds how many jobs per user the memory store keeps.
	DefaultStoreCap = 100
)

// memoryStore is an in-process Store used when Redis is not configured.
// Suitable for single-instance deployments; use RedisStore to share job
// state across replicas.
type memoryStore struct {
	mu        sync.RWMutex
	retention time.Duration
	cap       int
	now       func() time.Time
	jobs      map[uuid.UUID]Job
	results   map[uuid.UUID]storedResult
}

type storedResult struct {
	name string
	data []byte
}

// NewMemoryStore creates an in-process store retaining terminal jobs for the
// given duration. Non-positive values fall back to DefaultRetention.
func NewMemoryStore(retention time.Duration) Store {
	if retention <= 0 {
		retention = DefaultRetention
	}
	return &memoryStore{
		retention: retention,
		cap:       DefaultStoreCap,
		now:       time.Now,
		jobs:      make(map[uuid.UUID]Job),
		results:   make(map[uuid.UUID]storedResult),
	}
}

func (s *memoryStore) Create(ctx context.Context, j Job) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.jobs[j.ID] = cloneJob(j)
	s.prune(s.now())
	return nil
}

func (s *memoryStore) Save(ctx context.Context, j Job) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.jobs[j.ID] = cloneJob(j)
	s.prune(s.now())
	return nil
}

func (s *memoryStore) Get(ctx context.Context, id uuid.UUID) (Job, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	j, ok := s.jobs[id]
	if !ok {
		return Job{}, false, nil
	}
	return cloneJob(j), true, nil
}

func (s *memoryStore) ListByUser(ctx context.Context, tenantID uuid.UUID, userID uint, limit int) ([]Job, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	matched := make([]Job, 0)
	for _, j := range s.jobs {
		if j.OwnedBy(tenantID, userID) {
			matched = append(matched, j)
		}
	}
	sort.Slice(matched, func(a, b int) bool {
		return matched[a].CreatedAt.After(matched[b].CreatedAt)
	})
	if len(matched) > limit {
		matched = matched[:limit]
	}
	out := make([]Job, 0, len(matched))
	for _, j := range matched {
		out = append(out, cloneJob(j))
	}
	return out, nil
}

func (s *memoryStore) Delete(ctx context.Context, id uuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.jobs, id)
	delete(s.results, id)
	return nil
}

func (s *memoryStore) SaveResult(ctx context.Context, id uuid.UUID, name string, data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.results[id] = storedResult{name: name, data: append([]byte(nil), data...)}
	return nil
}

func (s *memoryStore) GetResult(ctx context.Context, id uuid.UUID) (string, []byte, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	res, ok := s.results[id]
	if !ok {
		return "", nil, false, nil
	}
	return res.name, append([]byte(nil), res.data...), true, nil
}

func (s *memoryStore) Touch(ctx context.Context, id uuid.UUID, at time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if j, ok := s.jobs[id]; ok {
		j.UpdatedAt = at
		s.jobs[id] = j
	}
	return nil
}

// prune evicts terminal jobs past the retention window, oldest first, keeps
// the total job count within the cap, and drops orphaned results. Callers
// must hold s.mu.
func (s *memoryStore) prune(now time.Time) {
	for id, j := range s.jobs {
		if j.Status.IsTerminal() && !j.FinishedAt.IsZero() && now.Sub(j.FinishedAt) > s.retention {
			delete(s.jobs, id)
			delete(s.results, id)
		}
	}
	for id := range s.results {
		if _, ok := s.jobs[id]; !ok {
			delete(s.results, id)
		}
	}
	if len(s.jobs) <= s.cap {
		return
	}
	ids := make([]uuid.UUID, 0, len(s.jobs))
	for id := range s.jobs {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(a, b int) bool {
		return s.jobs[ids[a]].CreatedAt.Before(s.jobs[ids[b]].CreatedAt)
	})
	for _, id := range ids[:len(s.jobs)-s.cap] {
		delete(s.jobs, id)
		delete(s.results, id)
	}
}

func cloneJob(j Job) Job {
	cp := j
	if j.Params != nil {
		cp.Params = make(map[string]any, len(j.Params))
		for k, v := range j.Params {
			cp.Params[k] = v
		}
	}
	return cp
}

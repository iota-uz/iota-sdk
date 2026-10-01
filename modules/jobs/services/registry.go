// Package services provides this package.
package services

import (
	"context"
	"fmt"
	"sort"
	"sync"
)

// ProgressReporter lets a job handler publish progress without knowing how it
// is persisted. Percent-based reporting covers bulk actions; total/done based
// reporting covers exports that know their row counts.
type ProgressReporter interface {
	// SetTotal declares the expected number of work units (0 when unknown).
	SetTotal(total int)
	// SetDone declares how many work units are complete.
	SetDone(done int)
	// Add increments the completed work units by n.
	Add(n int)
	// SetPercent overrides computed progress with an explicit percent (0-100).
	SetPercent(percent int)
	// SetPhase records a short human-readable phase label (e.g. "Writing rows").
	SetPhase(label string)
}

// Result is what a job handler returns. When Data is non-empty the worker
// stores it as a file via the upload service and the job gets a download link.
type Result struct {
	FileName string
	Data     []byte
}

// HandlerFunc executes one job. The ctx is tenant-scoped and carries the DB
// pool; params are the enqueue-time parameters.
type HandlerFunc func(ctx context.Context, params map[string]any, progress ProgressReporter) (Result, error)

// Registry maps job kinds to handlers. It is safe for concurrent use.
type Registry struct {
	mu       sync.RWMutex
	handlers map[string]HandlerFunc
}

func NewRegistry() *Registry {
	return &Registry{
		handlers: make(map[string]HandlerFunc),
	}
}

// Register associates kind with handler. It panics on an empty kind, a nil
// handler, or a duplicate registration — all programming errors that must
// surface at startup.
func (r *Registry) Register(kind string, handler HandlerFunc) {
	if kind == "" {
		panic("jobs: Register called with empty kind")
	}
	if handler == nil {
		panic(fmt.Sprintf("jobs: Register(%q) called with nil handler", kind))
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.handlers[kind]; exists {
		panic(fmt.Sprintf("jobs: kind %q registered twice", kind))
	}
	r.handlers[kind] = handler
}

// Get returns the handler registered for kind.
func (r *Registry) Get(kind string) (HandlerFunc, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	handler, ok := r.handlers[kind]
	return handler, ok
}

// Kinds returns the registered kinds sorted alphabetically.
func (r *Registry) Kinds() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	kinds := make([]string, 0, len(r.handlers))
	for kind := range r.handlers {
		kinds = append(kinds, kind)
	}
	sort.Strings(kinds)
	return kinds
}

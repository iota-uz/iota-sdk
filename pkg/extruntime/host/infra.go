package host

import (
	"context"
	"errors"
	"io"
	"net"
	"strings"
	"sync"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/pool"
)

// InfraError marks a failure of the platform rather than of the extension:
// the database or its connection failed, the worker was lost, the pool is
// closed or saturated. Callers that retry on a schedule (the workflow engine,
// the event dispatcher, the scheduler) retry it without counting it against
// the extension and without recording it as an author-visible failure.
type InfraError struct{ Err error }

func (e *InfraError) Error() string { return "extruntime: infrastructure failure: " + e.Err.Error() }
func (e *InfraError) Unwrap() error { return e.Err }

// IsInfrastructure reports whether err is a platform failure: an *InfraError,
// a lost worker, a closed or saturated pool, or a database connection /
// resource / serialization error. A cancelled or expired context is not (see
// below), and neither is a *pool.WireError: that is what the extension (or the
// engine on its behalf) raised on purpose.
func IsInfrastructure(err error) bool {
	if err == nil {
		return false
	}
	var ie *InfraError
	if errors.As(err, &ie) {
		return true
	}
	var we *pool.WireError
	if errors.As(err, &we) {
		return false
	}
	switch {
	case errors.Is(err, pool.ErrWorkerLost), errors.Is(err, pool.ErrPoolClosed), errors.Is(err, pool.ErrQueueFull),
		errors.Is(err, io.EOF), errors.Is(err, io.ErrUnexpectedEOF), errors.Is(err, net.ErrClosed):
		return true
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		// A cancelled or expired context is the caller's decision (a run
		// timeout, a termination, a handler that returned while its tx.call was
		// still running), not a failure of the platform; callers that care check
		// their own context.
		return false
	}
	var ne net.Error
	if errors.As(err, &ne) {
		return true
	}
	var pe *pgconn.PgError
	if errors.As(err, &pe) {
		return transientSQLState(pe.Code)
	}
	if pgconn.SafeToRetry(err) || (pgconn.Timeout(err) && !errors.Is(err, context.DeadlineExceeded)) {
		return true
	}
	var ce *pgconn.ConnectError
	if errors.As(err, &ce) {
		return true
	}
	// The pgx pool reports a closed pool and a lost connection with plain
	// errors that carry no type.
	msg := err.Error()
	return strings.Contains(msg, "closed pool") || strings.Contains(msg, "conn closed") || strings.Contains(msg, "conn is closed")
}

// transientSQLState is true for connection exceptions (08), insufficient
// resources (53), operator intervention (57: shutdown, cancelled query),
// system errors (58), serialization failures and deadlocks (40) and a lock
// that could not be taken (55P03).
func transientSQLState(code string) bool {
	if len(code) < 2 {
		return false
	}
	switch code[:2] {
	case "08", "53", "57", "58", "40":
		return true
	}
	return code == "55P03"
}

// infraState remembers the first infrastructure failure of a root invocation.
type infraState struct {
	mu  sync.Mutex
	err error
}

func (s *infraState) note(err error) {
	s.mu.Lock()
	if s.err == nil {
		s.err = err
	}
	s.mu.Unlock()
}

func (s *infraState) get() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.err
}

// withInfra turns the error of a run that hit an infrastructure failure into
// an *InfraError, whatever the extension made of it (a caught-and-rethrown
// host error looks like a business error from the outside).
func (r *rootState) withInfra(err error) error {
	if err == nil || IsInfrastructure(err) {
		return err
	}
	if cause := r.infra.get(); cause != nil {
		return &InfraError{Err: cause}
	}
	return err
}

package state

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/google/uuid"

	"github.com/iota-uz/iota-sdk/pkg/automation/expr"
)

// Reader is what the CEL variable `state` reads through: the live keys of one automation, looked up one at
// a time when an expression asks for them, so a step reads only the keys it needs. A key read once keeps the
// value it had within the reader (one step), so an expression that reads it twice sees one value.
type Reader struct {
	ctx        context.Context //nolint:containedctx // the reader lives for the evaluation of one step
	db         Querier
	tenant     uuid.UUID
	automation uuid.UUID

	mu    sync.Mutex
	cache map[string]found
	err   error // the first failure of the database
}

type found struct {
	value any
	ok    bool
}

var _ expr.Lookup = (*Reader)(nil)

// NewReader reads the memory of an automation through db: the transaction of the step, or a pool.
func NewReader(ctx context.Context, db Querier, tenant, automation uuid.UUID) *Reader {
	return &Reader{ctx: ctx, db: db, tenant: tenant, automation: automation, cache: map[string]found{}}
}

// Lookup is the value of a key, decoded as the format reads JSON (numbers as json.Number); ok is false for
// a key that is absent or expired.
func (r *Reader) Lookup(key string) (any, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if f, hit := r.cache[key]; hit {
		return f.value, f.ok, nil
	}
	if ValidateKey(key) != nil {
		// Nothing is stored under such a key.
		return nil, false, nil
	}
	e, ok, err := Get(r.ctx, r.db, r.tenant, r.automation, key)
	if err != nil {
		if r.err == nil {
			r.err = err
		}
		return nil, false, err
	}
	var v any
	if ok {
		if err := expr.UnmarshalJSON(e.Value, &v); err != nil {
			return nil, false, err
		}
	}
	r.cache[key] = found{value: v, ok: ok}
	return v, ok, nil
}

// Err is the first failure of the database a lookup met. The expression that read the key failed with its
// text, but only the caller can tell a platform failure (to retry the step) from the expression's own.
func (r *Reader) Err() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.err
}

// Snapshot is what every key read so far looked like: whether it was there and its value. A decision made
// over the memory (a preparation of an effect) keeps it to tell later whether it still holds.
func (r *Reader) Snapshot() map[string]string {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.cache) == 0 {
		return nil
	}
	out := make(map[string]string, len(r.cache))
	for key, f := range r.cache {
		out[key] = fingerprint(f.value, f.ok)
	}
	return out
}

// Same reports that every key of a snapshot reads as it did: through r, which reads the keys it has not read
// yet. An empty snapshot is the same for any memory, a nil Reader included.
func (r *Reader) Same(snapshot map[string]string) bool {
	if len(snapshot) == 0 {
		return true
	}
	if r == nil {
		return false
	}
	for key, was := range snapshot {
		v, ok, err := r.Lookup(key)
		if err != nil || fingerprint(v, ok) != was {
			return false
		}
	}
	return true
}

func fingerprint(value any, ok bool) string {
	if !ok {
		return "-"
	}
	b, err := json.Marshal(value) // keys of objects are written in order
	if err != nil {
		return "?"
	}
	return "+" + string(b)
}

// Empty is the memory of a context that has none (a trigger's, an automation not yet saved): no key is there.
type Empty struct{}

var _ expr.Lookup = Empty{}

// Lookup finds nothing.
func (Empty) Lookup(string) (any, bool, error) { return nil, false, nil }

package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/iota-uz/iota-sdk/pkg/automation/state"
)

// Memory is the people's and agents' view of an automation's memory between runs (the keys set_state writes
// and state.get reads): a list with search, one key, and a change or a delete of a key. Reads need
// Automations.Read or Automations.Author, a change Automations.Author.
type Memory interface {
	// List is the live keys of a graph in key order, filtered by a substring of the key.
	List(ctx context.Context, a Actor, id uuid.UUID, f state.Filter) (state.Page, error)
	// Get is one key; ErrNotFound when it is absent or has expired.
	Get(ctx context.Context, a Actor, id uuid.UUID, key string) (state.Entry, error)
	// Set writes a key with a JSON value that expires after ttl (0: never). The limits of set_state hold:
	// a breach is ErrInvalidInput (the key, the value, the lifetime) or ErrLimitExceeded (the keys).
	Set(ctx context.Context, a Actor, id uuid.UUID, key string, value json.RawMessage, ttl time.Duration) (state.Entry, error)
	// Delete removes a key; ErrNotFound when it is absent or has expired.
	Delete(ctx context.Context, a Actor, id uuid.UUID, key string) error
}

// Memory is the API of the keys of a graph's memory.
func (r *Runtime) Memory() Memory { return &memory{r: r} }

type memory struct{ r *Runtime }

// db is the transaction the caller joined with WithTx (a guarded change's apply), else the pool.
func (m *memory) db(ctx context.Context) state.DB {
	if outer, ok := ctx.Value(joinedTxKey{}).(pgx.Tx); ok && outer != nil {
		return outer
	}
	return m.r.cfg.DB
}

func (m *memory) check(ctx context.Context, a Actor, id uuid.UUID, write bool) error {
	var err error
	if write {
		err = m.r.service.authorize(a, PermAuthor)
	} else {
		err = m.r.service.authorizeAny(a, PermRead, PermAuthor)
	}
	if err != nil {
		return err
	}
	_, err = loadAutomation(ctx, m.r.reader(ctx), a.TenantID, id, false)
	return err
}

func (m *memory) List(ctx context.Context, a Actor, id uuid.UUID, f state.Filter) (state.Page, error) {
	if err := m.check(ctx, a, id, false); err != nil {
		return state.Page{}, err
	}
	return state.List(ctx, m.db(ctx), a.TenantID, id, f)
}

func (m *memory) Get(ctx context.Context, a Actor, id uuid.UUID, key string) (state.Entry, error) {
	if err := m.check(ctx, a, id, false); err != nil {
		return state.Entry{}, err
	}
	if err := state.ValidateKey(key); err != nil {
		return state.Entry{}, refusal(err)
	}
	e, ok, err := state.Get(ctx, m.db(ctx), a.TenantID, id, key)
	if err != nil {
		return state.Entry{}, err
	}
	if !ok {
		return state.Entry{}, ErrNotFound
	}
	return e, nil
}

// write runs a change of the memory in one transaction with the graph's row locked: a Delete of the graph
// waits for it (or it for the Delete), so a deleted graph never gets a key.
func (m *memory) write(ctx context.Context, a Actor, id uuid.UUID, fn func(tx pgx.Tx) error) error {
	if err := m.r.service.authorize(a, PermAuthor); err != nil {
		return err
	}
	return m.r.service.inTx(ctx, func(tx pgx.Tx) error {
		if _, err := loadAutomation(ctx, tx, a.TenantID, id, true); err != nil {
			return err
		}
		return fn(tx)
	})
}

func (m *memory) Set(ctx context.Context, a Actor, id uuid.UUID, key string, value json.RawMessage, ttl time.Duration) (state.Entry, error) {
	if len(value) == 0 || !json.Valid(value) {
		return state.Entry{}, fmt.Errorf("%w: the value is not JSON", ErrInvalidInput)
	}
	var e state.Entry
	err := m.write(ctx, a, id, func(tx pgx.Tx) error {
		var err error
		e, err = state.Put(ctx, tx, a.TenantID, id, key, value, ttl, nil)
		return err
	})
	if err != nil {
		return state.Entry{}, refusal(err)
	}
	return e, nil
}

func (m *memory) Delete(ctx context.Context, a Actor, id uuid.UUID, key string) error {
	err := m.write(ctx, a, id, func(tx pgx.Tx) error {
		deleted, err := state.Delete(ctx, tx, a.TenantID, id, key)
		if err != nil {
			return err
		}
		if !deleted {
			return ErrNotFound
		}
		return nil
	})
	return refusal(err)
}

// refusal is a refusal of the memory as the engine's sentinel with its message.
func refusal(err error) error {
	var se *state.Error
	if !errors.As(err, &se) {
		return err
	}
	if se.Code == state.CodeLimitExceeded {
		return fmt.Errorf("%w: %s", ErrLimitExceeded, se.Message)
	}
	return fmt.Errorf("%w: %s (%s)", ErrInvalidInput, se.Message, se.Code)
}

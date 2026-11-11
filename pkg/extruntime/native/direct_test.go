package native_test

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/native"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/pool"
	"github.com/iota-uz/iota-sdk/pkg/itf"
)

type note struct {
	Text string `json:"text"`
}

type total struct {
	N int `json:"n"`
}

// repo is a module service the Go defaults resolve from their dependencies.
type repo struct {
	mu        sync.Mutex
	committed []string
}

func (r *repo) add(ctx context.Context, text string) (int, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return 0, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO native_notes (text) VALUES ($1)`, text); err != nil {
		return 0, err
	}
	var n int
	err = tx.QueryRow(ctx, `SELECT count(*) FROM native_notes`).Scan(&n)
	return n, err
}

func directSet(t *testing.T) (*native.Set, *native.Def[note, total]) {
	t.Helper()
	set := native.NewSet()
	add := native.Operation("acme.d.add", native.Spec{Permission: "Notes.Write"}, func(d native.Deps) (native.OperationFunc[note, total], error) {
		r, err := native.Use[*repo](d)
		if err != nil {
			return nil, err
		}
		return func(ctx context.Context, tx native.Tx, in note) (total, error) {
			n, err := r.add(ctx, in.Text)
			if err != nil {
				return total{}, err
			}
			text := in.Text
			return total{N: n}, tx.AfterCommit(func(context.Context) {
				r.mu.Lock()
				r.committed = append(r.committed, text)
				r.mu.Unlock()
			})
		}, nil
	})
	count := native.Query("acme.d.count", native.Spec{}, func(native.Deps) (native.QueryFunc[struct{}, total], error) {
		return func(ctx context.Context, q native.QueryContext, _ struct{}) (total, error) {
			var out total
			return out, q.DB().QueryRow(ctx, `SELECT count(*) FROM native_notes`).Scan(&out.N)
		}, nil
	})
	save := native.Command("acme.d.save", native.Spec{}, func(native.Deps) (native.CommandFunc[note, total], error) {
		return func(ctx context.Context, c native.CommandContext, in note) (total, error) {
			var out total
			err := c.Transaction(ctx, func(ctx context.Context, tx native.Tx) (err error) {
				if out, err = add.Call(ctx, tx, in); err != nil {
					return err
				}
				if _, err := tx.Call(ctx, "acme.d.save", nil); err == nil {
					return errors.New("tx.call of an ordinary command must be refused")
				}
				if _, err := c.Call(ctx, "acme.d.save", nil); err == nil {
					return errors.New("ctx.call inside a transaction must be refused")
				}
				if in.Text == "undo" {
					return errors.New("undo")
				}
				return nil
			})
			if err != nil {
				return out, err
			}
			if _, err := c.Query(ctx, "acme.d.missing", json.RawMessage(`{}`)); err == nil {
				return out, errors.New("a query outside the set must be unknown")
			}
			return out, nil
		}, nil
	})
	require.NoError(t, set.Register(add, count, save))
	return set, save
}

func TestDirectDispatcherRunsGoDefaultsWithTheHostsTransactionShape(t *testing.T) {
	h := itf.NewHarness(t, itf.HarnessConfig{
		Name:      "extruntime_native",
		Database:  itf.DatabaseConfig{Provisioning: itf.ProvisioningPerTestDatabase},
		Isolation: itf.IsolationConfig{Mode: itf.IsolationCommitted},
	})
	scope := h.Scope(t)
	_, err := scope.Pool.Exec(context.Background(), `CREATE TABLE native_notes (id bigserial PRIMARY KEY, text text NOT NULL)`)
	require.NoError(t, err)

	r := &repo{}
	set, save := directSet(t)
	allowed := map[string]bool{"Notes.Write": true}
	d := native.NewDirect(native.Bind(set, native.StaticDeps(r)), native.WithAuthorizer(func(_ context.Context, p string) bool { return allowed[p] }))
	ctx := composables.WithPool(context.Background(), scope.Pool)

	out, err := save.Dispatch(ctx, d, note{Text: "a"})
	require.NoError(t, err)
	require.Equal(t, total{N: 1}, out)
	require.Equal(t, []string{"a"}, r.committed, "after-commit callbacks run after the commit")

	_, err = save.Dispatch(ctx, d, note{Text: "undo"})
	require.EqualError(t, err, "undo")
	require.Equal(t, []string{"a"}, r.committed, "nothing committed, no callback")

	raw, err := d.Dispatch(ctx, "acme.d.count", json.RawMessage(`{}`))
	require.NoError(t, err)
	require.JSONEq(t, `{"n":1}`, string(raw), "the rolled-back write is gone")

	_, err = d.Dispatch(ctx, "acme.d.save", json.RawMessage(`{"text":1}`))
	we, ok := native.AsError(err)
	require.True(t, ok, "%v", err)
	require.Equal(t, pool.ErrKindValidation, we.Kind)
	require.Equal(t, 422, native.HTTPStatus(err))

	delete(allowed, "Notes.Write")
	_, err = save.Dispatch(ctx, d, note{Text: "b"})
	require.True(t, native.IsKind(err, pool.ErrKindDenied), "%v", err)
	require.Equal(t, 403, native.HTTPStatus(err))

	t.Run("a caller-owned transaction is joined and refuses after-commit callbacks", func(t *testing.T) {
		allowed["Notes.Write"] = true
		tx, err := scope.Pool.Begin(ctx)
		require.NoError(t, err)
		defer func() { _ = tx.Rollback(context.Background()) }()
		_, err = d.Dispatch(composables.WithTx(ctx, tx), "acme.d.add", json.RawMessage(`{"text":"joined"}`))
		require.Equal(t, "runtime.afterCommitUnavailable", mustWire(t, err).Code)
	})
}

func mustWire(t *testing.T, err error) *pool.WireError {
	t.Helper()
	we, ok := native.AsError(err)
	require.True(t, ok, "%v", err)
	return we
}

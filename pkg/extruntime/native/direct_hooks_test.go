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
	"github.com/iota-uz/iota-sdk/pkg/itf"
)

type hookNote struct {
	Text string `json:"text"`
}

// An operation that calls a service which announces with composables.AfterCommit.
func hookSet(t *testing.T, announced *[]string, mu *sync.Mutex) *native.Set {
	t.Helper()
	set := native.NewSet()
	announce := native.Operation("acme.h.announce", native.Spec{}, func(native.Deps) (native.OperationFunc[hookNote, hookNote], error) {
		return func(ctx context.Context, _ native.Tx, in hookNote) (hookNote, error) {
			if err := composables.RequireCommitHooks(ctx); err != nil {
				return hookNote{}, err
			}
			composables.AfterCommit(ctx, func(context.Context) {
				mu.Lock()
				*announced = append(*announced, in.Text)
				mu.Unlock()
			})
			return in, nil
		}, nil
	})
	require.NoError(t, set.Add(announce))
	return set
}

// A dispatch inside the caller's transaction joins it with the caller's commit
// hooks: the callbacks run when the caller commits, never when it rolls back.
func TestDirectKeepsTheCallersCommitHooksWhenItJoinsItsTransaction(t *testing.T) {
	h := itf.NewHarness(t, itf.HarnessConfig{
		Name:      "extruntime_native_hooks",
		Database:  itf.DatabaseConfig{Provisioning: itf.ProvisioningPerTestDatabase},
		Isolation: itf.IsolationConfig{Mode: itf.IsolationCommitted},
	})
	scope := h.Scope(t)
	var (
		mu        sync.Mutex
		announced []string
	)
	set := hookSet(t, &announced, &mu)
	d := native.NewDirect(native.Bind(set, native.StaticDeps(struct{}{})))
	ctx := composables.WithPool(context.Background(), scope.Pool)
	got := func() []string { mu.Lock(); defer mu.Unlock(); return append([]string(nil), announced...) }

	errLater := errors.New("the caller's later step failed")
	err := composables.InTx(ctx, func(txCtx context.Context) error {
		_, err := d.Dispatch(txCtx, "acme.h.announce", json.RawMessage(`{"text":"rolled back"}`))
		require.NoError(t, err)
		return errLater
	})
	require.ErrorIs(t, err, errLater)
	require.Empty(t, got())

	require.NoError(t, composables.InTx(ctx, func(txCtx context.Context) error {
		_, err := d.Dispatch(txCtx, "acme.h.announce", json.RawMessage(`{"text":"committed"}`))
		require.NoError(t, err)
		require.Empty(t, got(), "the callback waits for the caller's commit")
		return nil
	}))
	require.Equal(t, []string{"committed"}, got())

	// A dispatch that opens its own transaction runs the callbacks after its commit.
	_, err = d.Dispatch(ctx, "acme.h.announce", json.RawMessage(`{"text":"own"}`))
	require.NoError(t, err)
	require.Equal(t, []string{"committed", "own"}, got())
}

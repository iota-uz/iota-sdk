package composables_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/composables"
)

func TestAfterCommit_WaitsForTheOwnerAndDropsOnRollback(t *testing.T) {
	var ran []string

	// No hooks in ctx: nothing to wait for, the callback runs at once.
	composables.AfterCommit(context.Background(), func(context.Context) { ran = append(ran, "immediate") })
	require.Equal(t, []string{"immediate"}, ran)

	// With hooks the callbacks wait for the owner, and run in order, once.
	ctx, hooks := composables.WithCommitHooks(context.Background())
	composables.AfterCommit(ctx, func(context.Context) { ran = append(ran, "first") })
	composables.AfterCommit(ctx, func(context.Context) { panic("a failing announcement") })
	composables.AfterCommit(ctx, func(context.Context) { ran = append(ran, "third") })
	require.Equal(t, []string{"immediate"}, ran)
	hooks.Run(ctx)
	require.Equal(t, []string{"immediate", "first", "third"}, ran, "a panicking callback does not stop the others")
	hooks.Run(ctx)
	require.Len(t, ran, 3)

	// A rollback drops them.
	ctx, hooks = composables.WithCommitHooks(context.Background())
	composables.AfterCommit(ctx, func(context.Context) { ran = append(ran, "dropped") })
	hooks.Discard()
	hooks.Run(ctx)
	require.Len(t, ran, 3)
}

type fakeTx struct{ pgx.Tx }

func TestWithTx_KeepsTheHooksOfTheSameTransactionAndDropsThemForAnother(t *testing.T) {
	tx, other := &fakeTx{}, &fakeTx{}
	ctx, hooks := composables.WithCommitHooks(composables.WithTx(context.Background(), tx))
	require.Same(t, hooks, composables.UseCommitHooks(ctx))

	require.Same(t, hooks, composables.UseCommitHooks(composables.WithTx(ctx, tx)), "joining the same transaction keeps its hooks")
	require.Nil(t, composables.UseCommitHooks(composables.WithTx(ctx, other)), "another transaction has its own outcome")

	require.NoError(t, composables.RequireCommitHooks(ctx))
	require.NoError(t, composables.RequireCommitHooks(context.Background()), "no transaction: the service opens its own")
	require.ErrorIs(t, composables.RequireCommitHooks(composables.WithTx(ctx, other)), composables.ErrNoCommitHooks)
}

func TestInSavepoint_RefusesAParentWithoutHooksBeforeOpeningTheSavepoint(t *testing.T) {
	// fakeTx has no Begin: reaching it would panic, so the refusal comes first.
	ctx := composables.WithTx(context.Background(), &fakeTx{})
	called := false
	err := composables.InSavepoint(ctx, func(context.Context) error { called = true; return nil })
	require.ErrorIs(t, err, composables.ErrNoCommitHooks)
	require.False(t, called)
}

func TestWithoutCommitHooks(t *testing.T) {
	ctx, hooks := composables.WithCommitHooks(context.Background())
	require.Same(t, hooks, composables.UseCommitHooks(ctx))
	require.Nil(t, composables.UseCommitHooks(composables.WithoutCommitHooks(ctx)))
}

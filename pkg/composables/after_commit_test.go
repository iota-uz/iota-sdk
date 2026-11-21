package composables_test

import (
	"context"
	"testing"

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

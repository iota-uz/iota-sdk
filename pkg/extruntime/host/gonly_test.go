package host_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/host"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
)

// Without a Node pool the host runs native contracts through their Go
// defaults, and a package export fails as unavailable before it is recorded.
func TestHostWithoutPoolRunsNativesAndRefusesPackages(t *testing.T) {
	e, _ := newNativeEnv(t, func(c *host.Config) { c.Pool = nil })
	res, err := e.host.Invoke(context.Background(), e.req("acme.n.save", `{"label":"go"}`, "Acme.Save"))
	require.NoError(t, err)
	require.JSONEq(t, `{"seen":1}`, string(res.Output))

	e.publish("acme.a", "1.0.0", jsPrelude+`export const addLabel = async () => ({ seen: 0 });`)
	e.contract(manifest.Resource{ID: "acme.a.addLabel", Kind: manifest.ResOperation, Input: obj("label", str()), Output: obj("seen", num())}, "acme.a", "1.0.0", "addLabel")
	_, err = e.host.Invoke(context.Background(), e.req("acme.a.addLabel", `{"label":"pkg"}`))
	require.Equal(t, host.CodePackageUnavailable, asWire(t, err).Code)
	require.Empty(t, e.invocations("acme.a.addLabel"), "nothing is recorded for an export that cannot run")
	_, err = e.host.Invoke(context.Background(), e.req("acme.n.viaPkg", `{"label":"x"}`))
	require.Equal(t, host.CodePackageUnavailable, asWire(t, err).Code, "a nested package export is refused too")
	require.Equal(t, []string{"go"}, e.items())
}

// A pre-allocated invocation ID is recorded; a repeat returns the recorded
// outcome without running the contract again.
func TestHostInvocationIDIsIdempotent(t *testing.T) {
	e, n := newNativeEnv(t)
	id := uuid.NewString()
	req := e.req("acme.n.save", `{"label":"once"}`, "Acme.Save")
	req.InvocationID = id
	first, err := e.host.Invoke(context.Background(), req)
	require.NoError(t, err)
	require.Equal(t, id, first.InvocationID)
	again, err := e.host.Invoke(context.Background(), req)
	require.NoError(t, err)
	require.JSONEq(t, string(first.Output), string(again.Output))
	require.Equal(t, host.OutcomeOK, again.Outcome)
	require.Equal(t, 1, n.defaultRuns(), "the repeat did not run the command")
	require.Equal(t, []string{"once"}, e.items())

	// A failed invocation repeats its failure.
	req = e.req("acme.n.save", `{"label":"rollback"}`, "Acme.Save")
	req.InvocationID = uuid.NewString()
	_, err = e.host.Invoke(context.Background(), req)
	require.Error(t, err)
	_, err = e.host.Invoke(context.Background(), req)
	require.Error(t, err)
	require.Equal(t, 2, n.defaultRuns())

	// An invocation without an outcome is reported, not re-run.
	_, err = e.db.Exec(context.Background(), `UPDATE extruntime.invocations SET finished_at = NULL, outcome = 'unknown' WHERE id = $1::uuid`, id)
	require.NoError(t, err)
	req = e.req("acme.n.save", `{"label":"once"}`, "Acme.Save")
	req.InvocationID = id
	_, err = e.host.Invoke(context.Background(), req)
	require.Equal(t, host.CodeInvocationUnfinished, asWire(t, err).Code)
	require.Equal(t, 2, n.defaultRuns())
}

// An operation joins a caller-owned transaction: the caller commits (or
// rolls back) its writes and runs the after-commit callbacks. An ordinary
// command cannot join one.
func TestHostRunsAnOperationInTheCallersTransaction(t *testing.T) {
	e, n := newNativeEnv(t)
	ctx := context.Background()
	tx, err := e.db.Begin(ctx)
	require.NoError(t, err)
	req := e.req("acme.n.add", `{"label":"joined"}`)
	req.Tx = tx
	res, err := e.host.Invoke(ctx, req)
	require.NoError(t, err)
	require.Empty(t, e.items(), "nothing is visible before the caller commits")
	require.NoError(t, tx.Commit(ctx))
	require.NotNil(t, res.AfterCommit)
	res.AfterCommit(ctx)
	require.Equal(t, []string{"joined"}, e.items())
	require.Equal(t, []string{"joined"}, n.afterCommits())

	tx, err = e.db.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback(ctx) }()
	req = e.req("acme.n.save", `{"label":"no"}`, "Acme.Save")
	req.Tx = tx
	_, err = e.host.Invoke(ctx, req)
	require.Equal(t, host.CodeTxNotAllowed, asWire(t, err).Code)
}

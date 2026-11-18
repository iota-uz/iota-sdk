package host_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/pool"
)

// A tx.call nobody awaited must not half-commit: the caller's transaction
// cannot commit while the nested invocation is running or after it was cut
// short, so neither the caller's writes nor the nested call's first write
// survive.
func TestUnawaitedTxCallRollsTheTransactionBackInsteadOfHalfCommitting(t *testing.T) {
	e := newEnv(t)
	e.publish("acme.a", "1.0.0", jsPrelude+`
export const forget = async (tx) => {
  await tx.insert(item, { label: "outer" });
  void tx.call({ id: "acme.a.slowAdd" }, { label: "late" });
  await new Promise((resolve) => setTimeout(resolve, 150)); // the call is under way when the handler returns
  return {};
};
export const slowAdd = async (tx, input) => {
  await tx.insert(item, { label: input.label + "-1" });
  await new Promise((resolve) => setTimeout(resolve, 400));
  await tx.insert(item, { label: input.label + "-2" });
  return {};
};
export const awaited = async (tx) => {
  await tx.insert(item, { label: "outer" });
  await tx.call({ id: "acme.a.slowAdd" }, { label: "late" });
  return {};
};`)
	e.contract(command("acme.a.forget", true, nil, nil), "acme.a", "1.0.0", "forget")
	e.contract(command("acme.a.awaited", true, nil, nil), "acme.a", "1.0.0", "awaited")
	e.contract(manifest.Resource{ID: "acme.a.slowAdd", Kind: manifest.ResOperation, Input: obj("label", str())}, "acme.a", "1.0.0", "slowAdd")

	_, err := e.host.Invoke(context.Background(), e.req("acme.a.forget", `null`))
	we, ok := pool.AsWireError(err)
	require.True(t, ok, "%v", err)
	require.Equal(t, "runtime.nestedInFlight", we.Code)
	require.Empty(t, e.items(), "nothing of the caller or the un-awaited call committed")

	_, err = e.host.Invoke(context.Background(), e.req("acme.a.awaited", `null`))
	require.NoError(t, err)
	require.Equal(t, []string{"outer", "late-1", "late-2"}, e.items(), "an awaited tx.call commits with its caller")
}

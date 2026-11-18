package host_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/host"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/pool"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
)

const guardPrelude = jsPrelude + `
const reject = (code, path, params) => Object.assign(new Error(code), { kind: "validation", code, issues: [{ code, path, params }] });
`

func (e *env) addGuard(id, target string, phase manifest.GuardPhase, pkg, source string) {
	e.art.add(e.t, pkg, "1.0.0", guardPrelude+source)
	e.catalog.guards = append(e.catalog.guards, spi.ResolvedGuard{Guard: manifest.Guard{ID: id, Export: "check", Target: target, Phase: phase}, Package: pkg, Version: "1.0.0"})
}

func issuesOf(t *testing.T, err error) []pool.Issue {
	t.Helper()
	we, ok := pool.AsWireError(err)
	require.True(t, ok, "%v", err)
	require.Equal(t, pool.ErrKindValidation, we.Kind, "%v", err)
	return we.Issues
}

func TestPrecheckRunsBeforeAnOrdinaryCommandAndSurvivesReplacement(t *testing.T) {
	e := newEnv(t)
	impl := func(tag string) string {
		return jsPrelude + `export const renew = async (ctx, input) => ctx.transaction(async (tx) => ({ key: (await tx.insert(item, { label: "` + tag + `:" + input.label })).key }));`
	}
	e.publish("acme.a", "1.0.0", impl("a"))
	e.publish("acme.b", "1.0.0", impl("b"))
	e.contract(command("acme.a.renew", false, obj("label", str()), obj("key", str())), "acme.a", "1.0.0", "renew")
	e.addGuard("acme.g.tooLong", "acme.a.renew", manifest.GuardInput, "acme.g", `
export const check = async (q, input) => { if (input.label.length > 5) throw reject("acme.g.tooLong", ["label"], { max: 5 }); };`)

	_, err := e.host.Invoke(context.Background(), e.req("acme.a.renew", `{"label":"way too long"}`))
	issues := issuesOf(t, err)
	require.Len(t, issues, 1)
	require.Equal(t, "acme.g.tooLong", issues[0].Code)
	require.Equal(t, "acme.g.tooLong", issues[0].Source, "the issue source is the guard ID")
	require.Equal(t, "input", issues[0].Phase)
	require.JSONEq(t, `["label"]`, string(issues[0].Path))
	require.Empty(t, e.items(), "the implementation never ran")
	require.Empty(t, e.invocations("acme.a.renew"), "a rejected precheck is not an invocation of the command")

	res, err := e.host.Invoke(context.Background(), e.req("acme.a.renew", `{"label":"short"}`))
	require.NoError(t, err)
	require.JSONEq(t, `{"key":"1"}`, string(res.Output))

	// Replace the implementation by another package's: the guard still applies.
	_, err = e.bindings.Activate(context.Background(), "acme.a.renew", "acme.b", "1.0.0", "renew", nil)
	require.NoError(t, err)
	_, err = e.host.Invoke(context.Background(), e.req("acme.a.renew", `{"label":"way too long"}`))
	require.Equal(t, "acme.g.tooLong", issuesOf(t, err)[0].Source, "a replacement cannot bypass the guard")
	res, err = e.host.Invoke(context.Background(), e.req("acme.a.renew", `{"label":"ok"}`))
	require.NoError(t, err)
	require.Equal(t, "acme.b", res.Binding.Package)
	require.Equal(t, []string{"a:short", "b:ok"}, e.items())
}

func TestOperationGuardRunsInTheTransactionBeforeAnyImplementationAndAggregatesIssues(t *testing.T) {
	e := newEnv(t)
	impl := func(tag string) string {
		return jsPrelude + `export const add = async (tx, input) => { await tx.insert(item, { label: "` + tag + `:" + input.label }); return { count: await tx.count(item) }; };`
	}
	e.publish("acme.a", "1.0.0", impl("a"))
	e.publish("acme.b", "1.0.0", impl("b"))
	e.contract(command("acme.a.add", true, obj("label", str()), obj("count", num())), "acme.a", "1.0.0", "add")
	e.addGuard("acme.g.capacity", "acme.a.add", manifest.GuardOperation, "acme.g", `
export const check = async (tx, input) => { if ((await tx.count(item)) >= 1) throw reject("acme.g.full", [], { max: 1 }); };`)
	e.addGuard("acme.h.label", "acme.a.add", manifest.GuardOperation, "acme.h", `
export const check = async (tx, input) => { if (input.label === "bad") throw reject("acme.h.badLabel", ["label"], {}); };`)

	res, err := e.host.Invoke(context.Background(), e.req("acme.a.add", `{"label":"one"}`))
	require.NoError(t, err)
	require.JSONEq(t, `{"count":1}`, string(res.Output))

	_, err = e.host.Invoke(context.Background(), e.req("acme.a.add", `{"label":"bad"}`))
	issues := issuesOf(t, err)
	require.Len(t, issues, 2, "issues of every failing guard are reported together")
	require.Equal(t, []string{"acme.g.capacity", "acme.h.label"}, []string{issues[0].Source, issues[1].Source})
	require.Equal(t, "write", issues[0].Phase)

	_, err = e.bindings.Activate(context.Background(), "acme.a.add", "acme.b", "1.0.0", "add", nil)
	require.NoError(t, err)
	_, err = e.host.Invoke(context.Background(), e.req("acme.a.add", `{"label":"two"}`))
	require.Equal(t, "acme.g.capacity", issuesOf(t, err)[0].Source, "the replacement implementation cannot bypass the transactional guard")
	require.Equal(t, []string{"a:one"}, e.items(), "the replacement never ran")
}

func TestOperationGuardAlsoGuardsTheOperationCalledThroughTxCall(t *testing.T) {
	e := newEnv(t)
	e.publish("acme.a", "1.0.0", jsPrelude+`
export const bump = async (tx, input) => { await tx.insert(item, { label: input.label }); return {}; };
export const run = async (tx, input) => { await tx.call({ id: "acme.a.bump" }, { label: "first" }); await tx.call({ id: "acme.a.bump" }, { label: input.label }); return {}; };`)
	e.contract(manifest.Resource{ID: "acme.a.bump", Kind: manifest.ResOperation, Input: obj("label", str()), Output: obj()}, "acme.a", "1.0.0", "bump")
	e.contract(command("acme.a.run", true, obj("label", str()), obj()), "acme.a", "1.0.0", "run")
	e.addGuard("acme.g.noDup", "acme.a.bump", manifest.GuardOperation, "acme.g", `
export const check = async (tx, input) => {
  const same = (await tx.find(item)).filter((r) => r.label === input.label);
  if (same.length > 0) throw reject("acme.g.duplicate", ["label"], {});
};`)
	_, err := e.host.Invoke(context.Background(), e.req("acme.a.run", `{"label":"first"}`))
	dup := issuesOf(t, err)[0]
	require.Equal(t, "acme.g.duplicate", dup.Code)
	require.Equal(t, "acme.g.noDup", dup.Source)
	require.Empty(t, e.items(), "the whole atomic command rolled back")
	_, err = e.host.Invoke(context.Background(), e.req("acme.a.run", `{"label":"second"}`))
	require.NoError(t, err)
	require.Equal(t, []string{"first", "second"}, e.items())
}

func TestWriteRuleRunsInTheWritingTransactionAndSeesUncommittedRows(t *testing.T) {
	e := newEnv(t)
	e.publish("acme.a", "1.0.0", jsPrelude+`
export const fill = async (ctx, input) => ctx.transaction(async (tx) => {
  for (const label of input.labels) await tx.insert(item, { label });
  return { n: await tx.count(item) };
});`)
	e.contract(command("acme.a.fill", false, obj("labels", manifest.Type{Kind: manifest.KindArray, Items: &manifest.Type{Kind: manifest.KindString}}), obj("n", num())), "acme.a", "1.0.0", "fill")
	e.addGuard("acme.g.noBad", itemEntity, manifest.GuardWrite, "acme.g", `
export const check = async (tx, change) => {
  if (change.op !== "insert" || change.existing !== null || !change.changed.includes("label")) throw new Error("unexpected change " + JSON.stringify(change));
  if (change.proposed.label === "bad") throw reject("acme.g.badRow", ["label"], {});
  if ((await tx.count(item)) >= 2) throw reject("acme.g.tooMany", [], { max: 2 });
};`)

	res, err := e.host.Invoke(context.Background(), e.req("acme.a.fill", `{"labels":["a","b"]}`))
	require.NoError(t, err)
	require.JSONEq(t, `{"n":2}`, string(res.Output))

	_, err = e.host.Invoke(context.Background(), e.req("acme.a.fill", `{"labels":["c"]}`))
	issues := issuesOf(t, err)
	require.Equal(t, "acme.g.tooMany", issues[0].Code, "the rule counts rows committed before")
	require.Equal(t, "acme.g.noBad", issues[0].Source, "the source is the guard ID, not the issue code")
	_, err = e.host.Invoke(context.Background(), e.req("acme.a.fill", `{"labels":["bad"]}`))
	issues = issuesOf(t, err)
	require.Equal(t, "acme.g.badRow", issues[0].Code)
	require.Equal(t, "write", issues[0].Phase)
	require.Equal(t, []string{"a", "b"}, e.items())
	require.Zero(t, e.idleInTx())
}

func TestWriteRuleCheckedByTheHostWithoutAnInvocation(t *testing.T) {
	e := newEnv(t)
	e.addGuard("acme.g.noBad", itemEntity, manifest.GuardWrite, "acme.g", `
export const check = async (tx, change) => { if (change.proposed.label === "bad") throw reject("acme.g.badRow", ["label"], {}); };`)
	tx, err := e.db.Begin(context.Background())
	require.NoError(t, err)
	defer func() { _ = tx.Rollback(context.Background()) }()
	sc := spi.Scope{Subject: e.subject()}
	require.NoError(t, e.host.CheckWrite(context.Background(), tx, sc, spi.WriteChange{Entity: itemEntity, Op: "insert", Proposed: []byte(`{"label":"fine"}`), Changed: []string{"label"}}))
	err = e.host.CheckWrite(context.Background(), tx, sc, spi.WriteChange{Entity: itemEntity, Op: "insert", Proposed: []byte(`{"label":"bad"}`), Changed: []string{"label"}})
	require.Equal(t, "acme.g.noBad", issuesOf(t, err)[0].Source, "a native writer outside any invocation is guarded too")
	var _ host.WireErrorer
}

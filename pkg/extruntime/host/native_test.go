package host_test

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/data"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/host"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/native"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/pool"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

type labelIn struct {
	Label string `json:"label"`
}

type seenOut struct {
	Seen int `json:"seen"`
}

type countOut struct {
	N int `json:"n"`
}

// natives is a native contract set with Go defaults that write host_items
// through the transaction in their context, like a module repository.
type natives struct {
	set  *native.Set
	add  *native.Def[labelIn, seenOut]
	save *native.Def[labelIn, seenOut]

	mu        sync.Mutex
	committed []string
	defaults  int
}

func (n *natives) afterCommits() []string {
	n.mu.Lock()
	defer n.mu.Unlock()
	return append([]string(nil), n.committed...)
}

func (n *natives) defaultRuns() int {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.defaults
}

func newNatives(t *testing.T) *natives {
	t.Helper()
	n := &natives{set: native.NewSet()}
	n.add = native.Operation("acme.n.add", native.Spec{}, func(native.Deps) (native.OperationFunc[labelIn, seenOut], error) {
		return func(ctx context.Context, tx native.Tx, in labelIn) (seenOut, error) {
			if in.Label == "invalid" {
				return seenOut{}, native.Validation(native.Issue{Code: "acme.n.invalid", Path: []string{"label"}})
			}
			// The context carries the host transaction and the tenant, so
			// repositories join it.
			rtx, err := composables.UseTx(ctx)
			if err != nil {
				return seenOut{}, err
			}
			tenant, err := composables.UseTenantID(ctx)
			if err != nil {
				return seenOut{}, err
			}
			if _, err := rtx.Exec(ctx, `INSERT INTO host_items (tenant_id, label) VALUES ($1, $2)`, tenant, in.Label); err != nil {
				return seenOut{}, err
			}
			var seen int
			if err := tx.DB().QueryRow(ctx, `SELECT count(*) FROM host_items WHERE tenant_id = $1`, tenant).Scan(&seen); err != nil {
				return seenOut{}, err
			}
			label := in.Label
			if err := tx.AfterCommit(func(context.Context) {
				n.mu.Lock()
				n.committed = append(n.committed, label)
				n.mu.Unlock()
			}); err != nil {
				return seenOut{}, err
			}
			return seenOut{Seen: seen}, nil
		}, nil
	})
	n.save = native.Command("acme.n.save", native.Spec{Permission: "Acme.Save"}, func(native.Deps) (native.CommandFunc[labelIn, seenOut], error) {
		return func(ctx context.Context, c native.CommandContext, in labelIn) (seenOut, error) {
			n.mu.Lock()
			n.defaults++
			n.mu.Unlock()
			var out seenOut
			err := c.Transaction(ctx, func(ctx context.Context, tx native.Tx) (err error) {
				if out, err = n.add.Call(ctx, tx, in); err != nil {
					return err
				}
				if in.Label == "rollback" {
					return errors.New("the command changed its mind")
				}
				return nil
			})
			return out, err
		}, nil
	})
	count := native.Query("acme.n.count", native.Spec{}, func(native.Deps) (native.QueryFunc[struct{}, countOut], error) {
		return func(ctx context.Context, q native.QueryContext, _ struct{}) (countOut, error) {
			tenant, _ := composables.UseTenantID(ctx)
			var c countOut
			err := q.DB().QueryRow(ctx, `SELECT count(*) FROM host_items WHERE tenant_id = $1`, tenant).Scan(&c.N)
			return c, err
		}, nil
	})
	viaPkg := native.Operation("acme.n.viaPkg", native.Spec{}, func(native.Deps) (native.OperationFunc[labelIn, seenOut], error) {
		return func(ctx context.Context, tx native.Tx, in labelIn) (seenOut, error) {
			raw, err := tx.Call(ctx, "acme.a.addLabel", json.RawMessage(`{"label":"pkg:`+in.Label+`"}`))
			if err != nil {
				return seenOut{}, err
			}
			var out seenOut
			return out, json.Unmarshal(raw, &out)
		}, nil
	})
	loop := native.Operation("acme.n.loop", native.Spec{}, func(native.Deps) (native.OperationFunc[labelIn, seenOut], error) {
		var self func(ctx context.Context, tx native.Tx, in labelIn) (seenOut, error)
		self = func(ctx context.Context, tx native.Tx, in labelIn) (seenOut, error) {
			raw, err := tx.Call(ctx, "acme.n.loop", json.RawMessage(`{"label":"again"}`))
			if err != nil {
				return seenOut{}, err
			}
			var out seenOut
			return out, json.Unmarshal(raw, &out)
		}
		return self, nil
	})
	badOut := native.Operation("acme.n.badOutput", native.Spec{Output: obj("n", num())}, func(native.Deps) (native.OperationFunc[labelIn, seenOut], error) {
		return func(context.Context, native.Tx, labelIn) (seenOut, error) { return seenOut{Seen: 1}, nil }, nil
	})
	panics := native.Operation("acme.n.panic", native.Spec{}, func(native.Deps) (native.OperationFunc[labelIn, seenOut], error) {
		return func(context.Context, native.Tx, labelIn) (seenOut, error) { panic("bug") }, nil
	})
	require.NoError(t, n.set.Register(n.add, n.save, count, viaPkg, loop, badOut, panics))
	return n
}

func newNativeEnv(t *testing.T, opts ...envOption) (*env, *natives) {
	t.Helper()
	n := newNatives(t)
	impls := native.Bind(n.set, nil)
	e := newEnv(t, append([]envOption{func(c *host.Config) { c.Natives = impls }}, opts...)...)
	for _, r := range n.set.Resources() {
		e.catalog.res[r.ID] = spi.ResolvedResource{Resource: r, Native: true}
	}
	return e, n
}

func TestNativeCommandRunsItsGoDefaultThroughTheHost(t *testing.T) {
	e, n := newNativeEnv(t)

	res, err := e.host.Invoke(context.Background(), e.req("acme.n.save", `{"label":"x"}`, "Acme.Save"))
	require.NoError(t, err)
	require.JSONEq(t, `{"seen":1}`, string(res.Output))
	require.True(t, res.Binding.Native)
	require.Equal(t, 1, res.CommittedTxCount)
	require.Equal(t, []string{"x"}, e.items())
	require.Equal(t, []string{"x"}, n.afterCommits(), "after-commit callbacks run once the transaction committed")

	saves, adds := e.invocations("acme.n.save"), e.invocations("acme.n.add")
	require.Len(t, saves, 1)
	require.Len(t, adds, 1)
	require.Equal(t, "ok", saves[0].Outcome)
	require.Equal(t, 1, saves[0].Committed)
	require.Equal(t, []string{"", "", ""}, []string{saves[0].Package, saves[0].Version, saves[0].Export}, "the Go default is recorded without a package binding")
	require.Equal(t, saves[0].ID, adds[0].Parent, "the operation is a nested invocation of the command")
	require.Equal(t, saves[0].Root, adds[0].Root)

	t.Run("a rolled-back transaction writes nothing and runs no after-commit callback", func(t *testing.T) {
		_, err := e.host.Invoke(context.Background(), e.req("acme.n.save", `{"label":"rollback"}`, "Acme.Save"))
		require.ErrorContains(t, err, "changed its mind", "a Go default's own error keeps its chain for Go callers")
		require.Equal(t, []string{"x"}, e.items())
		require.Equal(t, []string{"x"}, n.afterCommits())
		require.Equal(t, "error", e.invocations("acme.n.save")[1].Outcome)
	})

	t.Run("the definition's RBAC permission is checked", func(t *testing.T) {
		_, err := e.host.Invoke(context.Background(), e.req("acme.n.save", `{"label":"y"}`))
		we, ok := pool.AsWireError(err)
		require.True(t, ok, "%v", err)
		require.Equal(t, pool.ErrKindDenied, we.Kind)
		require.Equal(t, "runtime.permissionDenied", we.Code)
	})

	t.Run("input and output are validated against the contract", func(t *testing.T) {
		_, err := e.host.Invoke(context.Background(), e.req("acme.n.save", `{"label":1}`, "Acme.Save"))
		require.Equal(t, "runtime.invalidValue", asWire(t, err).Code)
		_, err = e.host.Invoke(context.Background(), e.req("acme.n.badOutput", `{"label":"a"}`))
		require.Equal(t, "runtime.invalidOutput", asWire(t, err).Code)
		require.Equal(t, []string{"x"}, e.items(), "an invalid output rolls the operation back")
	})

	t.Run("a Go default's validation issues travel as validation errors", func(t *testing.T) {
		_, err := e.host.Invoke(context.Background(), e.req("acme.n.save", `{"label":"invalid"}`, "Acme.Save"))
		issues := issuesOf(t, err)
		require.Len(t, issues, 1)
		require.Equal(t, "acme.n.invalid", issues[0].Code)
		require.JSONEq(t, `["label"]`, string(issues[0].Path))
	})

	t.Run("a panicking Go default fails its invocation, not the process", func(t *testing.T) {
		_, err := e.host.Invoke(context.Background(), e.req("acme.n.panic", `{"label":"x"}`))
		require.Equal(t, "runtime.nativePanic", asWire(t, err).Code)
		require.Equal(t, "error", e.invocations("acme.n.panic")[0].Outcome)
	})

	t.Run("recursion is bounded by the nesting limit", func(t *testing.T) {
		_, err := e.host.Invoke(context.Background(), e.req("acme.n.loop", `{"label":"go"}`))
		require.Equal(t, "runtime.nestingTooDeep", asWire(t, err).Code)
	})
	require.Zero(t, e.idleInTx())
}

func asWire(t *testing.T, err error) *pool.WireError {
	t.Helper()
	we, ok := pool.AsWireError(err)
	require.True(t, ok, "%v", err)
	return we
}

func TestPackageCallsNativeOperationInItsTransactionAndNativeQueries(t *testing.T) {
	e, n := newNativeEnv(t)
	e.publish("acme.a", "1.0.0", jsPrelude+`
export const bulk = async (tx, input) => {
  await tx.insert(item, { label: "outer" });
  const r = await tx.call({ id: "acme.n.add" }, { label: "inner" });
  const q = await tx.query({ id: "acme.n.count" }, {});
  if (input.fail) throw domain("acme.a.boom", {});
  return { seen: r.seen, counted: q.n };
};
export const snapshot = async (ctx) => ({ n: (await ctx.query({ id: "acme.n.count" }, {})).n });`)
	e.contract(command("acme.a.bulk", true, obj("fail", boolean()), obj("seen", num(), "counted", num()), "acme.a.boom"), "acme.a", "1.0.0", "bulk")
	e.contract(command("acme.a.snapshot", false, nil, obj("n", num())), "acme.a", "1.0.0", "snapshot")

	_, err := e.host.Invoke(context.Background(), e.req("acme.a.bulk", `{"fail":true}`))
	require.Equal(t, "acme.a.boom", asWire(t, err).Code)
	require.Empty(t, e.items(), "the native write rolled back with its caller")
	require.Empty(t, n.afterCommits(), "nothing committed, no callback")

	res, err := e.host.Invoke(context.Background(), e.req("acme.a.bulk", `{"fail":false}`))
	require.NoError(t, err)
	require.JSONEq(t, `{"seen":2,"counted":2}`, string(res.Output), "the Go operation and the Go query saw the caller's uncommitted row")
	require.Equal(t, []string{"outer", "inner"}, e.items())
	require.Equal(t, []string{"inner"}, n.afterCommits(), "the callback waited for the caller's commit")

	res, err = e.host.Invoke(context.Background(), e.req("acme.a.snapshot", `null`))
	require.NoError(t, err)
	require.JSONEq(t, `{"n":2}`, string(res.Output), "q.query reaches a Go query")

	bulk := e.invocations("acme.a.bulk")[1]
	add := e.invocations("acme.n.add")[1]
	require.Equal(t, bulk.ID, add.Parent)
	require.Equal(t, 1, bulk.Committed)
	require.Zero(t, add.Committed, "the joined transaction belongs to the package invocation")
}

func TestNativeDefaultCallsPackageOperationOnTheRightWorker(t *testing.T) {
	// The pool has one worker: a nested package invocation placed anywhere but
	// on the live package ancestor (or, under a Go root, in a free slot)
	// would wait for a slot forever.
	e, _ := newNativeEnv(t)
	e.publish("acme.a", "1.0.0", jsPrelude+`
export const addLabel = async (tx, input) => { await tx.insert(item, { label: input.label }); return { seen: await tx.count(item) }; };
export const outer = async (tx) => tx.call({ id: "acme.n.viaPkg" }, { label: "nested" });`)
	e.contract(manifest.Resource{ID: "acme.a.addLabel", Kind: manifest.ResOperation, Input: obj("label", str()), Output: obj("seen", num())}, "acme.a", "1.0.0", "addLabel")
	e.contract(command("acme.a.outer", true, nil, obj("seen", num())), "acme.a", "1.0.0", "outer")

	res, err := e.host.Invoke(context.Background(), e.req("acme.n.viaPkg", `{"label":"root"}`))
	require.NoError(t, err, "under a Go root the package operation takes the free slot")
	require.JSONEq(t, `{"seen":1}`, string(res.Output))

	res, err = e.host.Invoke(context.Background(), e.req("acme.a.outer", `null`))
	require.NoError(t, err, "under a package root it runs on that root's worker, past the Go frame")
	require.JSONEq(t, `{"seen":2}`, string(res.Output))
	require.Equal(t, []string{"pkg:root", "pkg:nested"}, e.items())
}

func TestPackageReplacementOfNativeCommandAndIndependentGuards(t *testing.T) {
	e, n := newNativeEnv(t)
	e.publish("acme.r", "1.0.0", jsPrelude+`
export const save = async (ctx, input) => {
  if (input.label === "boom") throw new Error("replacement failed");
  return ctx.transaction(async (tx) => tx.call({ id: "acme.n.add" }, { label: "r:" + input.label }));
};`)
	e.addGuard("acme.g.short", "acme.n.add", manifest.GuardOperation, "acme.g", `
export const check = async (tx, input) => { if (input.label.length > 8) throw reject("acme.g.tooLong", ["label"], { max: 8 }); };`)
	e.addGuard("acme.g.precheck", "acme.n.save", manifest.GuardInput, "acme.p", `
export const check = async (q, input) => { if (input.label === "forbidden") throw reject("acme.g.forbidden", ["label"], {}); };`)

	run := func(label string, perms ...string) (host.Result, error) {
		in, _ := json.Marshal(labelIn{Label: label})
		return e.host.Invoke(context.Background(), e.req("acme.n.save", string(in), perms...))
	}

	// The Go default, with both guards.
	_, err := run("much too long", "Acme.Save")
	require.Equal(t, "acme.g.short", issuesOf(t, err)[0].Source, "an operation guard rejects the Go default's write")
	_, err = run("forbidden", "Acme.Save")
	require.Equal(t, "acme.g.precheck", issuesOf(t, err)[0].Source)
	res, err := run("go", "Acme.Save")
	require.NoError(t, err)
	require.True(t, res.Binding.Native)
	require.Equal(t, []string{"go"}, e.items())

	// A command binding makes the package export the one active implementation.
	_, err = e.bindings.Activate(context.Background(), "acme.n.save", "acme.r", "1.0.0", "save", nil)
	require.NoError(t, err)
	before := n.defaultRuns()
	res, err = run("pkg", "Acme.Save")
	require.NoError(t, err)
	require.False(t, res.Binding.Native)
	require.Equal(t, "acme.r", res.Binding.Package)
	require.Equal(t, []string{"go", "r:pkg"}, e.items(), "the replacement wrote through the native operation")
	require.Equal(t, before, n.defaultRuns(), "the Go default did not run")

	_, err = run("boom", "Acme.Save")
	require.ErrorContains(t, err, "replacement failed")
	require.Equal(t, before, n.defaultRuns(), "no fallback to the Go default on error")

	_, err = run("pkg+guard", "Acme.Save")
	require.Equal(t, "acme.g.short", issuesOf(t, err)[0].Source, "the operation guard holds for the replacement")
	_, err = run("forbidden", "Acme.Save")
	require.Equal(t, "acme.g.precheck", issuesOf(t, err)[0].Source, "so does the precheck")
	_, err = run("pkg")
	require.Equal(t, "runtime.permissionDenied", asWire(t, err).Code, "the native permission binds the replacement too")

	recs := e.invocations("acme.n.save")
	var withBinding int
	for _, r := range recs {
		if r.Package == "acme.r" && r.Outcome == "ok" {
			withBinding++
		}
	}
	require.Equal(t, 1, withBinding, "the invocation records the replacement")

	// Closing the binding returns the command to its Go default.
	_, err = e.db.Exec(context.Background(), `UPDATE extruntime.bindings SET active_to = CURRENT_TIMESTAMP WHERE target_id = 'acme.n.save' AND active_to IS NULL`)
	require.NoError(t, err)
	res, err = run("back", "Acme.Save")
	require.NoError(t, err)
	require.True(t, res.Binding.Native)
	require.Equal(t, []string{"go", "r:pkg", "back"}, e.items())
}

// failingStore makes generic inserts fail with data-layer errors chosen by
// the label, as the data store raises them.
type failingStore struct{ spi.DataStore }

func dataErr(code serrors.Code, sentinel *serrors.Error, field string) error {
	err := serrors.New(code, "store failure").WithOp("extruntime.data.Store.Insert").WithCause(sentinel)
	if field != "" {
		err = err.WithFields(serrors.FieldViolation{Field: field})
	}
	return serrors.Wrap("outer", err)
}

func (s failingStore) Insert(ctx context.Context, tx pgx.Tx, sc spi.Scope, entity string, values json.RawMessage) (json.RawMessage, error) {
	var v struct{ Label string }
	_ = json.Unmarshal(values, &v)
	switch v.Label {
	case "unique":
		return nil, dataErr(serrors.AlreadyExists, data.ErrUniqueViolation, "number")
	case "reference":
		return nil, dataErr(serrors.FailedPrecondition, data.ErrReferenceViolation, "holder")
	case "native":
		return nil, dataErr(serrors.FailedPrecondition, data.ErrNativeWrite, "")
	case "foreign":
		return nil, dataErr(serrors.FailedPrecondition, data.ErrForeignWrite, "")
	case "denied":
		return nil, dataErr(serrors.PermissionDenied, data.ErrDenied, "")
	case "hidden":
		return nil, dataErr(serrors.PermissionDenied, data.ErrFieldNotAccessible, "")
	case "missing":
		return nil, dataErr(serrors.NotFound, data.ErrNotFound, "")
	case "conflict":
		return nil, dataErr(serrors.Conflict, data.ErrVersionConflict, "")
	}
	return s.DataStore.Insert(ctx, tx, sc, entity, values)
}

func TestDataLayerErrorsReachPackagesAsStableKinds(t *testing.T) {
	e := newEnv(t, func(c *host.Config) { c.Data = failingStore{DataStore: c.Data} })
	e.publish("acme.a", "1.0.0", jsPrelude+`
export const write = async (tx, input) => {
  try { await tx.insert(item, { label: input.label }); return { kind: "none", code: "", path: [], message: "" }; }
  catch (e) { return { kind: e.kind, code: e.code, path: e.issues?.[0]?.path ?? [], message: e.message }; }
};
export const rethrow = async (tx, input) => { await tx.insert(item, { label: input.label }); return {}; };`)
	out := obj("kind", str(), "code", str(), "path", manifest.Type{Kind: manifest.KindArray, Items: &manifest.Type{Kind: manifest.KindString}}, "message", str())
	e.contract(command("acme.a.write", true, obj("label", str()), out), "acme.a", "1.0.0", "write")
	e.contract(command("acme.a.rethrow", true, obj("label", str()), nil), "acme.a", "1.0.0", "rethrow")

	type result struct {
		Kind, Code, Message string
		Path                []string
	}
	cases := map[string]result{
		"unique":    {Kind: "validation", Code: host.CodeDataUnique, Path: []string{"number"}},
		"reference": {Kind: "validation", Code: host.CodeDataReference, Path: []string{"holder"}},
		"native":    {Kind: "denied", Code: host.CodeDataNativeWrite},
		"foreign":   {Kind: "denied", Code: host.CodeDataForeignWrite},
		"denied":    {Kind: "denied", Code: host.CodeDataDenied},
		"hidden":    {Kind: "denied", Code: host.CodeDataFieldHidden},
		"missing":   {Kind: "domain", Code: host.CodeDataNotFound},
		"conflict":  {Kind: "domain", Code: host.CodeDataVersionConflict},
	}
	for label, want := range cases {
		t.Run(label, func(t *testing.T) {
			res, err := e.host.Invoke(context.Background(), e.req("acme.a.write", `{"label":"`+label+`"}`))
			require.NoError(t, err)
			var got result
			require.NoError(t, json.Unmarshal(res.Output, &got))
			require.Equal(t, want.Kind, got.Kind)
			require.Equal(t, want.Code, got.Code)
			if want.Path == nil {
				want.Path = []string{}
			}
			require.Equal(t, want.Path, got.Path)
			require.NotContains(t, got.Message, "store failure", "no internal diagnostics reach the author")
		})
	}

	t.Run("platform domain errors pass the closed-errors check of any contract", func(t *testing.T) {
		_, err := e.host.Invoke(context.Background(), e.req("acme.a.rethrow", `{"label":"missing"}`))
		we := asWire(t, err)
		require.Equal(t, pool.ErrKindDomain, we.Kind)
		require.Equal(t, host.CodeDataNotFound, we.Code)
		_, err = e.host.Invoke(context.Background(), e.req("acme.a.rethrow", `{"label":"unique"}`))
		require.Equal(t, host.CodeDataUnique, asWire(t, err).Code)
	})
}

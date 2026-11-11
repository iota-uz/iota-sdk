package build_test

import (
	"context"
	"encoding/json"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/build"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/build/buildtest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/pool"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/pool/pooltest"
)

func newBuilder(t *testing.T) *build.NodeBuilder {
	t.Helper()
	b, err := build.NewNodeBuilder(buildtest.Config(t))
	require.NoError(t, err)
	return b
}

// loadTree reads testdata/<name> as a package tree.
func loadTree(t *testing.T, name string) build.Tree {
	t.Helper()
	root := filepath.Join("testdata", name)
	tree := build.Tree{}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(root, p)
		data, err := os.ReadFile(p)
		tree[filepath.ToSlash(rel)] = data
		return err
	})
	require.NoError(t, err)
	return tree
}

func clone(t build.Tree) build.Tree {
	c := make(build.Tree, len(t))
	for k, v := range t {
		c[k] = append([]byte(nil), v...)
	}
	return c
}

// replace edits one source file of a tree copy and fails if the text is absent.
func replace(t *testing.T, tree build.Tree, file, old, repl string) {
	t.Helper()
	src := string(tree[file])
	require.Contains(t, src, old, file)
	tree[file] = []byte(strings.Replace(src, old, repl, 1))
}

func lineOf(t *testing.T, tree build.Tree, file, needle string) int {
	t.Helper()
	for i, l := range strings.Split(string(tree[file]), "\n") {
		if strings.Contains(l, needle) {
			return i + 1
		}
	}
	t.Fatalf("%s: %q not found", file, needle)
	return 0
}

// nativeSources reads the native catalogs testdata/kit/<ns>.json.
func nativeSources(t *testing.T) []build.KitSource {
	t.Helper()
	var sources []build.KitSource
	for _, ns := range []string{"core", "crm"} {
		raw, err := os.ReadFile(filepath.Join("testdata", "kit", ns+".json"))
		require.NoError(t, err)
		var resources []manifest.Resource
		require.NoError(t, json.Unmarshal(raw, &resources))
		sources = append(sources, build.KitSource{Namespace: ns, Resources: resources})
	}
	return sources
}

func nativeKit(t *testing.T, b *build.NodeBuilder) build.Kit {
	t.Helper()
	kit, err := b.GenerateKit(context.Background(), nativeSources(t))
	require.NoError(t, err)
	return kit
}

func diagnostic(t *testing.T, err error, code string) build.Diagnostic {
	t.Helper()
	var failed *build.Failed
	require.ErrorAs(t, err, &failed)
	for _, d := range failed.Diagnostics {
		if d.Code == code {
			return d
		}
	}
	t.Fatalf("no %s diagnostic in %+v", code, failed.Diagnostics)
	return build.Diagnostic{}
}

func TestBuildAcmePackage(t *testing.T) {
	b := newBuilder(t)
	kit := nativeKit(t, b)
	tree := loadTree(t, "acme")

	started := time.Now()
	res, err := b.Build(context.Background(), tree, kit)
	require.NoError(t, err)
	t.Logf("build acme: %s (kit %s)", time.Since(started).Round(time.Millisecond), kit.Hash[:8])
	require.Empty(t, res.Diagnostics)
	require.NoError(t, res.Manifest.Validate())
	require.Equal(t, "acme.insurance", res.Manifest.Namespace)

	kinds := map[manifest.ResourceKind]int{}
	for _, r := range res.Manifest.Resources {
		kinds[r.Kind]++
	}
	for _, k := range []manifest.ResourceKind{
		manifest.ResEntity, manifest.ResOverlay, manifest.ResCommand, manifest.ResOperation, manifest.ResQuery, manifest.ResEvent,
		manifest.ResWorkflow, manifest.ResNotice, manifest.ResRoute, manifest.ResPermission, manifest.ResIssue, manifest.ResDomainError,
		manifest.ResColumn, manifest.ResPage, manifest.ResNavigation, manifest.ResSlot, manifest.ResCell, manifest.ResShell,
		manifest.ResContactResolver, manifest.ResDeliveryPolicy,
	} {
		require.NotZero(t, kinds[k], "manifest has no %s resource", k)
	}
	m := res.Manifest
	require.NotEmpty(t, m.Implementations)
	require.NotEmpty(t, m.Subscriptions)
	require.NotEmpty(t, m.Guards)
	require.NotEmpty(t, m.Templates)
	require.NotEmpty(t, m.Schedules)
	require.NotEmpty(t, m.Endpoints)
	require.Equal(t, []string{"quotes.example.uz"}, m.Capabilities.Network)
	require.Equal(t, []string{"SMS_TOKEN"}, m.Capabilities.Secrets)
	require.Equal(t, map[string]string{"core": kit.Hash, "crm": kit.Hash}, m.Dependencies)

	// Foreign (kit) resources are referenced, never re-declared.
	_, foreign := m.Find("crm.client")
	require.False(t, foreign)
	// Atomicity is carried by the contract; replacement of a foreign command is an implementation entry.
	activate, ok := m.Find("acme.insurance.activatePolicy")
	require.True(t, ok)
	require.True(t, activate.Atomic)
	var replaces bool
	for _, i := range m.Implementations {
		if i.Implements == "core.commands.rejectIncoming" && i.Kind == manifest.ImplAtomic {
			replaces = true
		}
	}
	require.True(t, replaces, "rejectIncoming replacement must be listed")

	// Pages point at the entry chunk and their export.
	page, ok := m.Find("acme.insurance.policies")
	require.True(t, ok)
	require.Equal(t, "policies", page.Export)
	require.NotEmpty(t, page.Chunk)
	require.Contains(t, res.Artifacts, page.Chunk)
	require.Contains(t, res.Artifacts, "client/manifest.json")
	require.Contains(t, res.Artifacts, "server.mjs")
	require.Contains(t, res.Artifacts, "contracts.mjs")

	var clientManifest struct {
		Schema  string   `json:"schema"`
		Entry   string   `json:"entry"`
		Chunks  []string `json:"chunks"`
		Imports []string `json:"imports"`
		Exports []string `json:"exports"`
	}
	require.NoError(t, json.Unmarshal(res.Artifacts["client/manifest.json"], &clientManifest))
	require.Equal(t, page.Chunk, clientManifest.Entry)
	require.GreaterOrEqual(t, len(clientManifest.Chunks), 2, "lazy(() => import()) screens are separate chunks")
	require.Contains(t, clientManifest.Imports, "solid-js")
	require.Contains(t, clientManifest.Exports, "policies")

	// No handler stripping and no server code in the browser graph.
	for name, content := range res.Artifacts {
		if strings.HasPrefix(name, "client/") && strings.HasSuffix(name, ".js") {
			require.NotContains(t, string(content), "quotes.example.uz", name)
			require.NotContains(t, string(content), "implement(", name)
			require.NotContains(t, string(content), "@iota-uz/sdk/runtime/server", name)
		}
	}
	// The pure contracts artifact has no handlers either.
	require.NotContains(t, string(res.Artifacts["contracts.mjs"]), "quotes.example.uz")
	require.NotContains(t, string(res.Artifacts["contracts.mjs"]), "implement(")
	require.Contains(t, string(res.Artifacts["server.mjs"]), `"@iota-uz/sdk/runtime/server"`)

	// Deterministic: another materialization produces the same artifacts.
	again, err := b.Build(context.Background(), clone(tree), kit)
	require.NoError(t, err)
	require.Equal(t, res.SourceHash, again.SourceHash)
	require.Equal(t, res.ArtifactHash, again.ArtifactHash)
	require.Equal(t, build.HashTree(res.Artifacts), res.ArtifactHash)

	// Check returns the same (empty) diagnostics without artifacts.
	diags, err := b.Check(context.Background(), tree, kit)
	require.NoError(t, err)
	require.Empty(t, diags)

	t.Run("server.mjs runs in a real pool worker", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), res.ArtifactHash)
		for name, content := range res.Artifacts {
			p := filepath.Join(dir, filepath.FromSlash(name))
			require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o755))
			require.NoError(t, os.WriteFile(p, content, 0o644))
		}
		p, err := pool.New(pool.Config{Size: 1, WorkerScript: pooltest.BuildWorker(t), CancelGrace: 300 * time.Millisecond})
		require.NoError(t, err)
		t.Cleanup(func() { _ = p.Close(context.Background()) })
		art := pool.Artifact{Hash: res.ArtifactHash, Dir: dir}
		user := pool.UserContext{ID: "7", Kind: "initiator", Permissions: []string{}}

		var calls []string
		host := pool.HostCallsFunc(func(_ context.Context, method string, args json.RawMessage) (json.RawMessage, error) {
			calls = append(calls, method)
			switch method {
			case "tx.update":
				return json.RawMessage(`{"key":"k1"}`), nil
			case "tx.start":
				return json.RawMessage(`{"instance":"wf-1"}`), nil
			case "q.find":
				return json.RawMessage(`[{"key":"k1","number":"P-1"}]`), nil
			}
			t.Fatalf("unexpected host call %s %s", method, args)
			return nil, nil
		})
		ctx := context.Background()

		out, err := p.Invoke(ctx, pool.Invocation{
			Artifact: art, Kind: pool.KindAtomic, Export: "activate", Resource: "acme.insurance.activatePolicy",
			Input: json.RawMessage(`{"policy":{"entity":"acme.insurance.policy","key":"k1"}}`),
			Ctx:   pool.InvokeContext{Tenant: "t-1", Trigger: "ui", User: user, Tx: "tx-1"},
			Calls: host,
		})
		require.NoError(t, err)
		require.JSONEq(t, `{"status":"active"}`, string(out))
		require.Equal(t, []string{"tx.update", "tx.start"}, calls)

		out, err = p.Invoke(ctx, pool.Invocation{
			Artifact: art, Kind: pool.KindQuery, Export: "expiring", Resource: "acme.insurance.expiring",
			Input: json.RawMessage(`{"before":"2030-01-01"}`),
			Ctx:   pool.InvokeContext{Tenant: "t-1", Trigger: "ui", User: user},
			Calls: host,
		})
		require.NoError(t, err)
		require.JSONEq(t, `[{"policy":{"entity":"acme.insurance.policy","key":"k1"},"number":"P-1"}]`, string(out))
	})
}

func TestGraphAndTypeDiagnostics(t *testing.T) {
	b := newBuilder(t)
	kit := nativeKit(t, b)
	base := loadTree(t, "acme")
	ctx := context.Background()

	t.Run("client importing the server entry", func(t *testing.T) {
		tree := clone(base)
		replace(t, tree, "src/client.tsx", `import * as c from "./contracts"`, "import * as c from \"./contracts\"\nimport { renew } from \"./server\"\nexport const leaked = renew")
		_, err := b.Build(ctx, tree, kit)
		d := diagnostic(t, err, "graph.serverInClient")
		require.Equal(t, "src/client.tsx", d.File)
		require.Equal(t, lineOf(t, tree, "src/client.tsx", `from "./server"`), d.Line)
		require.Equal(t, build.SeverityError, d.Severity)
		require.Contains(t, d.Message, "src/server.ts")
	})

	t.Run("client reaching the server SDK entry and Node builtins", func(t *testing.T) {
		tree := clone(base)
		replace(t, tree, "src/client.tsx", `import * as c from "./contracts"`, "import * as c from \"./contracts\"\nimport { guard } from \"@iota-uz/sdk/runtime/server\"\nimport { readFileSync } from \"node:fs\"\nexport const x = [guard, readFileSync]")
		_, err := b.Build(ctx, tree, kit)
		var failed *build.Failed
		require.ErrorAs(t, err, &failed)
		lines := map[int]string{}
		for _, d := range failed.Diagnostics {
			if d.Code == "graph.serverInClient" {
				lines[d.Line] = d.Message
			}
		}
		require.Contains(t, lines, lineOf(t, tree, "src/client.tsx", `"@iota-uz/sdk/runtime/server"`))
		require.Contains(t, lines, lineOf(t, tree, "src/client.tsx", `"node:fs"`))
	})

	t.Run("client reaching a module only the server uses", func(t *testing.T) {
		tree := clone(base)
		tree["src/serverOnly.ts"] = []byte("export const secret = 1\n")
		replace(t, tree, "src/server.ts", `import * as c from "./contracts"`, "import * as c from \"./contracts\"\nimport { secret } from \"./serverOnly\"\nexport const usesSecret = secret")
		replace(t, tree, "src/client.tsx", `import * as c from "./contracts"`, "import * as c from \"./contracts\"\nimport { secret } from \"./serverOnly\"\nexport const alsoSecret = secret")
		_, err := b.Build(ctx, tree, kit)
		d := diagnostic(t, err, "graph.serverInClient")
		require.Equal(t, "src/client.tsx", d.File)
		require.Contains(t, d.Message, "reachable only from the server graph")
	})

	t.Run("impure contracts", func(t *testing.T) {
		tree := clone(base)
		replace(t, tree, "src/contracts.ts", `import * as core from "@iota-kit/core"`, "import * as core from \"@iota-kit/core\"\nimport { readFileSync } from \"node:fs\"\nexport const leak = readFileSync")
		_, err := b.Build(ctx, tree, kit)
		d := diagnostic(t, err, "graph.impureContract")
		require.Equal(t, "src/contracts.ts", d.File)
		require.Equal(t, lineOf(t, tree, "src/contracts.ts", `"node:fs"`), d.Line)
	})

	t.Run("contracts reaching the server SDK entry and solid-js", func(t *testing.T) {
		tree := clone(base)
		replace(t, tree, "src/contracts.ts", `import * as core from "@iota-kit/core"`, "import * as core from \"@iota-kit/core\"\nimport { guard } from \"@iota-uz/sdk/runtime/server\"\nimport { createSignal } from \"solid-js\"\nexport const x = [guard, createSignal]")
		_, err := b.Build(ctx, tree, kit)
		var failed *build.Failed
		require.ErrorAs(t, err, &failed)
		var n int
		for _, d := range failed.Diagnostics {
			if d.Code == "graph.impureContract" && d.File == "src/contracts.ts" {
				n++
			}
		}
		require.Equal(t, 2, n)
	})

	t.Run("type error: tx.call of an ordinary command", func(t *testing.T) {
		tree := clone(base)
		replace(t, tree, "src/server.ts", "tx.call(c.ExtendExpiry, { policy: input.policy, months: 12 })", "tx.call(c.RenewPolicy, { policy: input.policy })")
		diags, err := b.Check(ctx, tree, kit)
		require.NoError(t, err)
		var found *build.Diagnostic
		for i := range diags {
			if diags[i].Code == "ts.2345" {
				found = &diags[i]
			}
		}
		require.NotNil(t, found, "%+v", diags)
		require.Equal(t, "src/server.ts", found.File)
		require.Equal(t, lineOf(t, tree, "src/server.ts", "tx.call(c.RenewPolicy"), found.Line)

		_, err = b.Build(ctx, tree, kit)
		require.Equal(t, "src/server.ts", diagnostic(t, err, "ts.2345").File)
	})

	t.Run("duplicate resource id", func(t *testing.T) {
		tree := clone(base)
		tree["src/contracts.ts"] = append(tree["src/contracts.ts"], []byte("\nexport const Dup = query('acme.insurance.expiring', { input: t.object({}), output: t.object({}) })\n")...)
		_, err := b.Build(ctx, tree, kit)
		d := diagnostic(t, err, "manifest.duplicateId")
		require.Equal(t, "src/contracts.ts", d.File)
		require.Equal(t, lineOf(t, tree, "src/contracts.ts", "export const Dup"), d.Line)
		require.Contains(t, d.Message, "acme.insurance.expiring")
	})

	t.Run("unknown kit module", func(t *testing.T) {
		tree := clone(base)
		replace(t, tree, "src/contracts.ts", `import * as core from "@iota-kit/core"`, "import * as core from \"@iota-kit/core\"\nimport * as ghost from \"@iota-kit/ghost\"\nexport const g = ghost")
		_, err := b.Build(ctx, tree, kit)
		d := diagnostic(t, err, "graph.unknownKit")
		require.Equal(t, "src/contracts.ts", d.File)
	})

	t.Run("invalid package.json", func(t *testing.T) {
		tree := clone(base)
		tree["package.json"] = []byte(`{"name":"x","iota":{"namespace":"Bad Name"}}`)
		_, err := b.Build(ctx, tree, kit)
		require.Equal(t, "package.json", diagnostic(t, err, "package.invalid").File)
	})
}

func TestPackageTests(t *testing.T) {
	b := newBuilder(t)
	kit := nativeKit(t, b)
	ctx := context.Background()

	t.Run("passing cases", func(t *testing.T) {
		started := time.Now()
		report, err := b.Test(ctx, loadTree(t, "acme"), kit)
		require.NoError(t, err)
		t.Logf("package tests: %s", time.Since(started).Round(time.Millisecond))
		require.True(t, report.Passed, "%+v", report.Cases)
		names := map[string]bool{}
		for _, c := range report.Cases {
			require.True(t, c.Passed, "%s: %s", c.Name, c.Message)
			require.Equal(t, "src/server.test.ts", c.File)
			names[c.Name] = true
		}
		require.True(t, names["extendExpiry moves the date and emits an event"])
		require.True(t, names["unknown outbound http is blocked"])
		require.Len(t, report.Cases, 7)
	})

	t.Run("failing and unknown-effect cases", func(t *testing.T) {
		tree := loadTree(t, "acme")
		tree["src/extra.test.ts"] = []byte(`import assert from "node:assert/strict"
import { test } from "@iota-uz/sdk/runtime/testing"

test("wrong expectation", () => {
  assert.equal(1 + 1, 3)
})

test("an unregistered fetch fails the test instead of reaching the network", async () => {
  await fetch("https://api.example.com/charge")
})
`)
		report, err := b.Test(ctx, tree, kit)
		require.NoError(t, err)
		require.False(t, report.Passed)
		results := map[string]build.TestCase{}
		for _, c := range report.Cases {
			results[c.Name] = c
		}
		wrong := results["wrong expectation"]
		require.False(t, wrong.Passed)
		require.Equal(t, "src/extra.test.ts", wrong.File)
		require.Contains(t, wrong.Message, "AssertionError")
		fetched := results["an unregistered fetch fails the test instead of reaching the network"]
		require.False(t, fetched.Passed)
		require.Contains(t, fetched.Message, "blocked in synthetic tests")
		require.True(t, results["extendExpiry moves the date and emits an event"].Passed, "other files still pass")
	})

	t.Run("a test file that does not bundle fails as a case", func(t *testing.T) {
		tree := loadTree(t, "acme")
		tree["src/broken.test.ts"] = []byte("import { nothing } from './missing'\nnothing()\n")
		report, err := b.Test(ctx, tree, kit)
		require.NoError(t, err)
		require.False(t, report.Passed)
		var broken *build.TestCase
		for i := range report.Cases {
			if report.Cases[i].File == "src/broken.test.ts" {
				broken = &report.Cases[i]
			}
		}
		require.NotNil(t, broken)
		require.False(t, broken.Passed)
	})
}

// kitModule JS must be pure references: no handler code from the package.
func requirePure(t *testing.T, kit build.Kit, module string) {
	t.Helper()
	files, ok := kit.Modules[module]
	require.True(t, ok, module)
	js := string(files["index.js"])
	require.NotEmpty(t, files["index.d.ts"])
	require.NotEmpty(t, files["package.json"])
	for _, handlerText := range []string{"implement(", "subscribe(", "templates(", "endpoint(", "schedule(", "quotes.example.uz", "async ", "await ", "=>", "fetch("} {
		require.NotContains(t, js, handlerText, "%s index.js contains handler code", module)
	}
}

func TestKitGenerationAndConsumerPackage(t *testing.T) {
	b := newBuilder(t)
	native := nativeKit(t, b)
	ctx := context.Background()

	acme, err := b.Build(ctx, loadTree(t, "acme"), native)
	require.NoError(t, err)

	// The catalog snapshot: native catalogs plus the active package.
	variants := map[string][]build.KitSource{
		"from manifest resources": append(nativeSources(t), build.KitSource{Namespace: "acme.insurance", Resources: acme.Manifest.Resources}),
		"shipping the pure contracts module": append(nativeSources(t), build.KitSource{
			Namespace:       "acme.insurance",
			Resources:       acme.Manifest.Resources,
			ContractsModule: acme.Artifacts["contracts.mjs"],
		}),
	}
	for name, sources := range variants {
		t.Run(name, func(t *testing.T) {
			kit, err := b.GenerateKit(ctx, sources)
			require.NoError(t, err)
			requirePure(t, kit, "@iota-kit/acme.insurance")
			require.Contains(t, string(kit.Modules["@iota-kit/acme.insurance"]["index.d.ts"]), "acme.insurance.activatePolicy")

			partner := loadTree(t, "partner")
			res, err := b.Build(ctx, partner, kit)
			require.NoError(t, err)
			require.Empty(t, res.Diagnostics)
			// The resolution snapshot pins every kit module the consumer resolved through to the snapshot hash.
			require.Equal(t, kit.Hash, res.Manifest.Dependencies["acme.insurance"])
			for ns, hash := range res.Manifest.Dependencies {
				require.Equal(t, kit.Hash, hash, ns)
			}
			sub := res.Manifest.Subscriptions
			require.Len(t, sub, 1)
			require.Equal(t, "acme.insurance.policyRenewed.v1", sub[0].Event)
			require.Len(t, res.Manifest.Implementations, 1)
			require.Equal(t, manifest.ImplAtomic, res.Manifest.Implementations[0].Kind)
			require.Equal(t, "acme.insurance.activatePolicy", res.Manifest.Implementations[0].Implements)
			// Foreign contracts are not re-declared by the consumer.
			_, redeclared := res.Manifest.Find("acme.insurance.policy")
			require.False(t, redeclared)

			// Generic writes to a foreign entity are a type error against the generated kit.
			bad := clone(partner)
			replace(t, bad, "src/server.ts", "const policy = await tx.get(acme.Policy, input.policy)", "await tx.insert(acme.Policy, {} as never)\n  const policy = await tx.get(acme.Policy, input.policy)")
			diags, err := b.Check(ctx, bad, kit)
			require.NoError(t, err)
			var typeErr bool
			for _, d := range diags {
				if d.Code == "ts.2345" && d.File == "src/server.ts" && d.Line == lineOf(t, bad, "src/server.ts", "tx.insert(acme.Policy") {
					typeErr = true
				}
			}
			require.True(t, typeErr, "%+v", diags)
		})
	}

	t.Run("native catalog kit", func(t *testing.T) {
		requirePure(t, native, "@iota-kit/core")
		requirePure(t, native, "@iota-kit/crm")
		js := string(native.Modules["@iota-kit/crm"]["index.js"])
		require.Contains(t, js, `nativeEntity("crm.client"`)
		require.Contains(t, js, `nativeList("crm.lists.Clients")`)
		require.Contains(t, js, `slotRef("crm.slots.clientToolbar")`)
		require.Contains(t, string(native.Modules["@iota-kit/core"]["index.js"]), `import * as kit_crm from "@iota-kit/crm"`)
	})

	t.Run("generated refs reproduce the catalog resources exactly", func(t *testing.T) {
		// A package test imports the generated modules, so the same build pipeline that
		// serves authors evaluates them: every public resource must read back as declared.
		tree := build.Tree{
			"package.json": []byte(`{"name":"roundtrip","type":"module"}`),
			"src/roundtrip.test.ts": []byte(`import assert from "node:assert/strict"
import { test } from "@iota-uz/sdk/runtime/testing"
import * as core from "@iota-kit/core"
import * as crm from "@iota-kit/crm"
import expectedCore from "./expected-core.json"
import expectedCrm from "./expected-crm.json"

type Resource = Record<string, unknown> & { id: string; kind: string }
const strip = (r: Resource) => { const { label, description, ...rest } = r; return rest }

for (const [name, module, expected] of [["core", core, expectedCore], ["crm", crm, expectedCrm]] as const) {
  test(name + " kit modules reproduce the catalog", () => {
    const actual = new Map<string, Resource>()
    for (const v of Object.values(module)) {
      const resource = (v as { resource?: Resource }).resource
      if (resource) actual.set(resource.id, resource)
    }
    let checked = 0
    for (const want of expected as Resource[]) {
      if (want.kind === "slot") { checked++; continue }
      const got = actual.get(want.id)
      assert.ok(got, "missing " + want.id)
      assert.deepEqual(strip(got), strip(want), want.id)
      checked++
    }
    assert.ok(checked > 0)
  })
}
`),
		}
		for _, ns := range []string{"core", "crm"} {
			raw, err := os.ReadFile(filepath.Join("testdata", "kit", ns+".json"))
			require.NoError(t, err)
			tree["src/expected-"+ns+".json"] = raw
		}
		report, err := b.Test(ctx, tree, native)
		require.NoError(t, err)
		require.True(t, report.Passed, "%+v", report.Cases)
		require.Len(t, report.Cases, 2)
	})

	t.Run("kit generation is deterministic", func(t *testing.T) {
		again := nativeKit(t, b)
		require.Equal(t, native.Hash, again.Hash)
		require.Equal(t, native.Modules, again.Modules)
	})

	t.Run("a dependency cycle between kit modules is rejected", func(t *testing.T) {
		client := manifest.Resource{ID: "a.thing", Kind: manifest.ResEntity, Fields: []manifest.Field{{ID: "other", Type: manifest.Type{Kind: manifest.KindRef, Entity: "b.thing"}}}}
		other := manifest.Resource{ID: "b.thing", Kind: manifest.ResEntity, Fields: []manifest.Field{{ID: "other", Type: manifest.Type{Kind: manifest.KindRef, Entity: "a.thing"}}}}
		_, err := b.GenerateKit(ctx, []build.KitSource{{Namespace: "a", Resources: []manifest.Resource{client}}, {Namespace: "b", Resources: []manifest.Resource{other}}})
		var failed *build.Failed
		require.ErrorAs(t, err, &failed)
		require.Equal(t, "kit.cycle", failed.Diagnostics[0].Code)
	})
}

func TestNpmDependenciesAreBundledIntoServer(t *testing.T) {
	if _, err := exec.LookPath("npm"); err != nil {
		t.Skip("npm is not installed")
	}
	b := newBuilder(t)
	tree := loadTree(t, "npmdep")
	res, err := b.Build(context.Background(), tree, build.Kit{})
	require.NoError(t, err)
	require.Empty(t, res.Diagnostics)
	server := string(res.Artifacts["server.mjs"])
	require.Contains(t, server, "addVat", "the npm dependency is bundled")
	require.NotContains(t, server, `from "pure-lib"`)

	t.Run("a lockfile is required", func(t *testing.T) {
		bad := clone(tree)
		delete(bad, "package-lock.json")
		_, err := b.Build(context.Background(), bad, build.Kit{})
		require.Equal(t, "package.json", diagnostic(t, err, "npm.lockfileMissing").File)
	})
}

func TestTreeHandling(t *testing.T) {
	require.Equal(t, build.HashTree(build.Tree{"a": []byte("1"), "b": []byte("2")}), build.HashTree(build.Tree{"b": []byte("2"), "a": []byte("1")}))
	require.NotEqual(t, build.HashTree(build.Tree{"a": []byte("12")}), build.HashTree(build.Tree{"a1": []byte("2")}))

	b := newBuilder(t)
	for _, p := range []string{"../escape.ts", "/abs.ts", "a/../b.ts", "node_modules/x/index.js"} {
		_, err := b.Check(context.Background(), build.Tree{p: []byte("x"), "package.json": []byte("{}")}, build.Kit{})
		require.Error(t, err, p)
	}
}

func TestOperationTimeoutKillsTheWorker(t *testing.T) {
	cfg := buildtest.Config(t)
	cfg.Timeout = 50 * time.Millisecond
	b, err := build.NewNodeBuilder(cfg)
	require.NoError(t, err)
	_, err = b.Check(context.Background(), loadTree(t, "acme"), build.Kit{})
	require.Error(t, err)
	require.ErrorIs(t, err, context.DeadlineExceeded)
}

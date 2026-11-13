package build_test

import (
	"context"
	"encoding/json"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/build"
)

// styledTree is a package that styles its client with the UI kit, Tailwind utilities and a scoped CSS module, and
// has a contract and a server so that the graphs that must refuse stylesheets exist.
func styledTree() build.Tree {
	return build.Tree{
		"package.json": []byte(`{
  "name": "@acme/styled", "version": "1.0.0", "private": true, "type": "module",
  "iota": { "namespace": "acme.styled", "contracts": "./src/contracts.ts", "server": "./src/server.ts", "client": "./src/client.tsx" }
}`),
		"src/contracts.ts": []byte(`import { query, t } from "@iota-uz/sdk/runtime"
export const Ping = query("acme.styled.ping", { input: t.object({}), output: t.object({ ok: t.string() }) })
`),
		"src/server.ts": []byte(`import { implement } from "@iota-uz/sdk/runtime/server"
import * as c from "./contracts"
export const ping = implement(c.Ping, async () => ({ ok: "yes" }))
`),
		"src/css-modules.d.ts": []byte(`declare module "*.module.css" {
  const classes: Readonly<Record<string, string>>
  export default classes
}
`),
		"src/client.tsx": []byte(`import { lazy } from "solid-js"
import { page } from "@iota-uz/sdk/runtime/client"

export const board = page("acme.styled.board", { path: "/styled/board", component: lazy(() => import("./Board")) })
`),
		"src/Board.tsx": []byte(`import { Button, Tabs } from "@iota-uz/sdk/solid-ui"
import styles from "./Board.module.css"

export default function Board() {
  return (
    <main class={styles.board}>
      <div class="p-4 bg-primary-600 hover:bg-primary-700 md:flex">Treaty</div>
      <Button>Bind</Button>
      <Tabs defaultValue="a" />
    </main>
  )
}
`),
		"src/Board.module.css": []byte(`.board { display: grid; letter-spacing: 3px; }
.unused { outline-width: 9px; }
`),
	}
}

type clientManifest struct {
	Entry  string   `json:"entry"`
	Chunks []string `json:"chunks"`
	CSS    []string `json:"css"`
	Shared []string `json:"shared"`
}

func buildStyled(t *testing.T, tree build.Tree) (build.Result, clientManifest) {
	t.Helper()
	b := newBuilder(t)
	res, err := b.Build(context.Background(), tree, nativeKit(t, b))
	require.NoError(t, err)
	var m clientManifest
	require.NoError(t, json.Unmarshal(res.Artifacts["client/manifest.json"], &m))
	return res, m
}

// The UI kit is the host's instance: the package bundle reads it through the host's lazy loader and carries none of
// its code. Both of the package's stylesheets are recorded in the client manifest, utilities first, and are part of the
// artifact the version hash covers.
func TestClientStylesAndSharedKit(t *testing.T) {
	res, m := buildStyled(t, styledTree())

	require.Contains(t, m.Shared, "@iota-uz/sdk/solid-ui", "the kit is recorded as a shared module of the version")
	var js strings.Builder
	for path, data := range res.Artifacts {
		if strings.HasPrefix(path, "client/") && strings.HasSuffix(path, ".js") {
			js.Write(data)
		}
	}
	bundle := js.String()
	require.Contains(t, bundle, `Symbol.for("iota.runtime.shared.loaders")`)
	require.Contains(t, bundle, "await load()", "the shim awaits the host's lazy loader")
	// Class names and markers of the kit's own Button and Tabs sources: none of them is in the package bundle.
	for _, kitCode := range []string{"btn-loading-indicator", "btn-primary", "TabsTrigger = ", "Tab components must be rendered inside Tabs"} {
		require.NotContains(t, bundle, kitCode, "the kit is shared, not bundled")
	}

	require.Len(t, m.CSS, 2)
	require.Equal(t, "client/tailwind.css", m.CSS[0], "utilities come first")
	require.Regexp(t, `^client/index-[A-Z0-9]+\.css$`, m.CSS[1])
	for _, css := range m.CSS {
		require.Contains(t, res.Artifacts, css)
	}

	// Utilities: only the classes the client sources use, on the SDK tokens and the host's token variables.
	tw := string(res.Artifacts["client/tailwind.css"])
	require.Contains(t, tw, "oklch(var(--primary-600)", "bg-primary-600 resolves through the SDK token mapping")
	require.Contains(t, tw, "oklch(var(--primary-700)", "hover:bg-primary-700 too")
	require.Contains(t, tw, ".p-4{")
	require.Contains(t, tw, "width>=48rem", "a responsive variant is compiled")
	require.NotContains(t, tw, "bg-fuchsia", "no class that no source uses")
	require.NotContains(t, tw, "bg-primary-400")
	require.NotContains(t, tw, "*,:before,:after", "no preflight: the host page already has it")
	require.NotContains(t, tw, "@layer base")
	require.NotRegexp(t, `:root|:host|@layer\s+(theme|base|components|properties)|@layer\s+[a-z, ]+;`, tw, "utilities only: no theme variables, no layer-order statement")
	require.NotContains(t, tw, "var(--spacing)", "theme values are inlined into the utilities")

	// A CSS module is scoped to the package, path and content, and emitted into the version's stylesheet.
	css := string(res.Artifacts[m.CSS[1]])
	require.Regexp(t, `\.Board_[0-9a-f]{8}_board\b`, css)
	require.NotRegexp(t, `\.board\s*\{`, css, "the local name is never emitted bare")
	require.Regexp(t, `Board_[0-9a-f]{8}_board`, bundle, "the component receives the scoped name")

	// The stylesheets are part of the immutable artifact: changing a class changes the version's hash.
	changed := styledTree()
	changed["src/Board.module.css"] = append(changed["src/Board.module.css"], []byte(".extra { color: red; }\n")...)
	again, _ := buildStyled(t, changed)
	require.NotEqual(t, res.ArtifactHash, again.ArtifactHash)
	same, _ := buildStyled(t, styledTree())
	require.Equal(t, res.ArtifactHash, same.ArtifactHash, "the build, stylesheets included, is deterministic")
}

// Two packages (or two versions of one) share a page: their same-named CSS modules must not share class names.
func TestCSSModuleNamesAreScopedToPackageAndContent(t *testing.T) {
	a, ma := buildStyled(t, styledTree())
	other := styledTree()
	for path, data := range other {
		other[path] = []byte(strings.ReplaceAll(string(data), "acme.styled", "acme.other"))
	}
	b, mb := buildStyled(t, other)
	nameA := regexpFind(t, `Board_[0-9a-f]{8}_board`, string(a.Artifacts[ma.CSS[1]]))
	nameB := regexpFind(t, `Board_[0-9a-f]{8}_board`, string(b.Artifacts[mb.CSS[1]]))
	require.NotEqual(t, nameA, nameB)
}

func regexpFind(t *testing.T, pattern, text string) string {
	t.Helper()
	found := regexp.MustCompile(pattern).FindString(text)
	require.NotEmpty(t, found, pattern)
	return found
}

func TestStylesheetDiagnostics(t *testing.T) {
	b := newBuilder(t)
	kit := nativeKit(t, b)
	ctx := context.Background()

	t.Run("a global stylesheet in client code is refused with the two allowed forms named", func(t *testing.T) {
		tree := styledTree()
		tree["src/global.css"] = []byte("body { color: red }\n")
		replace(t, tree, "src/Board.tsx", `import styles from "./Board.module.css"`, "import styles from \"./Board.module.css\"\nimport \"./global.css\"")
		_, err := b.Build(ctx, tree, kit)
		d := diagnostic(t, err, "graph.globalCss")
		require.Equal(t, "src/Board.tsx", d.File)
		require.Equal(t, lineOf(t, tree, "src/Board.tsx", `"./global.css"`), d.Line)
		require.Equal(t, build.SeverityError, d.Severity)
		require.Contains(t, d.Message, "shares one page")
		require.Contains(t, d.Message, "Tailwind")
		require.Contains(t, d.Message, ".module.css")
	})

	t.Run("a global stylesheet imported by a module stylesheet is located in the stylesheet", func(t *testing.T) {
		tree := styledTree()
		tree["src/base.css"] = []byte("a { color: red }\n")
		tree["src/Board.module.css"] = append([]byte("@import \"./base.css\";\n"), tree["src/Board.module.css"]...)
		_, err := b.Build(ctx, tree, kit)
		d := diagnostic(t, err, "graph.globalCss")
		require.Equal(t, "src/Board.module.css", d.File)
		require.Equal(t, 1, d.Line)
	})

	t.Run("stylesheets in the server graph are refused", func(t *testing.T) {
		tree := styledTree()
		tree["src/global.css"] = []byte("body { color: red }\n")
		replace(t, tree, "src/server.ts", `import * as c from "./contracts"`, "import * as c from \"./contracts\"\nimport \"./global.css\"")
		_, err := b.Build(ctx, tree, kit)
		d := diagnostic(t, err, "graph.cssOutsideClient")
		require.Equal(t, "src/server.ts", d.File)
		require.Equal(t, lineOf(t, tree, "src/server.ts", `"./global.css"`), d.Line)
		require.Contains(t, d.Message, "server graph")
	})

	t.Run("a CSS module in the contracts graph is refused too", func(t *testing.T) {
		tree := styledTree()
		replace(t, tree, "src/contracts.ts", `import { query, t } from "@iota-uz/sdk/runtime"`, "import { query, t } from \"@iota-uz/sdk/runtime\"\nimport styles from \"./Board.module.css\"\nexport const s = styles")
		_, err := b.Build(ctx, tree, kit)
		d := diagnostic(t, err, "graph.cssOutsideClient")
		require.Equal(t, "src/contracts.ts", d.File)
		require.Contains(t, d.Message, "contracts graph")
	})

	t.Run("the kit belongs to the client graph", func(t *testing.T) {
		tree := styledTree()
		replace(t, tree, "src/server.ts", `import * as c from "./contracts"`, "import * as c from \"./contracts\"\nimport { Button } from \"@iota-uz/sdk/solid-ui\"\nexport const b = Button")
		_, err := b.Build(ctx, tree, kit)
		d := diagnostic(t, err, "graph.clientInServer")
		require.Equal(t, "src/server.ts", d.File)
		require.Equal(t, lineOf(t, tree, "src/server.ts", `"@iota-uz/sdk/solid-ui"`), d.Line)

		tree = styledTree()
		replace(t, tree, "src/contracts.ts", `import { query, t } from "@iota-uz/sdk/runtime"`, "import { query, t } from \"@iota-uz/sdk/runtime\"\nimport { Button } from \"@iota-uz/sdk/solid-ui\"\nexport const b = Button")
		_, err = b.Build(ctx, tree, kit)
		require.Equal(t, "src/contracts.ts", diagnostic(t, err, "graph.impureContract").File)
	})

	t.Run("a CSS syntax error is located in the stylesheet", func(t *testing.T) {
		tree := styledTree()
		tree["src/Board.module.css"] = []byte(".board { display: grid;\n.oops {{ \n")
		_, err := b.Build(ctx, tree, kit)
		var failed *build.FailedError
		require.ErrorAs(t, err, &failed)
		var located bool
		for _, d := range failed.Diagnostics {
			if d.File == "src/Board.module.css" && d.Line > 0 {
				located = true
			}
		}
		require.True(t, located, "%+v", failed.Diagnostics)
	})

	t.Run("a module stylesheet without its type declaration says how to declare it", func(t *testing.T) {
		tree := styledTree()
		delete(tree, "src/css-modules.d.ts")
		_, err := b.Build(ctx, tree, kit)
		d := diagnostic(t, err, "ts.2307")
		require.Equal(t, "src/Board.tsx", d.File)
		require.Contains(t, d.Message, `declare module "*.module.css"`)
	})
}

// A package without a client graph, or whose client uses no utility, ships no utilities sheet.
func TestNoUtilitiesNoSheet(t *testing.T) {
	tree := styledTree()
	tree["src/Board.tsx"] = []byte(`import styles from "./Board.module.css"
export default function Board() {
  return <main class={styles.board}>Treaty</main>
}
`)
	res, m := buildStyled(t, tree)
	require.NotContains(t, res.Artifacts, "client/tailwind.css")
	require.Len(t, m.CSS, 1)
	require.NotContains(t, m.Shared, "@iota-uz/sdk/solid-ui", "a version that does not import the kit does not list it")
}

// Package tests run on Node: a test may import client code that uses the kit and CSS modules. The kit is an inert stand-in
// there (it only renders in the browser host), the module's class names are the real scoped ones, and a plain stylesheet
// import is harmless (the client graph's own build refuses it).
func TestPackageTestsCanImportStyledClientCode(t *testing.T) {
	tree := styledTree()
	tree["src/board.test.ts"] = []byte(`import assert from "node:assert/strict"
import { test } from "@iota-uz/sdk/runtime/testing"
import Board from "./Board"
import styles from "./Board.module.css"

test("the styled page module loads on the test host", () => {
  assert.equal(typeof Board, "function")
  assert.match(styles.board ?? "", /^Board_[0-9a-f]{8}_board$/)
})
`)
	b := newBuilder(t)
	report, err := b.Test(context.Background(), tree, nativeKit(t, b))
	require.NoError(t, err)
	require.True(t, report.Passed, "%+v", report.Cases)
	require.Len(t, report.Cases, 1)
}

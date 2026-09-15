package solidgen

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestWatchReconcilesAChangedFeatureGraph(t *testing.T) {
	t.Parallel()
	root := fixtureModule(t, `package feature
import solid "github.com/iota-uz/iota-sdk/pkg/clienthost/solid"
type Props struct{}
var Screen = solid.Import[Props]("./Screen.tsx")
`, `export default function Screen() { return null }
`)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	updates := make(chan Result, 4)
	errors := make(chan error, 1)
	done := make(chan error, 1)
	go func() {
		done <- Watch(ctx, Config{ModuleRoot: root}, func(result Result, err error) {
			if err != nil {
				select {
				case errors <- err:
				default:
				}
				return
			}
			updates <- result
		})
	}()

	select {
	case initial := <-updates:
		require.Equal(t, 1, initial.Features)
	case err := <-errors:
		require.NoError(t, err)
	case <-time.After(10 * time.Second):
		t.Fatal("watch did not perform initial generation")
	}
	require.NoError(t, os.WriteFile(filepath.Join(root, "feature", "Second.tsx"), []byte("export default function Second() { return null }\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "feature", "feature.go"), []byte(`package feature
import solid "github.com/iota-uz/iota-sdk/pkg/clienthost/solid"
type Props struct{}
var Screen = solid.Import[Props]("./Screen.tsx")
var Second = solid.Import[Props]("./Second.tsx")
`), 0o644))

	deadline := time.After(10 * time.Second)
	for {
		select {
		case updated := <-updates:
			if updated.Features == 2 {
				require.FileExists(t, filepath.Join(root, "feature", "Second.solid.generated.ts"))
				cancel()
				require.NoError(t, <-done)
				return
			}
		case err := <-errors:
			require.NoError(t, err)
		case <-deadline:
			t.Fatal("watch did not reconcile the changed graph")
		}
	}
}

func TestRunGeneratesColocatedBindingsAndCatalog(t *testing.T) {
	t.Parallel()
	root := fixtureModule(t, `package feature

import solidui "github.com/iota-uz/iota-sdk/pkg/clienthost/solid"

type Mode string
const (
	ModeCreate Mode = "create"
	ModeEdit Mode = "edit"
)

type Props struct {
	ID string `+"`json:\"id\"`"+`
	Mode Mode `+"`json:\"mode\"`"+`
	Optional *string `+"`json:\"optional,omitempty\"`"+`
}
type LoadRequest struct { ID string `+"`json:\"id\"`"+` }
type LoadResponse struct { Values []string `+"`json:\"values\"`"+` }
type SaveRequest struct { Draft Props `+"`json:\"draft\"`"+` }
type SaveResponse struct { Saved bool `+"`json:\"saved\"`"+` }

var Screen = solidui.Import[Props]("./Screen.tsx", solidui.WithRPC(
	solidui.Query[LoadRequest, LoadResponse]("product.load", solidui.Cacheable(2)),
	solidui.Action[SaveRequest, SaveResponse]("product.save", solidui.Invalidates("product.load")),
))
`, `export default function Screen() { return null }
`)

	result, err := Run(Config{ModuleRoot: root, Catalog: "web/src/solid-features.generated.ts"})
	require.NoError(t, err)
	require.Equal(t, 1, result.Features)
	require.Contains(t, result.Files, "feature/Screen.solid.generated.ts")
	require.Contains(t, result.Files, "web/src/solid-features.generated.ts")

	bindings := readFixture(t, root, "feature/Screen.solid.generated.ts")
	require.Contains(t, bindings, `export type Props = { "id": string; "mode": "create" | "edit"; "optional"?: string }`)
	require.Contains(t, bindings, `"product.load": { params: { "id": string }; result: { "values": Array<string> } }`)
	require.Contains(t, bindings, `kind: "query", cacheable: true, maxRetries: 2`)
	require.Contains(t, bindings, `kind: "mutation", cacheable: false, maxRetries: 0, invalidates: ["product.load"]`)
	require.Contains(t, bindings, `const checked: Component<SolidRouteProps<Props>> = Screen`)

	catalog := readFixture(t, root, "web/src/solid-features.generated.ts")
	require.Contains(t, catalog, `() => import("../../feature/Screen.solid.generated")`)
	_, err = Run(Config{ModuleRoot: root, Catalog: "web/src/solid-features.generated.ts", Check: true})
	require.NoError(t, err)
}

func TestRunCheckReportsDriftAndReconciliationOnlyDeletesOwnedFiles(t *testing.T) {
	t.Parallel()
	root := fixtureModule(t, `package feature
import solid "github.com/iota-uz/iota-sdk/pkg/clienthost/solid"
type Props struct { Name string `+"`json:\"name\"`"+` }
var Screen = solid.Import[Props]("./Screen.tsx")
`, `export default function Screen() { return null }
`)
	require.NoError(t, os.WriteFile(filepath.Join(root, "feature", "Old.solid.generated.ts"), []byte(generatedHeader+"old\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "feature", "Keep.solid.generated.ts"), []byte("// user-owned\n"), 0o644))

	_, err := Run(Config{ModuleRoot: root, Catalog: "web/src/solid-features.generated.ts", Check: true})
	require.ErrorContains(t, err, "solid generated files are stale")
	require.FileExists(t, filepath.Join(root, "feature", "Old.solid.generated.ts"))

	_, err = Run(Config{ModuleRoot: root, Catalog: "web/src/solid-features.generated.ts"})
	require.NoError(t, err)
	require.NoFileExists(t, filepath.Join(root, "feature", "Old.solid.generated.ts"))
	require.FileExists(t, filepath.Join(root, "feature", "Keep.solid.generated.ts"))
}

func TestRunRejectsMissingSourceUnsupportedWireTypeAndDuplicateFeature(t *testing.T) {
	t.Parallel()
	t.Run("missing source", func(t *testing.T) {
		t.Parallel()
		root := fixtureModule(t, `package feature
import solid "github.com/iota-uz/iota-sdk/pkg/clienthost/solid"
type Props struct{}
var Screen = solid.Import[Props]("./Missing.tsx")
`, `export default function Screen() { return null }
`)
		_, err := Run(Config{ModuleRoot: root})
		require.ErrorContains(t, err, `Solid source "./Missing.tsx"`)
		require.ErrorContains(t, err, "feature.go:")
	})

	t.Run("unsupported wire type", func(t *testing.T) {
		t.Parallel()
		root := fixtureModule(t, `package feature
import solid "github.com/iota-uz/iota-sdk/pkg/clienthost/solid"
type Props struct { Values map[int]string `+"`json:\"values\"`"+` }
var Screen = solid.Import[Props]("./Screen.tsx")
`, `export default function Screen() { return null }
`)
		_, err := Run(Config{ModuleRoot: root})
		require.ErrorContains(t, err, "JSON object keys must be strings")
		require.ErrorContains(t, err, "feature.go:")
	})

	t.Run("custom marshaler", func(t *testing.T) {
		t.Parallel()
		root := fixtureModule(t, `package feature
import solid "github.com/iota-uz/iota-sdk/pkg/clienthost/solid"
type Custom struct{}
func (Custom) MarshalJSON() ([]byte, error) { return []byte("{}"), nil }
type Props struct { Value Custom `+"`json:\"value\"`"+` }
var Screen = solid.Import[Props]("./Screen.tsx")
`, `export default function Screen() { return null }
`)
		_, err := Run(Config{ModuleRoot: root})
		require.ErrorContains(t, err, "defines MarshalJSON; use an explicit wire field type")
	})

	t.Run("duplicate feature", func(t *testing.T) {
		t.Parallel()
		root := fixtureModule(t, `package feature
import solid "github.com/iota-uz/iota-sdk/pkg/clienthost/solid"
type Props struct{}
var First = solid.Import[Props]("./Screen.tsx")
var Second = solid.Import[Props]("./Screen.tsx")
`, `export default function Screen() { return null }
`)
		_, err := Run(Config{ModuleRoot: root})
		require.ErrorContains(t, err, "duplicate Solid feature")
	})
}

func TestRunRejectsDynamicSourcesAndInvalidRPCContracts(t *testing.T) {
	t.Parallel()
	t.Run("dynamic source", func(t *testing.T) {
		t.Parallel()
		root := fixtureModule(t, `package feature
import solid "github.com/iota-uz/iota-sdk/pkg/clienthost/solid"
type Props struct{}
var source = "./Screen.tsx"
var Screen = solid.Import[Props](source)
`, `export default function Screen() { return null }
`)
		_, err := Run(Config{ModuleRoot: root})
		require.ErrorContains(t, err, "source must be a string literal")
	})

	t.Run("escaping source", func(t *testing.T) {
		t.Parallel()
		root := fixtureModule(t, `package feature
import solid "github.com/iota-uz/iota-sdk/pkg/clienthost/solid"
type Props struct{}
var Screen = solid.Import[Props]("../Screen.tsx")
`, `export default function Screen() { return null }
`)
		_, err := Run(Config{ModuleRoot: root})
		require.ErrorContains(t, err, "inside the declaring Go package")
	})

	t.Run("non-clean source", func(t *testing.T) {
		t.Parallel()
		root := fixtureModule(t, `package feature
import solid "github.com/iota-uz/iota-sdk/pkg/clienthost/solid"
type Props struct{}
var Screen = solid.Import[Props]("./a/../Screen.tsx")
`, `export default function Screen() { return null }
`)
		_, err := Run(Config{ModuleRoot: root})
		require.ErrorContains(t, err, "clean relative ./path.tsx")
	})

	t.Run("duplicate rpc", func(t *testing.T) {
		t.Parallel()
		root := fixtureModule(t, `package feature
import solid "github.com/iota-uz/iota-sdk/pkg/clienthost/solid"
type Props struct{}
var Screen = solid.Import[Props]("./Screen.tsx", solid.WithRPC(
	solid.Query[Props, Props]("same"),
	solid.Action[Props, Props]("same"),
))
`, `export default function Screen() { return null }
`)
		_, err := Run(Config{ModuleRoot: root})
		require.ErrorContains(t, err, `duplicate RPC method "same"`)
	})

	t.Run("invalid retry policy", func(t *testing.T) {
		t.Parallel()
		root := fixtureModule(t, `package feature
import solid "github.com/iota-uz/iota-sdk/pkg/clienthost/solid"
type Props struct{}
var Screen = solid.Import[Props]("./Screen.tsx", solid.WithRPC(
	solid.Action[Props, Props]("save", solid.Cacheable(1)),
))
`, `export default function Screen() { return null }
`)
		_, err := Run(Config{ModuleRoot: root})
		require.ErrorContains(t, err, "valid only for query")
	})

	t.Run("unknown invalidation", func(t *testing.T) {
		t.Parallel()
		root := fixtureModule(t, `package feature
import solid "github.com/iota-uz/iota-sdk/pkg/clienthost/solid"
type Props struct{}
var Screen = solid.Import[Props]("./Screen.tsx", solid.WithRPC(
	solid.Action[Props, Props]("save", solid.Invalidates("missing")),
))
`, `export default function Screen() { return null }
`)
		_, err := Run(Config{ModuleRoot: root})
		require.ErrorContains(t, err, `invalidates unknown query "missing"`)
	})
}

func TestJSONFieldOnlySkipsTheExactDashTag(t *testing.T) {
	t.Parallel()
	name, _, _, skipped := jsonField("Value", `json:"-,"`)
	require.Equal(t, "-", name)
	require.False(t, skipped)
	dashName, optional, asString, dashSkipped := jsonField("Value", `json:"-"`)
	require.Empty(t, dashName)
	require.False(t, optional)
	require.False(t, asString)
	require.True(t, dashSkipped)
}

func fixtureModule(t *testing.T, goSource, tsxSource string) string {
	t.Helper()
	root := t.TempDir()
	_, current, _, ok := runtime.Caller(0)
	require.True(t, ok)
	sdkRoot := filepath.Clean(filepath.Join(filepath.Dir(current), "..", ".."))
	goMod := "module example.test/app\n\ngo 1.24.10\n\nrequire github.com/iota-uz/iota-sdk v0.0.0\nreplace github.com/iota-uz/iota-sdk => " + filepath.ToSlash(sdkRoot) + "\n"
	require.NoError(t, os.WriteFile(filepath.Join(root, "go.mod"), []byte(goMod), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(root, "feature"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "feature", "feature.go"), []byte(goSource), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "feature", "Screen.tsx"), []byte(tsxSource), 0o644))
	return root
}

func readFixture(t *testing.T, root, name string) string {
	t.Helper()
	contents, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(name)))
	require.NoError(t, err)
	return strings.ReplaceAll(string(contents), "\r\n", "\n")
}

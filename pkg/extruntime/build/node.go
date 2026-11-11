package build

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
)

// Config configures the Node build worker adapter.
type Config struct {
	// Node is the node executable (default "node", resolved on PATH).
	Node string
	// WorkerScript is the path of the build worker, web/runtime dist
	// "build/worker.mjs" (see web/runtime/scripts/build-worker.mjs).
	WorkerScript string
	// WorkDir holds the per-operation temp directories and the npm cache
	// (default: the OS temp directory).
	WorkDir string
	// Timeout bounds one operation, including npm ci (default 5 minutes).
	Timeout time.Duration
	// NPM is the npm executable used for `npm ci --ignore-scripts` (default "npm").
	NPM string
	// NodePath is added to NODE_PATH of the worker; it must reach the native
	// `esbuild` package when it is not installed next to the worker script.
	NodePath string
	// Parallel bounds concurrent operations (default 2).
	Parallel int
}

// NodeBuilder runs every operation in a fresh build worker process: a Node
// program separate from the invocation pool. It implements Builder and
// KitGenerator.
type NodeBuilder struct {
	cfg Config
	sem chan struct{}
}

var (
	_ Builder      = (*NodeBuilder)(nil)
	_ KitGenerator = (*NodeBuilder)(nil)
)

// NewNodeBuilder validates cfg and returns the adapter.
func NewNodeBuilder(cfg Config) (*NodeBuilder, error) {
	if cfg.Node == "" {
		cfg.Node = "node"
	}
	node, err := exec.LookPath(cfg.Node)
	if err != nil {
		return nil, fmt.Errorf("build: node executable: %w", err)
	}
	cfg.Node = node
	if cfg.NPM == "" {
		cfg.NPM = "npm"
	}
	if cfg.WorkerScript == "" {
		return nil, errors.New("build: Config.WorkerScript is required")
	}
	script, err := filepath.Abs(cfg.WorkerScript)
	if err != nil {
		return nil, fmt.Errorf("build: worker script: %w", err)
	}
	if _, err := os.Stat(script); err != nil {
		return nil, fmt.Errorf("build: worker script: %w", err)
	}
	cfg.WorkerScript = script
	if cfg.WorkDir == "" {
		cfg.WorkDir = os.TempDir()
	}
	if err := os.MkdirAll(cfg.WorkDir, 0o755); err != nil {
		return nil, fmt.Errorf("build: work dir: %w", err)
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 5 * time.Minute
	}
	if cfg.Parallel <= 0 {
		cfg.Parallel = 2
	}
	return &NodeBuilder{cfg: cfg, sem: make(chan struct{}, cfg.Parallel)}, nil
}

// ---- wire types of the worker protocol (web/runtime/src/build/protocol.ts)

type kitPayload struct {
	Hash    string                       `json:"hash"`
	Modules map[string]map[string]string `json:"modules"`
}

type packageRequest struct {
	Op     string     `json:"op"`
	Dir    string     `json:"dir"`
	OutDir string     `json:"outDir"`
	Kit    kitPayload `json:"kit"`
}

type kitSourcePayload struct {
	Namespace       string              `json:"namespace"`
	Resources       []manifest.Resource `json:"resources"`
	ContractsModule string              `json:"contractsModule,omitempty"`
	Lists           []string            `json:"lists,omitempty"`
}

type kitRequest struct {
	Op      string             `json:"op"`
	Sources []kitSourcePayload `json:"sources"`
}

type buildResponse struct {
	Diagnostics []Diagnostic     `json:"diagnostics"`
	Manifest    *json.RawMessage `json:"manifest"`
	Artifacts   []string         `json:"artifacts"`
}

type testResponse struct {
	Diagnostics []Diagnostic `json:"diagnostics"`
	Cases       []TestCase   `json:"cases"`
}

type kitResponse struct {
	Diagnostics []Diagnostic                 `json:"diagnostics"`
	Hash        string                       `json:"hash"`
	Modules     map[string]map[string]string `json:"modules"`
}

// ---- Builder

// Check typechecks the tree and validates graphs and the manifest without
// returning artifacts.
func (b *NodeBuilder) Check(ctx context.Context, src Tree, kit Kit) ([]Diagnostic, error) {
	var resp buildResponse
	if _, err := b.runPackage(ctx, "check", src, kit, &resp, nil); err != nil {
		return nil, err
	}
	return resp.Diagnostics, nil
}

// Build returns *Failed when diagnostics contain errors.
func (b *NodeBuilder) Build(ctx context.Context, src Tree, kit Kit) (Result, error) {
	var resp buildResponse
	var artifacts Tree
	_, err := b.runPackage(ctx, "build", src, kit, &resp, func(out string) error {
		var rerr error
		artifacts, rerr = readArtifacts(out, resp.Artifacts)
		return rerr
	})
	if err != nil {
		return Result{}, err
	}
	res := Result{Diagnostics: resp.Diagnostics, SourceHash: HashTree(src)}
	if hasErrors(res.Diagnostics) {
		return res, &Failed{Diagnostics: res.Diagnostics}
	}
	if resp.Manifest == nil {
		return res, errors.New("build: worker returned no manifest")
	}
	dec := json.NewDecoder(bytes.NewReader(*resp.Manifest))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&res.Manifest); err != nil {
		return res, fmt.Errorf("build: manifest does not match pkg/extruntime/manifest: %w", err)
	}
	if err := res.Manifest.Validate(); err != nil {
		res.Diagnostics = append(res.Diagnostics, Diagnostic{File: "package.json", Severity: SeverityError, Code: "manifest.invalid", Message: err.Error()})
		return res, &Failed{Diagnostics: res.Diagnostics}
	}
	res.Artifacts = artifacts
	res.ArtifactHash = HashTree(artifacts)
	return res, nil
}

// Test runs the package's *.test.ts files against the synthetic test host.
func (b *NodeBuilder) Test(ctx context.Context, src Tree, kit Kit) (TestReport, error) {
	var resp testResponse
	if _, err := b.runPackage(ctx, "test", src, kit, &resp, nil); err != nil {
		return TestReport{}, err
	}
	report := TestReport{Passed: !hasErrors(resp.Diagnostics), Cases: resp.Cases}
	for _, c := range resp.Cases {
		if !c.Passed {
			report.Passed = false
		}
	}
	return report, nil
}

// GenerateKit renders `@iota-kit/<namespace>` modules for the sources.
func (b *NodeBuilder) GenerateKit(ctx context.Context, sources []KitSource) (Kit, error) {
	req := kitRequest{Op: "kit"}
	for _, s := range sources {
		resources := s.Resources
		if resources == nil {
			resources = []manifest.Resource{} // a source with only lists: the worker iterates an array, not null
		}
		req.Sources = append(req.Sources, kitSourcePayload{Namespace: s.Namespace, Resources: resources, ContractsModule: string(s.ContractsModule), Lists: s.Lists})
	}
	var resp kitResponse
	if err := b.run(ctx, "", req, &resp); err != nil {
		return Kit{}, err
	}
	if hasErrors(resp.Diagnostics) {
		return Kit{}, &Failed{Diagnostics: resp.Diagnostics}
	}
	kit := Kit{Hash: resp.Hash, Modules: make(map[string]Tree, len(resp.Modules))}
	for name, files := range resp.Modules {
		t := Tree{}
		for p, text := range files {
			t[p] = []byte(text)
		}
		kit.Modules[name] = t
	}
	return kit, nil
}

// ---- plumbing

func hasErrors(ds []Diagnostic) bool {
	for _, d := range ds {
		if d.Severity == SeverityError {
			return true
		}
	}
	return false
}

// runPackage materializes src, installs npm dependencies when the package has
// any, runs the worker op and decodes its response into out. collect, when
// set, runs before the temp directory is removed.
func (b *NodeBuilder) runPackage(ctx context.Context, op string, src Tree, kit Kit, out any, collect func(outDir string) error) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, b.cfg.Timeout)
	defer cancel()
	tmp, err := os.MkdirTemp(b.cfg.WorkDir, "iota-build-")
	if err != nil {
		return "", fmt.Errorf("build: temp dir: %w", err)
	}
	defer os.RemoveAll(tmp)
	dir, outDir := filepath.Join(tmp, "src"), filepath.Join(tmp, "out")
	if err := materialize(dir, src); err != nil {
		return "", err
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return "", fmt.Errorf("build: out dir: %w", err)
	}
	if diag, err := b.install(ctx, dir, src); err != nil {
		return "", err
	} else if diag != nil {
		return "", b.installFailure(op, out, *diag)
	}
	req := packageRequest{Op: op, Dir: dir, OutDir: outDir, Kit: kitPayload{Hash: kit.Hash, Modules: map[string]map[string]string{}}}
	for name, files := range kit.Modules {
		m := make(map[string]string, len(files))
		for p, c := range files {
			m[p] = string(c)
		}
		req.Kit.Modules[name] = m
	}
	if err := b.run(ctx, tmp, req, out); err != nil {
		return "", err
	}
	if collect != nil {
		if err := collect(filepath.Join(outDir, "artifacts")); err != nil {
			return "", err
		}
	}
	return outDir, nil
}

// installFailure reports a failed dependency install as a diagnostic in the
// response of the operation.
func (b *NodeBuilder) installFailure(op string, out any, d Diagnostic) error {
	switch r := out.(type) {
	case *buildResponse:
		r.Diagnostics = []Diagnostic{d}
		if op == "build" {
			return &Failed{Diagnostics: r.Diagnostics}
		}
		return nil
	default:
		return &Failed{Diagnostics: []Diagnostic{d}}
	}
}

func (b *NodeBuilder) run(ctx context.Context, home string, req any, out any) error {
	select {
	case b.sem <- struct{}{}:
		defer func() { <-b.sem }()
	case <-ctx.Done():
		return ctx.Err()
	}
	payload, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("build: encode request: %w", err)
	}
	cmd := exec.CommandContext(ctx, b.cfg.Node, "--no-warnings", b.cfg.WorkerScript)
	cmd.Stdin = bytes.NewReader(payload)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &limitedBuffer{buf: &stderr, max: 64 << 10}
	cmd.Env = b.env(home)
	ownProcessGroup(cmd)
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return fmt.Errorf("build: worker: %w", ctx.Err())
		}
		return fmt.Errorf("build: worker failed: %w\n%s", err, strings.TrimSpace(stderr.String()))
	}
	if err := json.Unmarshal(stdout.Bytes(), out); err != nil {
		return fmt.Errorf("build: worker response: %w\n%s", err, strings.TrimSpace(stderr.String()))
	}
	return nil
}

func (b *NodeBuilder) env(home string) []string {
	if home == "" {
		home = b.cfg.WorkDir
	}
	env := []string{"PATH=" + os.Getenv("PATH"), "TZ=UTC", "HOME=" + home}
	if b.cfg.NodePath != "" {
		env = append(env, "NODE_PATH="+b.cfg.NodePath)
	}
	return env
}

type limitedBuffer struct {
	buf *bytes.Buffer
	max int
}

func (l *limitedBuffer) Write(p []byte) (int, error) {
	if room := l.max - l.buf.Len(); room > 0 {
		if len(p) > room {
			l.buf.Write(p[:room])
		} else {
			l.buf.Write(p)
		}
	}
	return len(p), nil
}

// materialize writes the tree below dir, rejecting paths that escape it.
func materialize(dir string, t Tree) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("build: source dir: %w", err)
	}
	paths := make([]string, 0, len(t))
	for p := range t {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	for _, p := range paths {
		clean := path.Clean(p)
		if clean != p || p == "." || path.IsAbs(p) || strings.HasPrefix(p, "../") || p == ".." {
			return fmt.Errorf("build: invalid source path %q", p)
		}
		if p == "node_modules" || strings.HasPrefix(p, "node_modules/") || strings.Contains(p, "/node_modules/") {
			return fmt.Errorf("build: source path %q is inside node_modules; dependencies are installed from the lockfile", p)
		}
		target := filepath.Join(dir, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return fmt.Errorf("build: %w", err)
		}
		if err := os.WriteFile(target, t[p], 0o644); err != nil {
			return fmt.Errorf("build: %w", err)
		}
	}
	return nil
}

// providedDependencies are served by the build worker itself (SDK typings and
// runtime), never installed from the registry.
var providedDependencies = map[string]bool{"@iota-uz/sdk": true}

// install runs `npm ci --ignore-scripts` from the lockfile when package.json
// declares dependencies beyond the provided ones. A missing lockfile or a
// failed install is reported as a diagnostic, not as an infrastructure error.
func (b *NodeBuilder) install(ctx context.Context, dir string, src Tree) (*Diagnostic, error) {
	raw, ok := src["package.json"]
	if !ok {
		return nil, nil
	}
	var pkg struct {
		Dependencies         map[string]string `json:"dependencies"`
		DevDependencies      map[string]string `json:"devDependencies"`
		OptionalDependencies map[string]string `json:"optionalDependencies"`
	}
	if err := json.Unmarshal(raw, &pkg); err != nil {
		return nil, nil // the worker reports an invalid package.json with a location
	}
	needed := false
	for _, set := range []map[string]string{pkg.Dependencies, pkg.DevDependencies, pkg.OptionalDependencies} {
		for name := range set {
			if !providedDependencies[name] {
				needed = true
			}
		}
	}
	if !needed {
		return nil, nil
	}
	if _, lock := src["package-lock.json"]; !lock {
		if _, shrink := src["npm-shrinkwrap.json"]; !shrink {
			return &Diagnostic{File: "package.json", Severity: SeverityError, Code: "npm.lockfileMissing", Message: "package.json has dependencies but the tree has no package-lock.json; dependencies are installed with `npm ci` from the lockfile"}, nil
		}
	}
	cache := filepath.Join(b.cfg.WorkDir, ".npm-cache")
	cmd := exec.CommandContext(ctx, b.cfg.NPM, "ci", "--ignore-scripts", "--no-audit", "--no-fund", "--loglevel=error")
	cmd.Dir = dir
	cmd.Env = append(b.env(filepath.Dir(dir)), "npm_config_cache="+cache, "npm_config_update_notifier=false")
	ownProcessGroup(cmd)
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("build: npm ci: %w", ctx.Err())
		}
		msg := strings.TrimSpace(out.String())
		if len(msg) > 2000 {
			msg = msg[:2000]
		}
		return &Diagnostic{File: "package.json", Severity: SeverityError, Code: "npm.install", Message: "npm ci failed: " + msg}, nil
	}
	return nil, nil
}

func readArtifacts(root string, paths []string) (Tree, error) {
	t := make(Tree, len(paths))
	for _, p := range paths {
		clean := path.Clean(p)
		if clean != p || path.IsAbs(p) || strings.HasPrefix(p, "../") {
			return nil, fmt.Errorf("build: worker returned invalid artifact path %q", p)
		}
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(p)))
		if err != nil {
			return nil, fmt.Errorf("build: read artifact: %w", err)
		}
		t[p] = data
	}
	return t, nil
}

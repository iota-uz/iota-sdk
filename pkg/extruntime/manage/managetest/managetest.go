// Package managetest provides doubles of the build worker for tests of the
// management service and its consumers: a Builder that takes the manifest from
// the tree's manifest.json and a kit generator that renders refs and types
// from the catalog sources.
package managetest

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/artifact"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/build"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
)

// Builder implements build.Builder: the manifest comes from the tree's
// manifest.json, artifacts are server.mjs and contracts.mjs, and a file
// containing BAD_TYPE produces an error diagnostic.
type Builder struct {
	mu                    sync.Mutex
	Checks, Builds, Tests int
	LastKit               build.Kit
	TreesSeen             []build.Tree
	FailTests             bool
	OverrideArtifactHash  string
}

func (b *Builder) diagnostics(src build.Tree) []build.Diagnostic {
	var out []build.Diagnostic
	paths := make([]string, 0, len(src))
	for p := range src {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	for _, p := range paths {
		if strings.Contains(string(src[p]), "BAD_TYPE") {
			out = append(out, build.Diagnostic{File: p, Line: 1, Severity: build.SeverityError, Code: "ts.2345", Message: "BAD_TYPE is not assignable"})
		}
		if strings.Contains(string(src[p]), "WARN_ONLY") {
			out = append(out, build.Diagnostic{File: p, Line: 1, Severity: build.SeverityWarning, Code: "lint.warn", Message: "careful"})
		}
	}
	return out
}

// Check implements build.Builder.
func (b *Builder) Check(_ context.Context, src build.Tree, kit build.Kit) ([]build.Diagnostic, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.Checks++
	b.LastKit = kit
	return b.diagnostics(src), nil
}

// Build implements build.Builder.
func (b *Builder) Build(_ context.Context, src build.Tree, kit build.Kit) (build.Result, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.Builds++
	b.LastKit = kit
	b.TreesSeen = append(b.TreesSeen, src)
	ds := b.diagnostics(src)
	for _, d := range ds {
		if d.Severity == build.SeverityError {
			return build.Result{Diagnostics: ds}, &build.FailedError{Diagnostics: ds}
		}
	}
	var m manifest.Manifest
	if err := json.Unmarshal(src["manifest.json"], &m); err != nil {
		return build.Result{}, fmt.Errorf("manifest.json: %w", err)
	}
	arts := build.Tree{}
	for _, f := range []string{"server.mjs", "contracts.mjs"} {
		if c, ok := src[f]; ok {
			arts[f] = c
		}
	}
	// dist/... files are client chunks: they are served from the artifact root.
	for p, c := range src {
		if strings.HasPrefix(p, "dist/") {
			arts[strings.TrimPrefix(p, "dist/")] = c
		}
	}
	hash := artifact.TreeHash(arts)
	if b.OverrideArtifactHash != "" {
		hash = b.OverrideArtifactHash
	}
	return build.Result{Manifest: m, Artifacts: arts, SourceHash: "src-" + artifact.TreeHash(src), ArtifactHash: hash, Diagnostics: ds}, nil
}

// Test implements build.Builder.
func (b *Builder) Test(_ context.Context, src build.Tree, kit build.Kit) (build.TestReport, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.Tests++
	b.LastKit = kit
	return build.TestReport{Passed: !b.FailTests, Cases: []build.TestCase{{Name: "renews", File: "a.test.ts", Passed: !b.FailTests}}}, nil
}

// Kits implements build.KitGenerator: one module per source namespace with a
// declaration line per resource and the pure contracts module as index.js.
type Kits struct {
	mu      sync.Mutex
	Calls   int
	Sources []build.KitSource
}

// GenerateKit implements build.KitGenerator.
func (k *Kits) GenerateKit(_ context.Context, sources []build.KitSource) (build.Kit, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.Calls++
	k.Sources = sources
	kit := build.Kit{Modules: map[string]build.Tree{}}
	for _, s := range sources {
		var dts strings.Builder
		for _, r := range s.Resources {
			fmt.Fprintf(&dts, "export declare const %s: Ref<%q>; // %s\n", strings.ReplaceAll(r.ID, ".", "_"), r.Kind, r.ContractHash)
		}
		js := "// refs of " + s.Namespace + "\n" + string(s.ContractsModule)
		kit.Modules["@iota-kit/"+s.Namespace] = build.Tree{"index.js": []byte(js), "index.d.ts": []byte(dts.String())}
	}
	return kit, nil
}

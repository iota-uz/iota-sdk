// Package build turns a package source tree into an immutable version:
// three separately bundled ESM graphs (contracts, server, client), the
// extracted manifest and source-located diagnostics. The Builder runs in a
// dedicated build worker, never in the invocation pool.
package build

import (
	"context"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
)

// Tree is a package source tree: relative slash paths to file contents.
// It includes package.json and, when the package has npm dependencies, its
// lockfile.
type Tree map[string][]byte

type Severity string

const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
)

// Diagnostic points at the author's source, never at generated output.
type Diagnostic struct {
	File     string   `json:"file"`
	Line     int      `json:"line,omitempty"`
	Column   int      `json:"column,omitempty"`
	Severity Severity `json:"severity"`
	// Code is stable, e.g. "graph.serverInClient", "graph.impureContract",
	// "ts.2345", "manifest.duplicateId".
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Kit is the developer kit the package compiles against: public refs/types of
// core and other packages (catalog snapshot), never their handlers.
type Kit struct {
	// Hash identifies the catalog snapshot (contract resolution snapshot).
	Hash string
	// Modules maps a kit module name (e.g. "@iota-kit/core") to its files
	// (index.js with pure refs, index.d.ts with types).
	Modules map[string]Tree
}

// Result of a successful build. Artifacts is the immutable artifact tree
// loaded by workers and the browser: "server.mjs", "contracts.mjs",
// "client/<chunk>.js" and "client/manifest.json".
type Result struct {
	Manifest     manifest.Manifest `json:"manifest"`
	Artifacts    Tree              `json:"-"`
	SourceHash   string            `json:"sourceHash"`
	ArtifactHash string            `json:"artifactHash"`
	Diagnostics  []Diagnostic      `json:"diagnostics"`
}

// Builder is implemented by the Node build worker adapter.
type Builder interface {
	// Check typechecks and validates graphs without producing artifacts.
	Check(ctx context.Context, src Tree, kit Kit) ([]Diagnostic, error)
	// Build returns an error of type *Failed when diagnostics contain errors.
	Build(ctx context.Context, src Tree, kit Kit) (Result, error)
	// Test runs the package's synthetic tests (*.test.ts) against fake
	// effects; unknown external effects fail instead of reaching a network.
	Test(ctx context.Context, src Tree, kit Kit) (TestReport, error)
}

type TestCase struct {
	Name     string `json:"name"`
	File     string `json:"file"`
	Passed   bool   `json:"passed"`
	Message  string `json:"message,omitempty"`
	Duration int64  `json:"durationMs"`
}

type TestReport struct {
	Passed bool       `json:"passed"`
	Cases  []TestCase `json:"cases"`
}

// Failed carries the diagnostics of a failed build.
type Failed struct {
	Diagnostics []Diagnostic
}

func (f *Failed) Error() string {
	for _, d := range f.Diagnostics {
		if d.Severity == SeverityError {
			return "build failed: " + d.File + ": " + d.Message
		}
	}
	return "build failed"
}

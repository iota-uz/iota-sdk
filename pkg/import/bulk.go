// Package importpkg provides this package.
package importpkg

import (
	"context"

	"github.com/a-h/templ"
)

// ProgressSink receives progress updates from a BulkImportProcessor while it
// runs. A processor calls these methods synchronously from its own run
// goroutine, so they need not tolerate concurrent calls among themselves;
// implementations must, however, be safe to invoke concurrently with readers of
// the underlying state (e.g. RunStore.Get), which the storeSink achieves via the
// store lock.
type ProgressSink interface {
	// SetPhase records a human-readable label for the current stage of work
	// (e.g. "reading", "validating", "saving").
	SetPhase(phase string)
	// SetTotal sets the total number of work units expected.
	SetTotal(n int)
	// SetDone sets the absolute number of work units completed so far.
	SetDone(n int)
	// Add increments the number of completed work units by n.
	Add(n int)
}

// ImportRunOptions carries processor-agnostic switches for a single import run.
type ImportRunOptions struct {
	// DryRun, when true, asks the processor to validate and report without
	// persisting anything.
	DryRun bool
	// Options carries processor-specific knobs (e.g. sheet name, default
	// status). Keys and semantics are defined by the consuming processor.
	Options map[string]string
}

// ImportCount is a single labelled metric in an ImportResult, kept as an
// ordered slice entry so consumers control render order.
type ImportCount struct {
	// Label is a display label or i18n key for the metric.
	Label string
	// Value is the numeric value of the metric.
	Value int
}

// ImportResult summarises the outcome of a BulkImportProcessor run.
type ImportResult struct {
	// DryRun mirrors the run option, indicating whether anything was persisted.
	DryRun bool
	// Counts holds ordered, render-friendly metrics about the run.
	Counts []ImportCount
	// Warnings holds non-fatal messages surfaced to the user.
	Warnings []string
	// Detail is an optional consumer-supplied report component rendered in the
	// result view. It may be nil.
	Detail templ.Component
}

// BulkImportProcessor processes an entire import file in one call, reporting
// progress through the supplied sink and returning a summary result. The
// processor owns reading the file at filePath; the library makes no assumptions
// about its format.
type BulkImportProcessor interface {
	// Process runs the import described by opts against the file at filePath,
	// reporting progress through sink. It returns an ImportResult on success or
	// an error on failure.
	Process(ctx context.Context, filePath string, opts ImportRunOptions, sink ProgressSink) (ImportResult, error)
}

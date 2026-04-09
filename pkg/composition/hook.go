package composition

import "context"

// Hook is a lifecycle attachment point. Start runs at engine startup time and
// returns an optional StopFn that the engine invokes during teardown.
//
// The pattern (Start returns stop) eliminates the need for component structs
// to stash cancel funcs, channels, or other cross-phase state in side-channel
// fields. Whatever the teardown needs to clean up is captured by the closure
// Start returns.
//
// The returned StopFn may be nil if the resource has nothing to tear down.
type Hook struct {
	// Name identifies the hook for logging and error messages. Required.
	Name string

	// Start runs once at engine startup. The returned StopFn (may be nil)
	// is called once during shutdown.
	Start func(ctx context.Context) (StopFn, error)
}

// StopFn is the teardown closure returned by Hook.Start. The engine invokes
// it during Engine.Stop with a shutdown context.
type StopFn func(ctx context.Context) error

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
// each StopFn in reverse registration order during composition.Stop.
type StopFn func(ctx context.Context) error

type lifecycleContextKey struct{}

// WithLifecycleContext attaches the application lifecycle context to ctx.
//
// The context passed to Hook.Start is short-lived (it carries the startup
// timeout), so a hook that launches a background loop must not bind that loop
// to it. The bootstrap runtime attaches its lifecycle context here; it stays
// alive for the whole process and is cancelled as soon as graceful shutdown
// begins, before the stop closures run.
func WithLifecycleContext(ctx, lifecycle context.Context) context.Context {
	if lifecycle == nil {
		return ctx
	}
	return context.WithValue(ctx, lifecycleContextKey{}, lifecycle)
}

// LifecycleContext returns the application lifecycle context attached with
// WithLifecycleContext. Periodic tasks started from Hook.Start should select on
// its Done channel to stop accepting new work once shutdown begins. When no
// lifecycle context is attached, it returns ctx without its cancellation and
// deadline, so the caller still gets a context that outlives startup.
func LifecycleContext(ctx context.Context) context.Context {
	if lifecycle, ok := ctx.Value(lifecycleContextKey{}).(context.Context); ok && lifecycle != nil {
		return lifecycle
	}
	return context.WithoutCancel(ctx)
}

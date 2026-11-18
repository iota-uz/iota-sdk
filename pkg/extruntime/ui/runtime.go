package ui

import (
	"context"
	"fmt"
	"sync"

	"github.com/sirupsen/logrus"

	"github.com/iota-uz/iota-sdk/pkg/clienthost"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/originguard"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
)

// Config wires the UI slice.
type Config struct {
	// Source reports activated client manifests and open bindings. Required.
	Source Source
	// Artifacts serves the immutable client chunks. Required.
	Artifacts ArtifactStore
	// Invoker runs commands and queries for the browser. Required for
	// /_ext/rpc.
	Invoker Invoker
	// Catalog resolves contract IDs so the browser can only call queries and
	// commands, and the contract permission is enforced. Required with Invoker.
	Catalog spi.Catalog
	// Host describes the runtime host bundle served to the browser (entry
	// module URL, styles, SDK identity). Required.
	Host clienthost.Manifest
	// NativeRoutes names the native pages a screen binding may replace.
	// A route the host also declares as a typed contract (a native resource
	// of kind route, with params) is resolved through Catalog: a package
	// page replacing it receives typed path parameters.
	NativeRoutes NativeRoutes
	// Origins decides which sites may make state-changing requests to
	// /_ext/rpc (default: this origin only; see package originguard).
	Origins *originguard.Guard
	Logger  logrus.FieldLogger
}

func (c *Config) validate() error {
	switch {
	case c.Source == nil:
		return fmt.Errorf("extruntime ui: Source is required")
	case c.Artifacts == nil:
		return fmt.Errorf("extruntime ui: Artifacts is required")
	case c.Invoker != nil && c.Catalog == nil:
		return fmt.Errorf("extruntime ui: Catalog is required with Invoker")
	}
	return c.Host.Validate()
}

// Runtime holds the cached index shared by the controller, the navigation
// projection, the shell contributor and the column source.
type Runtime struct {
	cfg Config
	// styles keeps what the client manifests of the active artifacts say
	// about their stylesheets (see styles.go).
	styles versionStyles

	mu sync.Mutex
	ix *Index
	// prev is the index the current one replaced: a request that was routed
	// by it and reaches its handler after the rebuild is still served.
	prev *Index
}

// NewRuntime validates cfg.
func NewRuntime(cfg Config) (*Runtime, error) {
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	if cfg.Logger == nil {
		cfg.Logger = logrus.StandardLogger()
	}
	return &Runtime{cfg: cfg}, nil
}

// Index returns the index of the current snapshot, rebuilding it only when
// the revision changed (a source without revisions is rebuilt every call).
func (r *Runtime) Index(ctx context.Context) (*Index, error) {
	snap, err := r.cfg.Source.Snapshot(ctx)
	if err != nil {
		return nil, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.ix != nil && snap != nil && snap.Revision != "" && r.ix.Revision == snap.Revision {
		return r.ix, nil
	}
	snap = r.withStyles(ctx, snap)
	ix := buildIndex(snap, r.nativeRoutes(ctx, snap))
	for _, d := range ix.Diagnostics {
		r.cfg.Logger.WithField("revision", ix.Revision).Warn("extruntime ui: " + d)
	}
	r.prev, r.ix = r.ix, ix
	return ix, nil
}

// previous returns the index the current one replaced, if any.
func (r *Runtime) previous() *Index {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.prev
}

// nativeRoutes lists the native routes the index needs: the host's
// path-only routes, upgraded with the typed contract of every route the
// snapshot refers to (a screen binding's target, a page's route) that the
// catalog resolves to a native route resource. Typed routes therefore need
// no second registration in NativeRoutes.
func (r *Runtime) nativeRoutes(ctx context.Context, snap *Snapshot) map[string]NativeRouteDecl {
	out := make(map[string]NativeRouteDecl, len(r.cfg.NativeRoutes))
	for id, pattern := range r.cfg.NativeRoutes {
		out[id] = NativeRouteDecl{Pattern: pattern}
	}
	if r.cfg.Catalog == nil || snap == nil {
		return out
	}
	wanted := map[string]bool{}
	for _, b := range snap.Bindings {
		if b.SlotKind == SlotScreen {
			wanted[b.TargetID] = true
		}
	}
	for _, p := range snap.Packages {
		for _, res := range p.Manifest.Resources {
			if res.Kind == manifest.ResPage && res.Route != "" {
				wanted[res.Route] = true
			}
		}
	}
	for id := range wanted {
		resolved, ok, err := r.cfg.Catalog.Resolve(ctx, id)
		if err != nil {
			r.cfg.Logger.WithError(err).Warn("extruntime ui: resolve route " + id)
			continue
		}
		if !ok || !resolved.Native || resolved.Resource.Kind != manifest.ResRoute || resolved.Resource.Path == "" {
			continue
		}
		out[id] = NativeRouteDecl{Pattern: resolved.Resource.Path, Params: resolved.Resource.Params, Permission: resolved.Resource.Permission}
	}
	return out
}

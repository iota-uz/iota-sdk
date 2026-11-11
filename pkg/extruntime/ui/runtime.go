package ui

import (
	"context"
	"fmt"
	"sync"

	"github.com/sirupsen/logrus"

	"github.com/iota-uz/iota-sdk/pkg/clienthost"
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
	NativeRoutes NativeRoutes
	Logger       logrus.FieldLogger
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

	mu sync.Mutex
	ix *Index
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
	ix := BuildIndex(snap, r.cfg.NativeRoutes)
	for _, d := range ix.Diagnostics {
		r.cfg.Logger.WithField("revision", ix.Revision).Warn("extruntime ui: " + d)
	}
	r.ix = ix
	return ix, nil
}

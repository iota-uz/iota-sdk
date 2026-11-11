package runtime

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/iota-uz/iota-sdk/pkg/clienthost"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/artifact"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/catalog"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/data"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/host"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/ui"
	"github.com/iota-uz/iota-sdk/pkg/listext"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// UIConfig configures the UI slice on top of the management storage.
type UIConfig struct {
	// Host describes the browser runtime host bundle. Required.
	Host clienthost.Manifest
	// NativeRoutes names the native pages a screen binding may replace; they
	// are also registered as native route contracts so bindings can target
	// them.
	NativeRoutes ui.NativeRoutes
	// SourceTTL is how long the activated snapshot is cached. Default 2s.
	SourceTTL time.Duration
	Logger    logrus.FieldLogger
}

// UIManifests implements ui.ManifestSource over the active catalog: the
// activated versions with their manifests and artifact hashes. Column
// resources reach listext through the same manifests.
type UIManifests struct{ Catalog *catalog.Catalog }

var _ ui.ManifestSource = UIManifests{}

// ActivePackages implements ui.ManifestSource. The revision changes with the
// set of activated versions.
func (m UIManifests) ActivePackages(ctx context.Context) (string, []ui.ActivePackage, error) {
	active, err := m.Catalog.Active(ctx)
	if err != nil {
		return "", nil, err
	}
	ids := make([]string, 0, len(active))
	out := make([]ui.ActivePackage, 0, len(active))
	for _, av := range active {
		ids = append(ids, av.VersionID)
		out = append(out, ui.ActivePackage{Package: av.Namespace, Version: av.Version, ArtifactHash: av.ArtifactHash, Manifest: av.Manifest})
	}
	sort.Strings(ids)
	h := sha256.New()
	for _, id := range ids {
		_, _ = h.Write([]byte(id))
		_, _ = h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil)), out, nil
}

// UIArtifacts implements ui.ArtifactStore over the materialized artifacts.
type UIArtifacts struct{ Store *artifact.Store }

var _ ui.ArtifactStore = UIArtifacts{}

// Open implements ui.ArtifactStore. Unknown hashes and paths are
// ui.ErrArtifactNotFound, and so is everything outside the client subtree of
// the artifact: the browser store never hands out server code or manifests.
func (a UIArtifacts) Open(ctx context.Context, hash, filePath string) (ui.ArtifactFile, error) {
	if !strings.HasPrefix(filePath, ui.ClientDir) {
		return nil, ui.ErrArtifactNotFound
	}
	f, err := a.Store.OpenFile(ctx, hash, filePath)
	if err != nil {
		switch serrors.CodeOf(err) {
		case serrors.NotFound, serrors.Invalid:
			return nil, ui.ErrArtifactNotFound
		case serrors.Internal, serrors.AlreadyExists, serrors.Conflict, serrors.FailedPrecondition,
			serrors.PermissionDenied, serrors.Unauthenticated, serrors.RateLimited, serrors.Unavailable,
			serrors.Timeout, serrors.Canceled, serrors.Unimplemented:
			return nil, err
		}
		return nil, err
	}
	return f, nil
}

// UIConfig returns a ui.Config wired to this runtime, for applications that
// construct the UI component themselves.
func (rt *Runtime) UIConfig() (ui.Config, error) {
	if rt.Config.UI == nil {
		return ui.Config{}, errors.New("extruntime: UI is not configured")
	}
	ttl := rt.Config.UI.SourceTTL
	if ttl <= 0 {
		ttl = 2 * time.Second
	}
	return ui.Config{
		Source:    ui.NewPGSource(rt.db, UIManifests{Catalog: rt.Catalog}, ttl),
		Artifacts: UIArtifacts{Store: rt.Artifacts},
		Invoker:   rt.Host,
		Catalog:   rt.Catalog,
		Host:      rt.Config.UI.Host, NativeRoutes: rt.Config.UI.NativeRoutes, Logger: rt.Config.UI.Logger,
	}, nil
}

// holder hands the Runtime to consumers that are constructed before it.
type holder struct{ p atomic.Pointer[Runtime] }

var errNotReady = errors.New("extruntime: runtime is not built yet")

func (h *holder) get() (*Runtime, error) {
	if rt := h.p.Load(); rt != nil {
		return rt, nil
	}
	return nil, errNotReady
}

// lazy implements the UI slice's dependencies by delegating to the Runtime
// once the container has built it.
type lazy struct {
	h   *holder
	cfg atomic.Pointer[ui.Config]
}

func (l *lazy) resolve() (ui.Config, error) {
	if c := l.cfg.Load(); c != nil {
		return *c, nil
	}
	rt, err := l.h.get()
	if err != nil {
		return ui.Config{}, err
	}
	c, err := rt.UIConfig()
	if err != nil {
		return ui.Config{}, err
	}
	l.cfg.Store(&c)
	return c, nil
}

func (l *lazy) Snapshot(ctx context.Context) (*ui.Snapshot, error) {
	c, err := l.resolve()
	if err != nil {
		return nil, err
	}
	return c.Source.Snapshot(ctx)
}

func (l *lazy) Open(ctx context.Context, hash, p string) (ui.ArtifactFile, error) {
	c, err := l.resolve()
	if err != nil {
		return nil, err
	}
	return c.Artifacts.Open(ctx, hash, p)
}

func (l *lazy) Invoke(ctx context.Context, req host.Request) (host.Result, error) {
	rt, err := l.h.get()
	if err != nil {
		return host.Result{}, err
	}
	return rt.Host.Invoke(ctx, req)
}

func (l *lazy) Resolve(ctx context.Context, id string) (spi.ResolvedResource, bool, error) {
	rt, err := l.h.get()
	if err != nil {
		return spi.ResolvedResource{}, false, err
	}
	return rt.Catalog.Resolve(ctx, id)
}

func (l *lazy) Entities(ctx context.Context) ([]spi.ResolvedResource, error) {
	rt, err := l.h.get()
	if err != nil {
		return nil, err
	}
	return rt.Catalog.Entities(ctx)
}

func (l *lazy) Guards(ctx context.Context, target string, phase manifest.GuardPhase) ([]spi.ResolvedGuard, error) {
	rt, err := l.h.get()
	if err != nil {
		return nil, err
	}
	return rt.Catalog.Guards(ctx, target, phase)
}

func (l *lazy) NativeOverlayJoin(ctx context.Context, q data.Querier, sub spi.Subject, nativeID, baseAlias string, refs []data.OverlayFieldRef, args *data.Args) (*data.OverlayJoin, error) {
	rt, err := l.h.get()
	if err != nil {
		return nil, err
	}
	return rt.Data.NativeOverlayJoin(ctx, q, sub, nativeID, baseAlias, refs, args)
}

var (
	_ ui.Source             = (*lazy)(nil)
	_ ui.ArtifactStore      = (*lazy)(nil)
	_ ui.Invoker            = (*lazy)(nil)
	_ spi.Catalog           = (*lazy)(nil)
	_ listext.OverlayJoiner = (*lazy)(nil)
)

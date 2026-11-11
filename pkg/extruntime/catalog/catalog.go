package catalog

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// NotifyChannel is the PostgreSQL channel activation notifies on, so every
// replica drops its cached snapshot.
const NotifyChannel = "extruntime_catalog"

const opLoad = serrors.Op("extruntime.catalog.Catalog.load")

// ActiveVersion is one activated package version.
type ActiveVersion struct {
	VersionID    string
	Namespace    string
	Version      string
	ArtifactHash string
	Manifest     manifest.Manifest
}

type snapshot struct {
	versions  []ActiveVersion
	resources map[string]spi.ResolvedResource
	entities  []spi.ResolvedResource
	guards    []spi.ResolvedGuard
}

// Catalog implements spi.Catalog. It caches one snapshot of the native set
// and the active versions; Invalidate (called after activation, or by the
// LISTEN loop for activations on other replicas) drops it.
type Catalog struct {
	db      *pgxpool.Pool
	natives *NativeSet

	mu        sync.Mutex
	snap      *snapshot
	gen       uint64
	listeners []func()
}

var _ spi.Catalog = (*Catalog)(nil)

func New(db *pgxpool.Pool, natives *NativeSet) *Catalog {
	return &Catalog{db: db, natives: natives}
}

// Natives is the native contract set the catalog was built with.
func (c *Catalog) Natives() *NativeSet { return c.natives }

// OnChange registers fn to run (in its own goroutine) after every
// invalidation, so adapters can follow activations.
func (c *Catalog) OnChange(fn func()) {
	c.mu.Lock()
	c.listeners = append(c.listeners, fn)
	c.mu.Unlock()
}

// Invalidate drops the cached snapshot.
func (c *Catalog) Invalidate() {
	c.mu.Lock()
	c.snap = nil
	c.gen++
	listeners := append([]func(){}, c.listeners...)
	c.mu.Unlock()
	for _, fn := range listeners {
		go fn()
	}
}

func (c *Catalog) current(ctx context.Context) (*snapshot, error) {
	c.mu.Lock()
	if c.snap != nil {
		s := c.snap
		c.mu.Unlock()
		return s, nil
	}
	gen := c.gen
	c.mu.Unlock()

	s, err := c.load(ctx)
	if err != nil {
		return nil, err
	}
	c.mu.Lock()
	if c.gen == gen {
		c.snap = s
	}
	c.mu.Unlock()
	return s, nil
}

// Active lists the activated versions, ordered by namespace.
func (c *Catalog) Active(ctx context.Context) ([]ActiveVersion, error) {
	s, err := c.current(ctx)
	if err != nil {
		return nil, err
	}
	return append([]ActiveVersion(nil), s.versions...), nil
}

func (c *Catalog) load(ctx context.Context) (*snapshot, error) {
	rows, err := c.db.Query(ctx, `
		SELECT v.id::text, v.namespace, v.version, v.artifact_hash, v.manifest
		FROM extruntime.activations a JOIN extruntime.versions v ON v.id = a.version_id
		WHERE a.state = 'active' ORDER BY v.namespace`)
	if err != nil {
		return nil, serrors.FromDB(opLoad, err)
	}
	defer rows.Close()
	s := &snapshot{resources: map[string]spi.ResolvedResource{}}
	for rows.Next() {
		var av ActiveVersion
		var raw []byte
		if err := rows.Scan(&av.VersionID, &av.Namespace, &av.Version, &av.ArtifactHash, &raw); err != nil {
			return nil, serrors.FromDB(opLoad, err)
		}
		if err := json.Unmarshal(raw, &av.Manifest); err != nil {
			return nil, serrors.Wrap(opLoad, err)
		}
		s.versions = append(s.versions, av)
	}
	if err := rows.Err(); err != nil {
		return nil, serrors.FromDB(opLoad, err)
	}
	for _, r := range c.natives.Resources() {
		rr := spi.ResolvedResource{Resource: r, Native: true}
		s.resources[r.ID] = rr
		if r.Kind == manifest.ResEntity || r.Kind == manifest.ResOverlay {
			s.entities = append(s.entities, rr)
		}
	}
	for _, av := range s.versions {
		for _, r := range av.Manifest.Resources {
			if r.ContractHash == "" {
				r.ContractHash = manifest.ContractHashOf(r)
			}
			if _, taken := s.resources[r.ID]; taken {
				continue
			}
			rr := spi.ResolvedResource{Resource: r, Package: av.Namespace, Version: av.Version}
			s.resources[r.ID] = rr
			if r.Kind == manifest.ResEntity || r.Kind == manifest.ResOverlay {
				s.entities = append(s.entities, rr)
			}
		}
		for _, g := range av.Manifest.Guards {
			s.guards = append(s.guards, spi.ResolvedGuard{Guard: g, Package: av.Namespace, Version: av.Version})
		}
	}
	sort.SliceStable(s.guards, func(i, j int) bool { return s.guards[i].Guard.ID < s.guards[j].Guard.ID })
	return s, nil
}

// Resolve implements spi.Catalog.
func (c *Catalog) Resolve(ctx context.Context, id string) (spi.ResolvedResource, bool, error) {
	s, err := c.current(ctx)
	if err != nil {
		return spi.ResolvedResource{}, false, err
	}
	r, ok := s.resources[id]
	return r, ok, nil
}

// Entities implements spi.Catalog.
func (c *Catalog) Entities(ctx context.Context) ([]spi.ResolvedResource, error) {
	s, err := c.current(ctx)
	if err != nil {
		return nil, err
	}
	return append([]spi.ResolvedResource(nil), s.entities...), nil
}

// Guards implements spi.Catalog.
func (c *Catalog) Guards(ctx context.Context, target string, phase manifest.GuardPhase) ([]spi.ResolvedGuard, error) {
	s, err := c.current(ctx)
	if err != nil {
		return nil, err
	}
	var out []spi.ResolvedGuard
	for _, g := range s.guards {
		if g.Guard.Target == target && g.Guard.Phase == phase {
			out = append(out, g)
		}
	}
	return out, nil
}

// Listen invalidates the cache whenever another replica activates or
// deactivates a package. It reconnects until ctx is done.
func (c *Catalog) Listen(ctx context.Context) error {
	for ctx.Err() == nil {
		if err := c.listenOnce(ctx); err != nil && ctx.Err() == nil {
			select {
			case <-ctx.Done():
			case <-time.After(time.Second):
			}
		}
	}
	return ctx.Err()
}

func (c *Catalog) listenOnce(ctx context.Context) error {
	conn, err := c.db.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()
	if _, err := conn.Exec(ctx, "LISTEN "+NotifyChannel); err != nil {
		return err
	}
	// A notification may have been missed while not listening.
	c.Invalidate()
	for {
		_, err := conn.Conn().WaitForNotification(ctx)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				continue
			}
			return err
		}
		c.Invalidate()
	}
}

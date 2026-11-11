package ui

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// Querier is the read access PGSource needs; *pgxpool.Pool implements it.
type Querier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

// PGSource is the Source of production: activated manifests come from the
// management slice (ManifestSource), screen and shell bindings from
// extruntime.bindings. The result is cached for a short TTL so rendering a
// page costs at most one small query per interval, never a Node call.
type PGSource struct {
	db        Querier
	manifests ManifestSource
	ttl       time.Duration
	now       func() time.Time

	// generation, when set, reports a counter that changes whenever an
	// activation or binding changed (catalog invalidation); a cached snapshot
	// of another generation is never served, so a change in this process or a
	// notified one in another replica applies on the next request.
	generation func() uint64

	mu     sync.Mutex
	cached *Snapshot
	at     time.Time
	gen    uint64
}

var _ Source = (*PGSource)(nil)

// NewPGSource wires the source. ttl <= 0 disables caching (every call reads).
func NewPGSource(db Querier, manifests ManifestSource, ttl time.Duration) *PGSource {
	return &PGSource{db: db, manifests: manifests, ttl: ttl, now: time.Now}
}

// WithGeneration makes the cache follow a change counter (catalog.Generation).
func (s *PGSource) WithGeneration(generation func() uint64) *PGSource {
	s.generation = generation
	return s
}

// Snapshot reads the activated packages and the open screen and shell
// bindings. A closed binding (active_to set) is not part of it, so reverting
// a replacement takes effect on the next read.
func (s *PGSource) Snapshot(ctx context.Context) (*Snapshot, error) {
	const op = serrors.Op("extruntime.ui.PGSource.Snapshot")
	s.mu.Lock()
	defer s.mu.Unlock()
	var gen uint64
	if s.generation != nil {
		gen = s.generation()
	}
	if s.cached != nil && s.ttl > 0 && s.now().Sub(s.at) < s.ttl && gen == s.gen {
		return s.cached, nil
	}
	revision, packages, err := s.manifests.ActivePackages(ctx)
	if err != nil {
		return nil, serrors.Wrap(op, err)
	}
	rows, err := s.db.Query(ctx, `
		SELECT id::text, slot_kind, target_id, package, version, export, active_from
		FROM extruntime.bindings
		WHERE slot_kind IN ('screen', 'shell') AND active_to IS NULL
		ORDER BY slot_kind, target_id, id`)
	if err != nil {
		return nil, serrors.FromDB(op, err)
	}
	defer rows.Close()
	h := sha256.New()
	h.Write([]byte(revision))
	snap := &Snapshot{Packages: packages}
	for rows.Next() {
		var b Binding
		if err := rows.Scan(&b.ID, &b.SlotKind, &b.TargetID, &b.Package, &b.Version, &b.Export, &b.ActiveFrom); err != nil {
			return nil, serrors.FromDB(op, err)
		}
		snap.Bindings = append(snap.Bindings, b)
		h.Write([]byte{0})
		h.Write([]byte(b.ID))
	}
	if err := rows.Err(); err != nil {
		return nil, serrors.FromDB(op, err)
	}
	snap.Revision = hex.EncodeToString(h.Sum(nil))
	s.cached, s.at, s.gen = snap, s.now(), gen
	return snap, nil
}

// Invalidate drops the cached snapshot (for example after management
// activated or reverted something in this process).
func (s *PGSource) Invalidate() {
	s.mu.Lock()
	s.cached = nil
	s.mu.Unlock()
}

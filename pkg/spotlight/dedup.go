package spotlight

import (
	"time"

	lru "github.com/hashicorp/golang-lru/v2"

	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// EventDedupConfig tunes the in-process at-least-once dedupe window. NATS
// JetStream delivers events at-least-once; the same (provider, pk,
// event_id) tuple may arrive twice, producing two AddDocuments. Meili
// deduplicates by primary key, but our counters and lag metrics see the
// duplicate as a fresh event, distorting throughput.
type EventDedupConfig struct {
	// Capacity is the maximum number of distinct keys retained. Larger
	// values cost more memory; smaller values rotate keys out and
	// re-admit re-deliveries.
	Capacity int
	// TTL is how long a key is held in the cache. nats redelivers within
	// JetStream's max-redeliver window, typically minutes.
	TTL time.Duration
}

// DefaultEventDedupConfig returns a deduper configured for 10 000 entries
// over a 30-minute window. Issue #2810 §3.5.
func DefaultEventDedupConfig() EventDedupConfig {
	return EventDedupConfig{
		Capacity: 10_000,
		TTL:      30 * time.Minute,
	}
}

// EventDeduper is a TTL-bounded LRU keyed by event identity. Concurrent
// callers should serialize their own access if strict ordering matters;
// the LRU implementation itself is thread-safe.
type EventDeduper struct {
	cfg   EventDedupConfig
	cache *lru.Cache[string, time.Time]
}

// NewEventDeduper returns a deduper with the supplied config (zero-value
// fields are normalized to DefaultEventDedupConfig). Returns an error
// rather than panicking on cache init failure so callers can degrade
// gracefully — typically by falling back to a nil *EventDeduper which
// Seen() handles as "never seen" (every event passes through).
func NewEventDeduper(cfg EventDedupConfig) (*EventDeduper, error) {
	const op serrors.Op = "spotlight.NewEventDeduper"
	if cfg.Capacity <= 0 {
		cfg.Capacity = DefaultEventDedupConfig().Capacity
	}
	if cfg.TTL <= 0 {
		cfg.TTL = DefaultEventDedupConfig().TTL
	}
	cache, err := lru.New[string, time.Time](cfg.Capacity)
	if err != nil {
		return nil, serrors.E(op, err)
	}
	return &EventDeduper{cfg: cfg, cache: cache}, nil
}

// Seen returns true when the supplied key was already recorded within
// the TTL window. The key is refreshed on every call so a duplicate that
// arrives mid-window slides the TTL forward.
func (d *EventDeduper) Seen(provider, primaryKey, eventID string) bool {
	if d == nil {
		return false
	}
	key := provider + "\x00" + primaryKey + "\x00" + eventID
	if recordedAt, ok := d.cache.Get(key); ok {
		if time.Since(recordedAt) <= d.cfg.TTL {
			return true
		}
	}
	d.cache.Add(key, time.Now())
	return false
}

// DedupStats reports the dedup cache fill level. Exposed for the
// /system/spotlight UI and the dedup metrics counter.
type DedupStats struct {
	Entries  int
	Capacity int
}

// Stats reports the current cache fill level.
func (d *EventDeduper) Stats() DedupStats {
	if d == nil {
		return DedupStats{}
	}
	return DedupStats{Entries: d.cache.Len(), Capacity: d.cfg.Capacity}
}

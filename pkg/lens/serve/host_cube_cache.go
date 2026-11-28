package serve

import (
	"container/list"
	"context"
	"sync"
	"time"
)

type cubeCacheEntry struct {
	key       string
	value     ResolvedCube
	expiresAt time.Time
	element   *list.Element
}

type cubeFlight struct {
	done    chan struct{}
	cancel  context.CancelFunc
	waiters int
	value   ResolvedCube
	err     error
}

// hostCubeCache memoizes resolved cubes (and lazily loaded CubeSpecs) with
// bounded LRU capacity, TTL expiry, and singleflight coalescing keyed by
// canonical ReportKey (DataScope, Namespace, Version, Locale, Timezone,
// Variant, Params).
type hostCubeCache struct {
	mu           sync.Mutex
	ttl          time.Duration
	maxEntries   int
	buildTimeout time.Duration
	limiter      *ReportBuildLimiter
	byKey        map[string]*cubeCacheEntry
	flights      map[string]*cubeFlight
	lru          *list.List
	closed       bool
}

func newHostCubeCache(ttl time.Duration, maxEntries int, buildTimeout time.Duration, limiter *ReportBuildLimiter) *hostCubeCache {
	if ttl <= 0 {
		ttl = defaultHostTTL
	}
	if maxEntries <= 0 {
		maxEntries = defaultHostMaxEntries
	}
	if buildTimeout <= 0 {
		buildTimeout = defaultWorkTimeout
	}
	return &hostCubeCache{
		ttl:          ttl,
		maxEntries:   maxEntries,
		buildTimeout: buildTimeout,
		limiter:      limiter,
		byKey:        make(map[string]*cubeCacheEntry),
		flights:      make(map[string]*cubeFlight),
		lru:          list.New(),
	}
}

func (c *hostCubeCache) getOrLoad(
	ctx context.Context,
	key ReportKey,
	admit bool,
	load func(context.Context) (ResolvedCube, error),
) (ResolvedCube, error) {
	canonical, err := key.canonical()
	if err != nil {
		return ResolvedCube{}, err
	}
	if RecomputeRequested(ctx) {
		return c.loadDirect(ctx, canonical, admit, load)
	}

	// Fast path: cache hit or join an already-admitted flight without touching BuildLimiter.
	if val, hit, flight, closedErr := c.lookupOrJoin(canonical); closedErr != nil {
		return ResolvedCube{}, closedErr
	} else if hit {
		return val, nil
	} else if flight != nil {
		return c.waitFlight(ctx, flight)
	}

	// Acquire admission BEFORE publishing a flight so no flight in c.flights can
	// ever block waiting on BuildLimiter while another caller holding BuildLimiter
	// (e.g. ReportCache.Materialize) waits on that flight.
	var release func()
	if admit && c.limiter != nil {
		var acquireErr error
		release, acquireErr = c.limiter.Acquire(ctx)
		if acquireErr != nil {
			return ResolvedCube{}, acquireErr
		}
	}

	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		if release != nil {
			release()
		}
		return ResolvedCube{}, context.Canceled
	}
	if val, hit := c.lookupLocked(canonical); hit {
		c.mu.Unlock()
		if release != nil {
			release()
		}
		return val, nil
	}
	if flight := c.flights[canonical]; flight != nil {
		flight.waiters++
		c.mu.Unlock()
		if release != nil {
			release()
		}
		return c.waitFlight(ctx, flight)
	}

	workCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), c.buildTimeout)
	flight := &cubeFlight{
		done:    make(chan struct{}),
		cancel:  cancel,
		waiters: 1,
	}
	c.flights[canonical] = flight
	c.mu.Unlock()

	go c.runFlight(workCtx, canonical, flight, release, load)
	return c.waitFlight(ctx, flight)
}

func (c *hostCubeCache) loadDirect(
	ctx context.Context,
	canonical string,
	admit bool,
	load func(context.Context) (ResolvedCube, error),
) (ResolvedCube, error) {
	c.mu.Lock()
	closed := c.closed
	c.mu.Unlock()
	if closed {
		return ResolvedCube{}, context.Canceled
	}
	if admit && c.limiter != nil {
		release, err := c.limiter.Acquire(ctx)
		if err != nil {
			return ResolvedCube{}, err
		}
		defer release()
	}
	val, err := load(ctx)
	if err != nil {
		return ResolvedCube{}, err
	}
	if ctx.Err() != nil {
		return ResolvedCube{}, ctx.Err()
	}
	c.store(canonical, val)
	return val, nil
}

func (c *hostCubeCache) lookupOrJoin(canonical string) (ResolvedCube, bool, *cubeFlight, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return ResolvedCube{}, false, nil, context.Canceled
	}
	if val, hit := c.lookupLocked(canonical); hit {
		return val, true, nil, nil
	}
	if flight := c.flights[canonical]; flight != nil {
		flight.waiters++
		return ResolvedCube{}, false, flight, nil
	}
	return ResolvedCube{}, false, nil, nil
}

func (c *hostCubeCache) lookupLocked(canonical string) (ResolvedCube, bool) {
	entry := c.byKey[canonical]
	if entry == nil {
		return ResolvedCube{}, false
	}
	if !time.Now().Before(entry.expiresAt) {
		c.removeLocked(entry)
		return ResolvedCube{}, false
	}
	c.lru.MoveToFront(entry.element)
	return entry.value, true
}

func (c *hostCubeCache) runFlight(
	workCtx context.Context,
	canonical string,
	flight *cubeFlight,
	release func(),
	load func(context.Context) (ResolvedCube, error),
) {
	defer flight.cancel()
	if release != nil {
		defer release()
	}
	val, err := load(workCtx)
	if err == nil && workCtx.Err() != nil {
		err = workCtx.Err()
	}
	c.mu.Lock()
	if err == nil && !c.closed {
		c.storeLocked(canonical, val)
	}
	flight.value = val
	flight.err = err
	if c.flights[canonical] == flight {
		delete(c.flights, canonical)
	}
	close(flight.done)
	c.mu.Unlock()
}

func (c *hostCubeCache) waitFlight(ctx context.Context, flight *cubeFlight) (ResolvedCube, error) {
	defer func() {
		c.mu.Lock()
		flight.waiters--
		if flight.waiters == 0 {
			flight.cancel()
		}
		c.mu.Unlock()
	}()
	select {
	case <-ctx.Done():
		return ResolvedCube{}, ctx.Err()
	case <-flight.done:
		if flight.err != nil {
			return ResolvedCube{}, flight.err
		}
		return flight.value, nil
	}
}

func (c *hostCubeCache) store(canonical string, val ResolvedCube) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return
	}
	c.storeLocked(canonical, val)
}

func (c *hostCubeCache) storeLocked(canonical string, val ResolvedCube) {
	now := time.Now()
	if existing := c.byKey[canonical]; existing != nil {
		existing.value = val
		existing.expiresAt = now.Add(c.ttl)
		c.lru.MoveToFront(existing.element)
		return
	}
	entry := &cubeCacheEntry{
		key:       canonical,
		value:     val,
		expiresAt: now.Add(c.ttl),
	}
	entry.element = c.lru.PushFront(entry)
	c.byKey[canonical] = entry
	for c.lru.Len() > c.maxEntries {
		back := c.lru.Back()
		if back == nil {
			break
		}
		c.removeLocked(back.Value.(*cubeCacheEntry))
	}
}

func (c *hostCubeCache) removeLocked(entry *cubeCacheEntry) {
	if entry == nil {
		return
	}
	if entry.element != nil {
		c.lru.Remove(entry.element)
		entry.element = nil
	}
	if c.byKey[entry.key] == entry {
		delete(c.byKey, entry.key)
	}
}

func (c *hostCubeCache) Close() {
	c.mu.Lock()
	c.closed = true
	cancels := make([]context.CancelFunc, 0, len(c.flights))
	for _, flight := range c.flights {
		cancels = append(cancels, flight.cancel)
	}
	clear(c.byKey)
	c.lru.Init()
	c.mu.Unlock()
	for _, cancel := range cancels {
		cancel()
	}
}

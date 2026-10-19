package serve

import (
	"container/list"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/lens"
	"github.com/iota-uz/iota-sdk/pkg/lens/document"
	"github.com/iota-uz/iota-sdk/pkg/lens/resource"
	lensruntime "github.com/iota-uz/iota-sdk/pkg/lens/runtime"
)

// ReportKey makes tenant, producer-version and presentation identity explicit.
// Params must contain normalized producer inputs, including permission variants.
type ReportKey struct {
	Namespace        string `json:"namespace"`
	DataScope        string `json:"dataScope"`
	Version          string `json:"version"`
	Locale           string `json:"locale"`
	Timezone         string `json:"timezone"`
	Variant          string `json:"variant"`
	Params           string `json:"params"`
	ParentSnapshotID string `json:"parentSnapshotID"`
}

func (k ReportKey) canonical() (string, error) {
	keyBytes := len(k.Namespace) + len(k.DataScope) + len(k.Version) + len(k.Locale) + len(k.Timezone) + len(k.Variant) + len(k.Params) + len(k.ParentSnapshotID)
	if keyBytes > 64<<10 {
		return "", resource.NewLimitError("report key bytes", int64(keyBytes), 64<<10)
	}
	if strings.TrimSpace(k.Namespace) == "" || strings.TrimSpace(k.DataScope) == "" || strings.TrimSpace(k.Version) == "" {
		return "", fmt.Errorf("report key requires namespace, data scope and version")
	}
	encoded, err := json.Marshal(k)
	return string(encoded), err
}

// ReportDefinition supplies one immutable report registration and host-owned state.
type ReportDefinition[T any] struct {
	Config  Config
	Request lensruntime.Request
	Options document.BuildOptions
	Prepare func(*document.DashboardDocument) error
	State   T
}

// CachedReport is a request-owned view. Document may be decorated freely.
// State remains host-owned; hosts synchronize mutable state and reserve its
// retained payload explicitly against the shared resource budget.
type CachedReport[T any] struct {
	Document *document.DashboardDocument
	Handlers *Handlers
	State    T
	release  func()
	spec     func() (lens.DashboardSpec, error)
}

// Spec returns an owned execution/export specification. Renderer-only templ
// icon callbacks are omitted; wire presentation and execution metadata remain.
func (r *CachedReport[T]) Spec() (lens.DashboardSpec, error) {
	if r == nil || r.spec == nil {
		return lens.DashboardSpec{}, fmt.Errorf("report registration is unavailable")
	}
	return r.spec()
}

// Release ends the registration lease. The owned Document can outlive it.
func (r *CachedReport[T]) Release() {
	if r != nil && r.release != nil {
		r.release()
	}
}

// ReportRegistration borrows execution and host state without decoding a document.
// Release it after the complete operation. A forgotten lease retains its budget.
type ReportRegistration[T any] struct {
	Handlers *Handlers
	State    T
	release  func()
	spec     func() (lens.DashboardSpec, error)
}

// Spec returns an owned execution/export specification with renderer-only templ
// icon callbacks omitted.
func (r *ReportRegistration[T]) Spec() (lens.DashboardSpec, error) {
	if r == nil || r.spec == nil {
		return lens.DashboardSpec{}, fmt.Errorf("report registration is unavailable")
	}
	return r.spec()
}

// Release ends this registration lease; repeated calls are harmless.
func (r *ReportRegistration[T]) Release() {
	if r != nil && r.release != nil {
		r.release()
	}
}

// ReportCacheOptions controls retained payload, work admission and host cleanup.
// Zero entry/derived/document/concurrent/waiter limits use128 / 16 / 16MiB / 4 / 128.
// MaxPendingBuilds zero means no queue; BuildTimeout defaults to two minutes.
// Budget is required. Share BuildLimiter across caches for process-wide admission.
type ReportCacheOptions[T any] struct {
	Budget              *resource.Budget
	BuildLimiter        *ReportBuildLimiter
	MaxEntries          int
	MaxDerivedPerReport int
	MaxDocumentBytes    int64
	MaxConcurrentBuilds int
	MaxPendingBuilds    int
	MaxWaitersPerBuild  int
	BuildTimeout        time.Duration
	OnEvict             func(T)
}

type cachedDerived struct {
	document    *document.FrozenDocument
	reservation *resource.Reservation
	snapshotID  string
	forget      func()
	touch       func(context.Context) error
	backed      bool
}

type reportEntry[T any] struct {
	key             string
	snapshotID      string
	document        *document.FrozenDocument
	reservation     *resource.Reservation
	handlers        *Handlers
	specReservation *resource.Reservation
	state           T
	parentID        string
	acquireParent   func(context.Context) (func(), error)
	derivedOwners   int
	dataScope       string
	element         *list.Element
	derived         map[string]*cachedDerived
	active          bool
	leases          int
	disposed        bool
}

type reportFlight[T any] struct {
	done    chan struct{}
	cancel  context.CancelFunc
	waiters int
	entry   *reportEntry[T]
	err     error
}

// ReportCache owns document reuse, execution registrations and derived payloads.
// It bounds retained wire payload and admitted builds, not arbitrary callback
// allocations or the process heap. One shared Budget can cover multiple caches,
// snapshot stores and explicitly reserved host state.
type ReportCache[T any] struct {
	mu             sync.Mutex
	opts           ReportCacheOptions[T]
	byKey          map[string]*reportEntry[T]
	bySnapshot     map[string]*reportEntry[T]
	flights        map[string]*reportFlight[T]
	derivedFlights map[string]*derivedReportFlight
	lru            *list.List
	sessions       *SessionRegistry
	admitted       int
	closed         bool
	rejections     atomic.Uint64
}

// NewReportCache constructs a bounded cache; it starts no permanent goroutines.
func NewReportCache[T any](opts ReportCacheOptions[T]) (*ReportCache[T], error) {
	if opts.Budget == nil {
		return nil, fmt.Errorf("report cache requires a shared resource budget")
	}
	if opts.MaxEntries <= 0 {
		opts.MaxEntries = 128
	}
	if opts.MaxDerivedPerReport <= 0 {
		opts.MaxDerivedPerReport = 16
	}
	if opts.MaxDocumentBytes <= 0 {
		opts.MaxDocumentBytes = document.DefaultMaxDocumentBytes
	}
	if opts.MaxConcurrentBuilds <= 0 {
		opts.MaxConcurrentBuilds = 4
	}
	if opts.MaxWaitersPerBuild <= 0 {
		opts.MaxWaitersPerBuild = 128
	}
	if opts.MaxPendingBuilds < 0 {
		return nil, fmt.Errorf("pending build limit cannot be negative")
	}
	if opts.BuildTimeout <= 0 {
		opts.BuildTimeout = 2 * time.Minute
	}
	if opts.BuildLimiter == nil {
		opts.BuildLimiter = NewReportBuildLimiter(opts.MaxConcurrentBuilds, opts.MaxPendingBuilds)
	}
	return &ReportCache[T]{opts: opts, byKey: map[string]*reportEntry[T]{}, bySnapshot: map[string]*reportEntry[T]{}, flights: map[string]*reportFlight[T]{}, derivedFlights: map[string]*derivedReportFlight{}, lru: list.New(), sessions: NewSessionRegistry()}, nil
}

func (c *ReportCache[T]) view(ctx context.Context, entry *reportEntry[T]) (*CachedReport[T], error) {
	registration, err := c.registration(ctx, entry, false)
	if err != nil {
		return nil, err
	}
	doc, err := entry.document.Document()
	if err != nil {
		registration.Release()
		return nil, err
	}
	return &CachedReport[T]{Document: doc, Handlers: entry.handlers, State: entry.state, release: registration.Release, spec: registration.spec}, nil
}

func (c *ReportCache[T]) registration(ctx context.Context, entry *reportEntry[T], allowEvicted bool) (*ReportRegistration[T], error) {
	return c.registrationWithParent(ctx, entry, allowEvicted, true)
}

func (c *ReportCache[T]) registrationWithParent(ctx context.Context, entry *reportEntry[T], allowEvicted, withParent bool) (*ReportRegistration[T], error) {
	parentRelease := func() {}
	if withParent && entry != nil && entry.acquireParent != nil {
		var err error
		parentRelease, err = entry.acquireParent(ctx)
		if err != nil {
			return nil, err
		}
	}
	c.mu.Lock()
	if entry == nil || entry.disposed || (!entry.active && entry.derivedOwners == 0 && !allowEvicted) {
		c.mu.Unlock()
		parentRelease()
		return nil, document.ErrSnapshotGone
	}
	entry.leases++
	c.mu.Unlock()
	var once sync.Once
	release := func() {
		once.Do(func() {
			c.mu.Lock()
			entry.leases--
			dispose := !entry.active && entry.leases == 0 && !entry.disposed
			if dispose {
				entry.disposed = true
			}
			c.mu.Unlock()
			if dispose {
				c.releaseRetained(entry)
			}
			parentRelease()
		})
	}
	return &ReportRegistration[T]{Handlers: entry.handlers, State: entry.state, release: release, spec: func() (lens.DashboardSpec, error) {
		spec, _, err := ownReportSpec(entry.handlers.spec, c.opts.MaxDocumentBytes)
		return spec, err
	}}, nil
}

func (c *ReportCache[T]) LookupSnapshotRegistration(ctx context.Context, id string) (*ReportRegistration[T], error) {
	c.mu.Lock()
	entry := c.bySnapshot[strings.TrimSpace(id)]
	c.mu.Unlock()
	ok, err := c.live(ctx, entry)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, document.ErrSnapshotGone
	}
	return c.registration(ctx, entry, false)
}

func (c *ReportCache[T]) live(ctx context.Context, entry *reportEntry[T]) (bool, error) {
	if entry == nil {
		return false, nil
	}
	var err error
	if toucher, ok := entry.handlers.snapshots.(interface {
		Touch(context.Context, string) error
	}); ok {
		err = toucher.Touch(ctx, entry.snapshotID)
	} else {
		_, err = entry.handlers.snapshots.Get(ctx, entry.snapshotID)
	}
	if err != nil {
		if !errors.Is(err, document.ErrSnapshotGone) {
			return false, err
		}
		c.evict(entry)
		return false, nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if !entry.active && entry.derivedOwners == 0 {
		return false, nil
	}
	if entry.active {
		c.lru.MoveToFront(entry.element)
	}
	return true, nil
}

// Lookup returns a leased, independently owned view of a live keyed report.
func (c *ReportCache[T]) Lookup(ctx context.Context, key ReportKey) (*CachedReport[T], bool, error) {
	key = reportKeyForContext(ctx, key)
	if owner, _ := ctx.Value(derivedBuildOwnershipKey{}).(*derivedBuildOwnership); owner != nil && owner.dataScope != key.DataScope {
		return nil, false, fmt.Errorf("derived report cannot replace its parent data scope")
	}
	canonical, err := key.canonical()
	if err != nil {
		return nil, false, err
	}
	c.mu.Lock()
	entry := c.byKey[canonical]
	c.mu.Unlock()
	ok, err := c.live(ctx, entry)
	if !ok || err != nil {
		return nil, false, err
	}
	view, err := c.view(ctx, entry)
	if err == nil {
		if owner, _ := ctx.Value(derivedBuildOwnershipKey{}).(*derivedBuildOwnership); owner != nil {
			if err = c.retainForOwner(ctx, entry, owner); err != nil {
				view.Release()
				return nil, false, err
			}
		}
	}
	return view, err == nil, err
}

// LookupSnapshot returns a leased document view by frozen snapshot identity.
func (c *ReportCache[T]) LookupSnapshot(ctx context.Context, id string) (*CachedReport[T], error) {
	c.mu.Lock()
	entry := c.bySnapshot[strings.TrimSpace(id)]
	c.mu.Unlock()
	ok, err := c.live(ctx, entry)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, document.ErrSnapshotGone
	}
	return c.view(ctx, entry)
}

// Materialize coalesces one identity and gives every caller an owned document.
// The caller must Release the returned registration lease.
func (c *ReportCache[T]) Materialize(ctx context.Context, key ReportKey, build func(context.Context) (ReportDefinition[T], error)) (*CachedReport[T], error) {
	view, err := c.materialize(ctx, key, build)
	c.countRejection(err)
	return view, err
}

func (c *ReportCache[T]) materialize(ctx context.Context, key ReportKey, build func(context.Context) (ReportDefinition[T], error)) (*CachedReport[T], error) {
	key = reportKeyForContext(ctx, key)
	if result, ok, err := c.Lookup(ctx, key); ok || err != nil {
		return result, err
	}
	if build == nil {
		return nil, fmt.Errorf("report builder is required")
	}
	canonical, err := key.canonical()
	if err != nil {
		return nil, err
	}
	c.mu.Lock()
	flight := c.flights[canonical]
	if flight != nil && flight.waiters >= c.opts.MaxWaitersPerBuild {
		c.mu.Unlock()
		return nil, &resource.LimitError{Kind: resource.LimitAdmission, Resource: "report waiters", Requested: int64(flight.waiters + 1), Limit: int64(c.opts.MaxWaitersPerBuild)}
	}
	if flight == nil {
		if c.closed {
			c.mu.Unlock()
			return nil, context.Canceled
		}
		if ctx.Value(reportBuildContextKey{}) != c.opts.BuildLimiter && len(c.flights)+len(c.derivedFlights) >= c.opts.MaxConcurrentBuilds+c.opts.MaxPendingBuilds {
			c.mu.Unlock()
			return nil, resource.NewLimitError("report_build_queue", int64(len(c.flights)+1), int64(c.opts.MaxConcurrentBuilds+c.opts.MaxPendingBuilds))
		}
		workCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), c.opts.BuildTimeout)
		flight = &reportFlight[T]{done: make(chan struct{}), cancel: cancel}
		c.flights[canonical] = flight
		go c.run(workCtx, canonical, key, flight, build)
	}
	flight.waiters++
	c.mu.Unlock()
	defer func() {
		c.mu.Lock()
		flight.waiters--
		if flight.waiters == 0 {
			flight.cancel()
			if flight.entry != nil {
				flight.entry.leases--
				dispose := !flight.entry.active && flight.entry.leases == 0 && !flight.entry.disposed
				if dispose {
					flight.entry.disposed = true
				}
				c.mu.Unlock()
				if dispose {
					c.releaseRetained(flight.entry)
				}
				return
			}
		}
		c.mu.Unlock()
	}()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-flight.done:
		if flight.err != nil {
			return nil, flight.err
		}
		registration, err := c.registration(ctx, flight.entry, true)
		if err != nil {
			return nil, err
		}
		doc, err := flight.entry.document.Document()
		if err != nil {
			registration.Release()
			return nil, err
		}
		if owner, _ := ctx.Value(derivedBuildOwnershipKey{}).(*derivedBuildOwnership); owner != nil {
			if err := c.retainForOwner(ctx, flight.entry, owner); err != nil {
				registration.Release()
				return nil, err
			}
		}
		return &CachedReport[T]{Document: doc, Handlers: registration.Handlers, State: registration.State, release: registration.Release, spec: registration.spec}, nil
	}
}

func (c *ReportCache[T]) run(ctx context.Context, key string, identity ReportKey, flight *reportFlight[T], build func(context.Context) (ReportDefinition[T], error)) {
	defer flight.cancel()
	workCtx, release, err := c.admit(ctx)
	if err != nil {
		c.finish(key, flight, nil, err)
		return
	}
	defer release()
	ctx = workCtx
	// A request can miss Lookup, then enter a new flight after a previous flight
	// has already published and disappeared. Recheck before invoking its producer.
	c.mu.Lock()
	existing := c.byKey[key]
	c.mu.Unlock()
	if ok, lookupErr := c.live(ctx, existing); lookupErr != nil {
		c.finish(key, flight, nil, lookupErr)
		return
	} else if ok {
		c.mu.Lock()
		pinned := existing.active && !existing.disposed
		if pinned {
			existing.leases++
		}
		c.mu.Unlock()
		if pinned {
			c.finish(key, flight, existing, nil)
			return
		}
	}
	definition, err := build(ctx)

	if err != nil {
		c.finish(key, flight, nil, err)
		return
	}
	stateTransferred := false
	defer func() {
		if !stateTransferred && c.opts.OnEvict != nil {
			c.opts.OnEvict(definition.State)
		}
	}()
	if definition.Request.DataScope != identity.DataScope || (definition.Request.Namespace != "" && definition.Request.Namespace != identity.Namespace) || definition.Request.Locale != identity.Locale || definition.Request.Timezone != identity.Timezone {
		c.finish(key, flight, nil, fmt.Errorf("report request data scope differs from cache key"))
		return
	}
	definition.Config.Sessions = c.sessions
	owner, _ := ctx.Value(derivedBuildOwnershipKey{}).(*derivedBuildOwnership)
	if owner != nil {
		definition.Config.Sessions = owner.sessions
	}
	if definition.Options.SnapshotID != "" {
		c.finish(key, flight, nil, fmt.Errorf("report cache owns snapshot identities"))
		return
	}
	ownedSpec, specBytes, err := ownReportSpec(definition.Config.Spec, c.opts.MaxDocumentBytes)
	if err != nil {
		c.finish(key, flight, nil, err)
		return
	}
	specReservation, err := c.reserve(specBytes + int64(len(key)))
	if err != nil {
		c.finish(key, flight, nil, err)
		return
	}
	defer func() {
		if !stateTransferred {
			specReservation.Release()
		}
	}()
	definition.Config.Spec = ownedSpec
	handlers, err := New(definition.Config)
	if err != nil {
		c.finish(key, flight, nil, err)
		return
	}
	doc, err := handlers.BuildDocument(ctx, definition.Request, definition.Options)
	if err != nil {
		c.finish(key, flight, nil, err)
		return
	}
	snapshotID := doc.SnapshotID
	cleanup := func() { handlers.ForgetSnapshot(context.Background(), snapshotID) }
	doc, err = ownPreparedDocument(doc, c.opts.MaxDocumentBytes)
	if err == nil && definition.Prepare != nil {
		err = definition.Prepare(doc)
	}
	if err == nil && doc.SnapshotID != snapshotID {
		err = fmt.Errorf("report preparation cannot replace snapshot identity")
	}
	if err == nil {
		err = ctx.Err()
	}
	var frozen *document.FrozenDocument
	if err == nil {
		frozen, err = document.Freeze(doc, c.opts.MaxDocumentBytes)
	}
	var reservation *resource.Reservation
	if err == nil {
		reservation, err = c.reserve(frozen.Size())
	}
	if err != nil {
		cleanup()
		c.finish(key, flight, nil, err)
		return
	}
	entry := &reportEntry[T]{key: key, snapshotID: doc.SnapshotID, document: frozen, reservation: reservation, handlers: handlers, state: definition.State, derived: map[string]*cachedDerived{}, active: true, leases: 1}
	entry.parentID = identity.ParentSnapshotID
	entry.dataScope = identity.DataScope
	if owner != nil {
		entry.acquireParent = owner.acquireParent
	}
	entry.specReservation = specReservation
	c.mu.Lock()
	if ctx.Err() != nil {
		c.mu.Unlock()
		reservation.Release()
		cleanup()
		c.finish(key, flight, nil, ctx.Err())
		return
	}
	entry.element = c.lru.PushFront(entry)
	stateTransferred = true
	c.byKey[key] = entry
	c.bySnapshot[entry.snapshotID] = entry
	var evicted []*reportEntry[T]
	for c.lru.Len() > c.opts.MaxEntries {
		old := c.lru.Back().Value.(*reportEntry[T])
		c.removeLocked(old)
		evicted = append(evicted, old)
	}
	c.mu.Unlock()
	for _, old := range evicted {
		c.dispose(old)
	}
	if owner != nil {
		// A cancelled first builder must not poison other coalesced owners.
		// Every waiter adopts independently before releasing the flight pin.
		_ = c.retainForOwner(ctx, entry, owner)
	}
	c.finish(key, flight, entry, nil)
}

func (c *ReportCache[T]) finish(key string, flight *reportFlight[T], entry *reportEntry[T], err error) {
	c.mu.Lock()
	if entry != nil && flight.waiters == 0 {
		entry.leases--
	}
	flight.entry, flight.err = entry, err
	delete(c.flights, key)
	close(flight.done)
	c.mu.Unlock()
	if entry != nil {
		c.disposeIfEvicted(entry)
	}
}

func (c *ReportCache[T]) disposeIfEvicted(entry *reportEntry[T]) {
	c.mu.Lock()
	dispose := !entry.active && entry.leases == 0 && !entry.disposed
	if dispose {
		entry.disposed = true
	}
	c.mu.Unlock()
	if dispose {
		c.releaseRetained(entry)
	}
}

func (c *ReportCache[T]) reserve(size int64) (*resource.Reservation, error) {
	if size > c.opts.Budget.Limit() {
		return nil, resource.NewLimitError("entry bytes", size, c.opts.Budget.Limit())
	}
	for {
		reservation, err := c.opts.Budget.Reserve(size)
		if err == nil {
			return reservation, nil
		}
		c.mu.Lock()
		oldest := c.lru.Back()
		if oldest == nil {
			c.mu.Unlock()
			return nil, err
		}
		entry := oldest.Value.(*reportEntry[T])
		c.removeLocked(entry)
		c.mu.Unlock()
		c.dispose(entry)
	}
}

func (c *ReportCache[T]) removeLocked(entry *reportEntry[T]) {
	if !entry.active {
		return
	}
	entry.active = false
	if c.byKey[entry.key] == entry {
		delete(c.byKey, entry.key)
	}
	if entry.derivedOwners == 0 {
		if c.bySnapshot[entry.snapshotID] == entry {
			delete(c.bySnapshot, entry.snapshotID)
		}
	}
	c.lru.Remove(entry.element)
}

func (c *ReportCache[T]) evict(entry *reportEntry[T]) {
	c.mu.Lock()
	active := entry.active
	c.removeLocked(entry)
	c.mu.Unlock()
	if active {
		c.dispose(entry)
	}
}

func (c *ReportCache[T]) dispose(entry *reportEntry[T]) {
	c.mu.Lock()
	dispose := entry.leases == 0 && !entry.disposed
	if dispose {
		entry.disposed = true
	}
	c.mu.Unlock()
	if dispose {
		c.releaseRetained(entry)
	}
}

func (c *ReportCache[T]) releaseRetained(entry *reportEntry[T]) {
	entry.handlers.ForgetSnapshot(context.Background(), entry.snapshotID)
	for _, child := range entry.derived {
		if child.forget != nil {
			child.forget()
		} else if child.snapshotID != entry.snapshotID {
			entry.handlers.ForgetSnapshot(context.Background(), child.snapshotID)
		}
	}
	entry.reservation.Release()
	entry.specReservation.Release()
	for _, child := range entry.derived {
		child.reservation.Release()
	}
	if c.opts.OnEvict != nil {
		c.opts.OnEvict(entry.state)
	}
}

// MaterializeDerived reuses the parent scheduler, with bounded distinct flights
// reserved before enqueueing work into that scheduler.
func (c *ReportCache[T]) MaterializeDerived(ctx context.Context, parentID, key string, class DerivedSurfaceClass, build func(context.Context) (*document.DashboardDocument, error)) (*document.DashboardDocument, error) {
	view, err := c.materializeDerived(ctx, parentID, key, class, build)
	c.countRejection(err)
	return view, err
}

func (c *ReportCache[T]) materializeDerived(ctx context.Context, parentID, key string, class DerivedSurfaceClass, build func(context.Context) (*document.DashboardDocument, error)) (*document.DashboardDocument, error) {
	if strings.TrimSpace(key) == "" || build == nil {
		return nil, fmt.Errorf("derived report requires key and builder")
	}
	registration, err := c.LookupSnapshotRegistration(ctx, parentID)
	if err != nil {
		return nil, err
	}
	defer registration.Release()
	c.mu.Lock()
	parent := c.bySnapshot[parentID]
	if parent == nil || !parent.active {
		c.mu.Unlock()
		return nil, document.ErrSnapshotGone
	}
	if cached := parent.derived[key]; cached != nil {
		c.mu.Unlock()
		if cached.snapshotID == parentID || !cached.backed {
			return cached.document.Document()
		}
		c.mu.Lock()
		childEntry := c.bySnapshot[cached.snapshotID]
		c.mu.Unlock()
		store := parent.handlers.snapshots
		if childEntry != nil {
			store = childEntry.handlers.snapshots
		}
		var liveErr error
		if cached.touch != nil {
			liveErr = cached.touch(ctx)
		} else if toucher, ok := store.(interface {
			Touch(context.Context, string) error
		}); ok {
			liveErr = toucher.Touch(ctx, cached.snapshotID)
		} else {
			_, liveErr = store.Get(ctx, cached.snapshotID)
		}
		if liveErr == nil {
			return cached.document.Document()
		}
		if !errors.Is(liveErr, document.ErrSnapshotGone) {
			return nil, liveErr
		}
		c.mu.Lock()
		if parent.derived[key] == cached {
			delete(parent.derived, key)
			cached.reservation.Release()
		}
		c.mu.Unlock()
		if cached.forget != nil {
			cached.forget()
		} else {
			parent.handlers.ForgetSnapshot(context.Background(), cached.snapshotID)
		}
		c.mu.Lock()
	}
	flightKey := parentID + ":" + key
	flight := c.derivedFlights[flightKey]
	if flight != nil && flight.waiters >= c.opts.MaxWaitersPerBuild {
		c.mu.Unlock()
		return nil, &resource.LimitError{Kind: resource.LimitAdmission, Resource: "report waiters", Requested: int64(flight.waiters + 1), Limit: int64(c.opts.MaxWaitersPerBuild)}
	}
	if flight == nil {
		limit := c.opts.MaxConcurrentBuilds + c.opts.MaxPendingBuilds
		if c.closed || len(c.flights)+len(c.derivedFlights) >= limit {
			requested := len(c.flights) + len(c.derivedFlights) + 1
			c.mu.Unlock()
			return nil, resource.NewLimitError("report_build_queue", int64(requested), int64(limit))
		}
		if len(parent.derived) >= c.opts.MaxDerivedPerReport {
			c.mu.Unlock()
			return nil, resource.NewLimitError("derived_reports", int64(len(parent.derived)+1), int64(c.opts.MaxDerivedPerReport))
		}
		workCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), c.opts.BuildTimeout)
		flight = &derivedReportFlight{done: make(chan struct{}), cancel: cancel, class: class}
		c.derivedFlights[flightKey] = flight
		parent.leases++
		go c.runDerived(workCtx, parent, flightKey, key, flight, build)
	} else if class == DerivedSurfaceInteractive {
		flight.class = class
	}
	flight.waiters++
	c.mu.Unlock()
	if class == DerivedSurfaceInteractive {
		parent.handlers.promoteDerived(parentID, "report:"+key)
	}
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
		return nil, ctx.Err()
	case <-flight.done:
		if flight.err != nil {
			return nil, flight.err
		}
		return flight.child.document.Document()
	}
}

func (c *ReportCache[T]) runDerived(ctx context.Context, parent *reportEntry[T], flightKey, key string, flight *derivedReportFlight, build func(context.Context) (*document.DashboardDocument, error)) {
	defer flight.cancel()
	finish := func(child *cachedDerived, err error) {
		c.mu.Lock()
		flight.child, flight.err = child, err
		delete(c.derivedFlights, flightKey)
		parent.leases--
		dispose := !parent.active && parent.leases == 0 && !parent.disposed
		if dispose {
			parent.disposed = true
		}
		close(flight.done)
		c.mu.Unlock()
		if dispose {
			c.releaseRetained(parent)
		}
	}
	workCtx, release, err := c.admit(ctx)
	if err != nil {
		finish(nil, err)
		return
	}
	defer release()
	c.mu.Lock()
	class := flight.class
	c.mu.Unlock()
	loaded, err := parent.handlers.executeDerivedSurface(workCtx, parent.snapshotID, "report:"+key, class, func(ctx context.Context) (any, error) {
		c.mu.Lock()
		active := parent.active
		c.mu.Unlock()
		if !active {
			return nil, document.ErrSnapshotGone
		}
		owner := &derivedBuildOwnership{sessions: parent.handlers.sessions, dataScope: parent.dataScope, children: map[string]*derivedChildReceipt{}, maxChildren: c.opts.MaxDerivedPerReport, acquireParent: func(ctx context.Context) (func(), error) {
			if ok, err := c.live(ctx, parent); err != nil {
				return nil, err
			} else if !ok {
				return nil, document.ErrSnapshotGone
			}
			lease, err := c.registration(ctx, parent, false)
			if err != nil {
				return nil, err
			}
			return lease.Release, nil
		}}
		ctx = context.WithValue(context.WithValue(ctx, derivedReportParentKey{}, parent.snapshotID), derivedBuildOwnershipKey{}, owner)
		doc, err := build(ctx)
		if err != nil {
			owner.take("")
			return nil, err
		}
		if doc == nil {
			owner.take("")
			return nil, fmt.Errorf("derived builder returned nil document")
		}
		receipt := owner.take(doc.SnapshotID)
		backed := receipt != nil
		if !backed && doc.SnapshotID != parent.snapshotID {
			lookupErr := touchReportSnapshot(ctx, parent.handlers.snapshots, doc.SnapshotID)
			if lookupErr == nil {
				backed = true
			} else if !errors.Is(lookupErr, document.ErrSnapshotGone) {
				return nil, lookupErr
			}
		}
		if !backed && (doc.Endpoints.Panel != "" || doc.Endpoints.Query != "") {
			return nil, document.ErrSnapshotGone
		}
		published := false
		defer func() {
			if !published && doc.SnapshotID != parent.snapshotID {
				if receipt != nil {
					receipt.forget()
				} else if backed {
					parent.handlers.ForgetSnapshot(context.Background(), doc.SnapshotID)
				}
			}
		}()
		frozen, err := document.Freeze(doc, c.opts.MaxDocumentBytes)
		if err != nil {
			return nil, err
		}
		reservation, err := c.opts.Budget.Reserve(frozen.Size())
		if err != nil {
			return nil, err
		}
		child := &cachedDerived{document: frozen, reservation: reservation, snapshotID: doc.SnapshotID}
		child.backed = backed
		if receipt != nil {
			child.forget = receipt.forget
			child.touch = receipt.touch
		}
		if doc.SnapshotID != parent.snapshotID {
			if err := parent.handlers.BindExecutionSession(doc.SnapshotID, parent.snapshotID); err != nil {
				reservation.Release()
				return nil, err
			}
		}
		c.mu.Lock()
		defer c.mu.Unlock()
		if !parent.active || ctx.Err() != nil {
			reservation.Release()
			return nil, document.ErrSnapshotGone
		}
		if len(parent.derived) >= c.opts.MaxDerivedPerReport {
			reservation.Release()
			return nil, resource.NewLimitError("derived_reports", int64(len(parent.derived)+1), int64(c.opts.MaxDerivedPerReport))
		}
		parent.derived[key] = child
		published = true
		return child, nil
	}, func() {
		c.mu.Lock()
		interactive := flight.class == DerivedSurfaceInteractive
		c.mu.Unlock()
		if interactive {
			parent.handlers.promoteDerived(parent.snapshotID, "report:"+key)
		}
	}, true)
	if err != nil {
		finish(nil, err)
		return
	}
	child, ok := loaded.(*cachedDerived)
	if !ok {
		finish(nil, fmt.Errorf("derived report returned %T", loaded))
		return
	}
	finish(child, nil)
}

type derivedReportFlight struct {
	done    chan struct{}
	cancel  context.CancelFunc
	waiters int
	class   DerivedSurfaceClass
	child   *cachedDerived
	err     error
}

type reportBuildContextKey struct{}
type derivedReportParentKey struct{}

// DerivedReportParent reports the parent identity supplied to a derived builder.
// ReportCache automatically incorporates it in nested materialization keys.
func DerivedReportParent(ctx context.Context) string {
	parent, _ := ctx.Value(derivedReportParentKey{}).(string)
	return parent
}

func reportKeyForContext(ctx context.Context, key ReportKey) ReportKey {
	if parent := DerivedReportParent(ctx); parent != "" {
		key.ParentSnapshotID = parent
	}
	return key
}

func (c *ReportCache[T]) admit(ctx context.Context) (context.Context, func(), error) {
	if ctx.Value(reportBuildContextKey{}) == c.opts.BuildLimiter {
		return ctx, func() {}, nil
	}
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return ctx, nil, context.Canceled
	}
	c.admitted++
	c.mu.Unlock()
	release, err := c.opts.BuildLimiter.acquire(ctx)
	if err != nil {
		c.mu.Lock()
		c.admitted--
		c.mu.Unlock()
		return ctx, nil, err
	}
	return context.WithValue(ctx, reportBuildContextKey{}, c.opts.BuildLimiter), func() { release(); c.mu.Lock(); c.admitted--; c.mu.Unlock() }, nil
}

// ReportBuildLimiter is shareable across registrations and caches, including
// caches with different host-state types. Nested synchronous builders inherit
// their parent's slot; the callback context must not escape into unrelated work.
type ReportBuildLimiter struct {
	mu            sync.Mutex
	slots         chan struct{}
	admitted, max int
}

// NewReportBuildLimiter admits at most concurrent builders plus pending waiters.
func NewReportBuildLimiter(concurrent, pending int) *ReportBuildLimiter {
	if concurrent <= 0 {
		concurrent = 4
	}
	if pending < 0 {
		pending = 0
	}
	return &ReportBuildLimiter{slots: make(chan struct{}, concurrent), max: concurrent + pending}
}

func (l *ReportBuildLimiter) acquire(ctx context.Context) (func(), error) {
	l.mu.Lock()
	if l.admitted >= l.max {
		requested := l.admitted + 1
		l.mu.Unlock()
		return nil, resource.NewLimitError("report_build_queue", int64(requested), int64(l.max))
	}
	l.admitted++
	l.mu.Unlock()
	select {
	case l.slots <- struct{}{}:
		return func() { <-l.slots; l.mu.Lock(); l.admitted--; l.mu.Unlock() }, nil
	case <-ctx.Done():
		l.mu.Lock()
		l.admitted--
		l.mu.Unlock()
		return nil, ctx.Err()
	}
}

// ReportCacheStats reports current ownership and request rejection counters.
type ReportCacheStats struct {
	Entries, Derived, Flights, Admitted int
	RetainedBytes                       int64
	Rejections                          uint64
}

// Stats reports shared Budget usage, including other owners using that Budget.
func (c *ReportCache[T]) Stats() ReportCacheStats {
	c.mu.Lock()
	defer c.mu.Unlock()
	stats := ReportCacheStats{Entries: len(c.byKey), Flights: len(c.flights) + len(c.derivedFlights), Admitted: c.admitted, RetainedBytes: c.opts.Budget.Used(), Rejections: c.rejections.Load()}
	for _, entry := range c.byKey {
		stats.Derived += len(entry.derived)
	}
	return stats
}

// Close rejects new work, cancels queued builds and evicts registrations.
// Outstanding leases retain their accounted state until Release.
func (c *ReportCache[T]) Close() {
	c.mu.Lock()
	c.closed = true
	for _, flight := range c.flights {
		flight.cancel()
	}
	for _, flight := range c.derivedFlights {
		flight.cancel()
	}
	entries := make([]*reportEntry[T], 0, len(c.byKey))
	for _, entry := range c.byKey {
		c.removeLocked(entry)
		entries = append(entries, entry)
	}
	c.mu.Unlock()
	for _, entry := range entries {
		c.dispose(entry)
	}
}

func touchReportSnapshot(ctx context.Context, store document.SnapshotStore, id string) error {
	if toucher, ok := store.(interface {
		Touch(context.Context, string) error
	}); ok {
		return toucher.Touch(ctx, id)
	}
	_, err := store.Get(ctx, id)
	return err
}

func (c *ReportCache[T]) retainForOwner(ctx context.Context, entry *reportEntry[T], owner *derivedBuildOwnership) error {
	lease, err := c.registrationWithParent(ctx, entry, true, false)
	if err != nil {
		return err
	}
	c.mu.Lock()
	entry.derivedOwners++
	c.bySnapshot[entry.snapshotID] = entry
	c.mu.Unlock()
	var once sync.Once
	return owner.add(entry.snapshotID, &derivedChildReceipt{
		touch: func(ctx context.Context) error {
			return touchReportSnapshot(ctx, entry.handlers.snapshots, entry.snapshotID)
		},
		forget: func() {
			once.Do(func() {
				c.mu.Lock()
				entry.derivedOwners--
				last := entry.derivedOwners == 0
				if last && !entry.active && c.bySnapshot[entry.snapshotID] == entry {
					delete(c.bySnapshot, entry.snapshotID)
				}
				c.mu.Unlock()
				if last {
					c.evict(entry)
				}
				lease.Release()
			})
		},
	})
}

func (c *ReportCache[T]) countRejection(err error) {
	if errors.Is(err, resource.ErrLimit) {
		c.rejections.Add(1)
	}
}

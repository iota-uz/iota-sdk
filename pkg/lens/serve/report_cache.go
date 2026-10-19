package serve

import (
	"container/list"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/lens"
	"github.com/iota-uz/iota-sdk/pkg/lens/document"
	"github.com/iota-uz/iota-sdk/pkg/lens/resource"
	lensruntime "github.com/iota-uz/iota-sdk/pkg/lens/runtime"
)

// ReportKey makes tenant, producer-version and presentation identity explicit.
// Params must contain normalized producer inputs, including permission variants.
type ReportKey struct {
	Namespace string
	DataScope string
	Version   string
	Locale    string
	Timezone  string
	Variant   string
	Params    string
}

func (k ReportKey) canonical() (string, error) {
	keyBytes := len(k.Namespace) + len(k.DataScope) + len(k.Version) + len(k.Locale) + len(k.Timezone) + len(k.Variant) + len(k.Params)
	if keyBytes > 64<<10 {
		return "", resource.NewLimitError("report key bytes", int64(keyBytes), 64<<10)
	}
	if strings.TrimSpace(k.Namespace) == "" || strings.TrimSpace(k.DataScope) == "" || strings.TrimSpace(k.Version) == "" {
		return "", fmt.Errorf("report key requires namespace, data scope and version")
	}
	encoded, err := json.Marshal(k)
	return string(encoded), err
}

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

func (r *CachedReport[T]) Spec() (lens.DashboardSpec, error) { return r.spec() }

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

func (r *ReportRegistration[T]) Spec() (lens.DashboardSpec, error) { return r.spec() }

func (r *ReportRegistration[T]) Release() {
	if r != nil && r.release != nil {
		r.release()
	}
}

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
}

type reportEntry[T any] struct {
	key             string
	snapshotID      string
	document        *document.FrozenDocument
	reservation     *resource.Reservation
	handlers        *Handlers
	specReservation *resource.Reservation
	state           T
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
}

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

func (c *ReportCache[T]) view(entry *reportEntry[T]) (*CachedReport[T], error) {
	registration, err := c.registration(entry, false)
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

func (c *ReportCache[T]) registration(entry *reportEntry[T], allowEvicted bool) (*ReportRegistration[T], error) {
	c.mu.Lock()
	if entry == nil || entry.disposed || (!entry.active && !allowEvicted) {
		c.mu.Unlock()
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
	return c.registration(entry, false)
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
	if !entry.active {
		return false, nil
	}
	c.lru.MoveToFront(entry.element)
	return true, nil
}

func (c *ReportCache[T]) Lookup(ctx context.Context, key ReportKey) (*CachedReport[T], bool, error) {
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
	view, err := c.view(entry)
	return view, err == nil, err
}

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
	return c.view(entry)
}

func (c *ReportCache[T]) Materialize(ctx context.Context, key ReportKey, build func(context.Context) (ReportDefinition[T], error)) (*CachedReport[T], error) {
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
		registration, err := c.registration(flight.entry, true)
		if err != nil {
			return nil, err
		}
		doc, err := flight.entry.document.Document()
		if err != nil {
			registration.Release()
			return nil, err
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
	if definition.Prepare != nil {
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
	delete(c.byKey, entry.key)
	delete(c.bySnapshot, entry.snapshotID)
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
		entry.handlers.ForgetSnapshot(context.Background(), child.snapshotID)
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
		if cached.snapshotID == parentID {
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
		if toucher, ok := store.(interface {
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
		parent.handlers.ForgetSnapshot(context.Background(), cached.snapshotID)
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
		doc, err := build(ctx)
		if err != nil {
			return nil, err
		}
		published := false
		defer func() {
			if !published && doc != nil && doc.SnapshotID != parent.snapshotID {
				parent.handlers.ForgetSnapshot(context.Background(), doc.SnapshotID)
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
	})
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

type ReportCacheStats struct {
	Entries, Derived, Flights, Admitted int
	RetainedBytes                       int64
}

func (c *ReportCache[T]) Stats() ReportCacheStats {
	c.mu.Lock()
	defer c.mu.Unlock()
	stats := ReportCacheStats{Entries: len(c.byKey), Flights: len(c.flights) + len(c.derivedFlights), Admitted: c.admitted, RetainedBytes: c.opts.Budget.Used()}
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

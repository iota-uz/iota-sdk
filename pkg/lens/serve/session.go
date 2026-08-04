package serve

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/lens"
	"github.com/iota-uz/iota-sdk/pkg/lens/datasource"
)

const (
	priorityInteractive  = 0
	priorityExport       = 50
	priorityRootBase     = 100
	priorityIntent       = 2000
	priorityIdlePrefetch = 3000
	priorityAgingStep    = 2 * time.Second
)

type scheduledResult struct {
	value any
	err   error
}

type scheduledJob struct {
	key        string
	priority   int
	order      int
	sequence   uint64
	queuedAt   time.Time
	background bool
	running    bool
	preempted  bool
	revision   int
	base       context.Context
	cancel     context.CancelFunc
	run        func(context.Context) (any, error)
	waiters    map[uint64]chan scheduledResult
}

type scheduledCall struct {
	result <-chan scheduledResult
	cancel func()
}

// DerivedSurfaceClass selects the SDK-owned dynamic priority for host-rendered
// child documents. Applications identify whether an HTTP request is an
// interactive activation or speculative intent; they never provide a numeric
// priority, wave, or concurrency budget.
type DerivedSurfaceClass string

const (
	DerivedSurfaceInteractive DerivedSurfaceClass = "interactive"
	DerivedSurfaceIntent      DerivedSurfaceClass = "intent"
)

func (c scheduledCall) Cancel() {
	if c.cancel != nil {
		c.cancel()
	}
}

type executionSession struct {
	mu                sync.Mutex
	jobs              map[string]*scheduledJob
	queue             []*scheduledJob
	running           int
	backgroundRunning int
	intentRunning     int
	idleRunning       int
	backgroundReady   bool
	runningForeground map[int]int
	maxConcurrency    int
	workTimeout       time.Duration
	nextSequence      uint64
	nextWaiter        uint64
	lastUsed          time.Time
	prefetchOnce      sync.Once
	prefetched        map[string]struct{}
	metric            func(Metric)
	activeRevision    int
	released          bool
}

// SessionRegistry lets independently registered Lens documents participate in
// one snapshot execution graph. A host uses one registry for related document
// handlers, then aliases a derived drawer snapshot to its parent after the
// child document has been minted.
type SessionRegistry struct {
	mu       sync.Mutex
	sessions map[string]*executionSession
	aliases  map[string]string
}

func (r *SessionRegistry) isDerived(snapshotID string) bool {
	if r == nil {
		return false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	_, ok := r.aliases[strings.TrimSpace(snapshotID)]
	return ok
}

func NewSessionRegistry() *SessionRegistry {
	return &SessionRegistry{
		sessions: make(map[string]*executionSession),
		aliases:  make(map[string]string),
	}
}

func (r *SessionRegistry) canonicalLocked(snapshotID string) string {
	seen := map[string]bool{}
	for {
		parent := r.aliases[snapshotID]
		if parent == "" || seen[parent] {
			return snapshotID
		}
		seen[snapshotID] = true
		snapshotID = parent
	}
}

func (r *SessionRegistry) bind(childSnapshotID, parentSnapshotID string) error {
	childSnapshotID = strings.TrimSpace(childSnapshotID)
	parentSnapshotID = strings.TrimSpace(parentSnapshotID)
	if childSnapshotID == "" || parentSnapshotID == "" || childSnapshotID == parentSnapshotID {
		return fmt.Errorf("session alias requires distinct child and parent snapshots")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	parentSnapshotID = r.canonicalLocked(parentSnapshotID)
	if childSnapshotID == parentSnapshotID {
		return fmt.Errorf("session alias creates a cycle")
	}
	if existing := r.sessions[childSnapshotID]; existing != nil {
		return fmt.Errorf("child snapshot session already started")
	}
	r.aliases[childSnapshotID] = parentSnapshotID
	return nil
}

func newExecutionSession(maxConcurrency int, workTimeout time.Duration, metrics ...func(Metric)) *executionSession {
	if maxConcurrency < 2 {
		maxConcurrency = 2
	}
	created := &executionSession{
		jobs:              make(map[string]*scheduledJob),
		runningForeground: make(map[int]int),
		prefetched:        make(map[string]struct{}),
		maxConcurrency:    maxConcurrency,
		workTimeout:       workTimeout,
		lastUsed:          time.Now(),
	}
	if len(metrics) > 0 {
		created.metric = metrics[0]
	}
	return created
}

func (s *executionSession) submit(
	base context.Context,
	key string,
	priority int,
	order int,
	run func(context.Context) (any, error),
	revisions ...int,
) scheduledCall {
	result := make(chan scheduledResult, 1)
	s.mu.Lock()
	if s.released {
		s.mu.Unlock()
		result <- scheduledResult{err: context.Canceled}
		close(result)
		return scheduledCall{result: result}
	}
	s.nextWaiter++
	waiterID := s.nextWaiter
	s.lastUsed = time.Now()
	revision := 0
	if len(revisions) > 0 {
		revision = revisions[0]
	}
	if existing := s.jobs[key]; existing != nil && !existing.preempted {
		s.reportLocked(Metric{Name: MetricRedundantWork, Value: 1, Labels: map[string]string{"class": priorityClass(priority)}})
		existing.waiters[waiterID] = result
		if priority < existing.priority {
			s.promoteLocked(existing, priority, order)
		}
		s.dispatchLocked()
		s.mu.Unlock()
		return scheduledCall{result: result, cancel: func() { s.detach(key, waiterID) }}
	}
	s.nextSequence++
	job := &scheduledJob{
		key: key, priority: priority, order: order, sequence: s.nextSequence,
		queuedAt:   time.Now(),
		background: priority >= priorityIntent,
		base:       base,
		revision:   revision,
		run:        run,
		waiters:    map[uint64]chan scheduledResult{waiterID: result},
	}
	s.jobs[key] = job
	s.queue = append(s.queue, job)
	s.dispatchLocked()
	s.mu.Unlock()
	return scheduledCall{result: result, cancel: func() { s.detach(key, waiterID) }}
}

func (s *executionSession) promoteLocked(job *scheduledJob, priority, order int) {
	if job == nil || priority >= job.priority {
		return
	}
	previousPriority := job.priority
	previousBackground := job.background
	job.priority = priority
	job.order = order
	job.background = priority >= priorityIntent
	if !job.running {
		return
	}
	s.adjustRunningLocked(previousPriority, previousBackground, -1)
	s.adjustRunningLocked(priority, job.background, 1)
}

func (s *executionSession) advanceRevision(revision int) {
	if revision <= 0 {
		return
	}
	s.mu.Lock()
	if revision <= s.activeRevision {
		s.mu.Unlock()
		return
	}
	s.activeRevision = revision
	queuedWaiters := make([]chan scheduledResult, 0)
	kept := s.queue[:0]
	for _, job := range s.queue {
		if job.revision > 0 && job.revision < revision && job.priority <= priorityIntent {
			delete(s.jobs, job.key)
			for _, waiter := range job.waiters {
				queuedWaiters = append(queuedWaiters, waiter)
			}
			continue
		}
		kept = append(kept, job)
	}
	s.queue = kept
	for _, job := range s.jobs {
		if job.running && job.revision > 0 && job.revision < revision && job.priority <= priorityIntent && job.cancel != nil {
			delete(s.jobs, job.key)
			job.cancel()
		}
	}
	s.mu.Unlock()
	for _, waiter := range queuedWaiters {
		waiter <- scheduledResult{err: context.Canceled}
		close(waiter)
	}
}

func (s *executionSession) detach(key string, waiterID uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	job := s.jobs[key]
	if job == nil {
		return
	}
	delete(job.waiters, waiterID)
	if len(job.waiters) > 0 {
		return
	}
	if job.running {
		if job.cancel != nil {
			if job.background {
				s.reportLocked(Metric{Name: MetricCancelledSpeculation, Value: 1, Labels: map[string]string{"class": priorityClass(job.priority)}})
			}
			job.cancel()
		}
		return
	}
	for index, queued := range s.queue {
		if queued == job {
			s.queue = append(s.queue[:index], s.queue[index+1:]...)
			break
		}
	}
	delete(s.jobs, key)
	if job.background {
		s.reportLocked(Metric{Name: MetricCancelledSpeculation, Value: 1, Labels: map[string]string{"class": priorityClass(job.priority)}})
	}
}

func (s *executionSession) dispatchLocked() {
	// Visible/interactive work must never sit behind a speculative query merely
	// because the latter claimed the last slot a moment earlier. Cancellation
	// is cooperative at the datasource boundary; once the background job exits,
	// dispatch immediately fills the released slot with foreground work.
	if s.running >= s.maxConcurrency && s.bestQueuedLocked(false) >= 0 {
		s.preemptBackgroundLocked()
	}
	for s.running < s.maxConcurrency {
		index := s.nextJobLocked()
		if index < 0 {
			return
		}
		job := s.queue[index]
		s.queue = append(s.queue[:index], s.queue[index+1:]...)
		job.running = true
		s.running++
		s.reportLocked(Metric{Name: MetricSchedulerSaturation, Value: float64(s.running) / float64(s.maxConcurrency)})
		s.adjustRunningLocked(job.priority, job.background, 1)
		go s.execute(job)
	}
}

func (s *executionSession) preemptBackgroundLocked() {
	var candidate *scheduledJob
	for _, job := range s.jobs {
		if !job.running || !job.background || job.cancel == nil {
			continue
		}
		if candidate == nil || job.priority > candidate.priority ||
			(job.priority == candidate.priority && job.sequence > candidate.sequence) {
			candidate = job
		}
	}
	if candidate == nil {
		return
	}
	s.reportLocked(Metric{Name: MetricCancelledSpeculation, Value: 1, Labels: map[string]string{"class": priorityClass(candidate.priority), "reason": "foreground_preemption"}})
	candidate.preempted = true
	candidate.cancel()
	candidate.cancel = nil
}

func (s *executionSession) nextJobLocked() int {
	foregroundIndex := s.bestQueuedLocked(false)
	if foregroundIndex >= 0 {
		queuedPriority := agedPriority(s.queue[foregroundIndex], time.Now(), s.backgroundReady)
		blocked := false
		for runningPriority, count := range s.runningForeground {
			if count > 0 && queuedPriority > runningPriority {
				blocked = true
				break
			}
		}
		if !blocked {
			return foregroundIndex
		}
	}
	if !s.backgroundReady || s.backgroundRunning >= min(2, s.maxConcurrency) {
		return -1
	}
	return s.bestRunnableBackgroundLocked()
}

// bestRunnableBackgroundLocked gives intent and idle speculation independent
// bounded lanes. An expensive SDK-owned idle traversal can keep making progress
// without preventing a derived document's first row from warming in parallel.
// Foreground still owns the global cap and preempts speculation when saturated.
func (s *executionSession) bestRunnableBackgroundLocked() int {
	best := -1
	now := time.Now()
	for index, job := range s.queue {
		if !job.background {
			continue
		}
		if job.priority >= priorityIdlePrefetch {
			if s.idleRunning > 0 {
				continue
			}
		} else if s.intentRunning > 0 {
			continue
		}
		if best < 0 || lessScheduledJob(job, s.queue[best], now, s.backgroundReady) {
			best = index
		}
	}
	return best
}

func (s *executionSession) bestQueuedLocked(background bool) int {
	best := -1
	now := time.Now()
	for index, job := range s.queue {
		if job.background != background {
			continue
		}
		if best < 0 || lessScheduledJob(job, s.queue[best], now, s.backgroundReady) {
			best = index
		}
	}
	return best
}

func lessScheduledJob(left, right *scheduledJob, now time.Time, allowRootAging bool) bool {
	leftPriority := agedPriority(left, now, allowRootAging)
	rightPriority := agedPriority(right, now, allowRootAging)
	return leftPriority < rightPriority ||
		(leftPriority == rightPriority && left.order < right.order) ||
		(leftPriority == rightPriority && left.order == right.order && left.sequence < right.sequence)
}

func agedPriority(job *scheduledJob, now time.Time, allowRootAging bool) int {
	floor := priorityRootBase
	switch {
	case job.priority == priorityInteractive:
		return priorityInteractive
	case job.priority >= priorityIdlePrefetch:
		floor = priorityIdlePrefetch
	case job.priority >= priorityIntent:
		floor = priorityIntent
	case !allowRootAging:
		return job.priority
	}
	age := now.Sub(job.queuedAt)
	if age <= 0 {
		return job.priority
	}
	return max(floor, job.priority-int(age/priorityAgingStep))
}

func (s *executionSession) execute(job *scheduledJob) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(job.base), s.workTimeout)
	s.mu.Lock()
	job.cancel = cancel
	s.reportLocked(Metric{
		Name:   MetricQueueWait,
		Value:  float64(time.Since(job.queuedAt).Microseconds()) / 1000,
		Labels: map[string]string{"class": priorityClass(job.priority)},
	})
	if len(job.waiters) == 0 {
		cancel()
	}
	s.mu.Unlock()
	started := time.Now()
	value, err := job.run(ctx)
	cancel()

	s.mu.Lock()
	outcome := "ok"
	if err != nil {
		outcome = "error"
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			outcome = "cancelled"
		}
	}
	s.reportLocked(Metric{
		Name:   MetricExecutionDuration,
		Value:  float64(time.Since(started).Microseconds()) / 1000,
		Labels: map[string]string{"class": priorityClass(job.priority), "outcome": outcome},
	})
	if s.jobs[job.key] == job {
		delete(s.jobs, job.key)
	}
	s.running--
	s.adjustRunningLocked(job.priority, job.background, -1)
	s.lastUsed = time.Now()
	waiters := make([]chan scheduledResult, 0, len(job.waiters))
	for _, waiter := range job.waiters {
		waiters = append(waiters, waiter)
	}
	s.dispatchLocked()
	s.mu.Unlock()

	for _, waiter := range waiters {
		waiter <- scheduledResult{value: value, err: err}
		close(waiter)
	}
}

func (s *executionSession) adjustRunningLocked(priority int, background bool, delta int) {
	if background {
		s.backgroundRunning += delta
		if priority >= priorityIdlePrefetch {
			s.idleRunning += delta
		} else {
			s.intentRunning += delta
		}
		return
	}
	if delta > 0 {
		s.runningForeground[priority] += delta
		return
	}
	if s.runningForeground[priority] <= -delta {
		delete(s.runningForeground, priority)
		return
	}
	s.runningForeground[priority] += delta
}

func (s *executionSession) enableBackground() {
	s.mu.Lock()
	if s.released {
		s.mu.Unlock()
		return
	}
	s.backgroundReady = true
	s.dispatchLocked()
	s.mu.Unlock()
}

// cancelBackground drops every queued speculative job and interrupts running
// speculative work. Foreground/interactive jobs retain their own request
// lifecycle and are deliberately left alone.
func (s *executionSession) cancelBackground() {
	s.mu.Lock()
	waiters := s.cancelBackgroundLocked()
	s.mu.Unlock()
	deliverCancellation(waiters)
}

func (s *executionSession) release() {
	s.mu.Lock()
	s.released = true
	waiters := s.cancelBackgroundLocked()
	s.mu.Unlock()
	deliverCancellation(waiters)
}

func (s *executionSession) cancelBackgroundLocked() []chan scheduledResult {
	waiters := make([]chan scheduledResult, 0)
	kept := s.queue[:0]
	for _, job := range s.queue {
		if !job.background {
			kept = append(kept, job)
			continue
		}
		delete(s.jobs, job.key)
		for _, waiter := range job.waiters {
			waiters = append(waiters, waiter)
		}
		s.reportLocked(Metric{Name: MetricCancelledSpeculation, Value: 1, Labels: map[string]string{"class": priorityClass(job.priority)}})
	}
	s.queue = kept
	for _, job := range s.jobs {
		if job.running && job.background && job.cancel != nil {
			job.cancel()
		}
	}
	s.backgroundReady = false
	return waiters
}

func deliverCancellation(waiters []chan scheduledResult) {
	for _, waiter := range waiters {
		waiter <- scheduledResult{err: context.Canceled}
		close(waiter)
	}
}

func (s *executionSession) idleBefore(cutoff time.Time) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running == 0 && len(s.queue) == 0 && s.lastUsed.Before(cutoff)
}

func priorityClass(priority int) string {
	switch {
	case priority == priorityInteractive:
		return "interactive"
	case priority == priorityExport:
		return "export"
	case priority < priorityIntent:
		return "root"
	case priority < priorityIdlePrefetch:
		return "intent_prefetch"
	default:
		return "idle_prefetch"
	}
}

func datasourceExecutionClass(priority int) datasource.ExecutionClass {
	switch {
	case priority == priorityInteractive:
		return datasource.ExecutionClassInteractiveCritical
	case priority == priorityExport:
		return datasource.ExecutionClassExport
	case priority == priorityRootBase:
		return datasource.ExecutionClassRootCritical
	case priority < priorityIntent:
		return datasource.ExecutionClassActiveBelowFold
	case priority < priorityIdlePrefetch:
		return datasource.ExecutionClassIntentPrefetch
	default:
		return datasource.ExecutionClassIdlePrefetch
	}
}

func (h *Handlers) effectivePanelPriority(snapshotID, prefetchHeader string, layoutPriority int) int {
	if strings.EqualFold(strings.TrimSpace(prefetchHeader), "intent") {
		return priorityIntent
	}
	if h.sessions != nil && h.sessions.isDerived(snapshotID) {
		return priorityInteractive
	}
	return layoutPriority
}

func (s *executionSession) reportLocked(metric Metric) {
	if s.metric != nil {
		s.metric(metric)
	}
}

func (s *executionSession) markPrefetched(key string) {
	s.mu.Lock()
	s.prefetched[key] = struct{}{}
	s.mu.Unlock()
}

func (s *executionSession) wasPrefetched(key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.prefetched[key]
	return ok
}

func (h *Handlers) session(snapshotID string) *executionSession {
	registry := h.sessions
	if registry == nil {
		registry = NewSessionRegistry()
		h.sessions = registry
	}
	registry.mu.Lock()
	defer registry.mu.Unlock()
	snapshotID = registry.canonicalLocked(snapshotID)
	cutoff := time.Now().Add(-2 * h.workTimeout)
	for id, session := range registry.sessions {
		if session.idleBefore(cutoff) {
			delete(registry.sessions, id)
		}
	}
	if existing := registry.sessions[snapshotID]; existing != nil {
		return existing
	}
	created := newExecutionSession(defaultConcurrency, h.workTimeout, func(metric Metric) {
		h.observeMetric(context.Background(), metric)
	})
	registry.sessions[snapshotID] = created
	return created
}

func (h *Handlers) releaseSession(snapshotID string) {
	registry := h.sessions
	if registry == nil {
		return
	}
	registry.mu.Lock()
	if _, derived := registry.aliases[snapshotID]; derived {
		// A derived document may be reopened from the browser's immutable
		// document cache without another materialization request. Keep its alias
		// for the lifetime of the parent snapshot so every reopen continues to
		// share the parent's scheduler and singleflight graph.
		registry.mu.Unlock()
		return
	}
	snapshotID = registry.canonicalLocked(snapshotID)
	session := registry.sessions[snapshotID]
	delete(registry.sessions, snapshotID)
	for child, parent := range registry.aliases {
		if registry.canonicalLocked(parent) == snapshotID {
			delete(registry.aliases, child)
		}
	}
	registry.mu.Unlock()
	if session != nil {
		session.release()
	}
}

// BindExecutionSession joins a derived document snapshot to the parent's
// scheduler. It must be called before the child receives its first panel,
// exploration, export or derived-document request.
func (h *Handlers) BindExecutionSession(childSnapshotID, parentSnapshotID string) error {
	if h.sessions == nil {
		h.sessions = NewSessionRegistry()
	}
	return h.sessions.bind(childSnapshotID, parentSnapshotID)
}

// ExecuteExportSurface runs a snapshot export in the same execution graph as
// root panels, explorations and derived documents. Export is user-visible
// foreground work, but unlike viewport-driven root rendering it deliberately
// submits one throughput-oriented scope to the engine: the engine may resolve
// all required datasets concurrently while its runtime memo reuses dependency
// work already completed by panel requests from this snapshot.
func (h *Handlers) ExecuteExportSurface(
	ctx context.Context,
	snapshotID string,
	key string,
	run func(context.Context) (any, error),
) (any, error) {
	snapshotID = strings.TrimSpace(snapshotID)
	key = strings.TrimSpace(key)
	if snapshotID == "" || key == "" || run == nil {
		return nil, fmt.Errorf("export surface requires snapshot, key and executor")
	}
	call := h.session(snapshotID).submit(ctx, "export:"+snapshotID+":"+key, priorityExport, 0, run)
	defer call.Cancel()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case result := <-call.result:
		return result.value, result.err
	}
}

// ExecuteDerivedSurface joins a host-rendered child document to the snapshot's
// scheduler and singleflight namespace. A click promotes an in-flight intent
// request with the same key instead of starting a second calculation.
func (h *Handlers) ExecuteDerivedSurface(
	ctx context.Context,
	snapshotID string,
	key string,
	class DerivedSurfaceClass,
	run func(context.Context) (any, error),
) (any, error) {
	snapshotID = strings.TrimSpace(snapshotID)
	key = strings.TrimSpace(key)
	if snapshotID == "" || key == "" || run == nil {
		return nil, fmt.Errorf("derived surface requires snapshot, key and executor")
	}
	priority := priorityInteractive
	if class == DerivedSurfaceIntent {
		priority = priorityIntent
	}
	call := h.session(snapshotID).submit(ctx, "derived:"+snapshotID+":"+key, priority, 0, run)
	defer call.Cancel()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case result := <-call.result:
		return result.value, result.err
	}
}

type panelPriority struct {
	priority int
	order    int
}

type panelPriorities map[string]panelPriority

func panelExecutionPriorities(plan lens.ExecutionPlan) panelPriorities {
	result := make(panelPriorities)
	for _, surface := range plan.Surfaces {
		if surface.Kind != lens.ExecutionSurfaceRoot {
			continue
		}
		result[surface.PanelID] = panelPriority{
			priority: priorityRootBase + surface.Depth,
			order:    surface.VisualOrder,
		}
	}
	return result
}

func (p panelPriorities) forPanel(panelID string) (int, int) {
	if value, ok := p[panelID]; ok {
		return value.priority, value.order
	}
	return priorityRootBase + len(p), len(p)
}

func (p panelPriorities) forRequest(request PanelRequest) (int, int) {
	priority, order := p.forPanel(request.PanelID)
	// The first content row remains exclusive on a cold document. Once a
	// panel is below it, a browser-measured viewport band may bring useful
	// work forward without letting host dashboard code declare priorities.
	if request.ViewportRank != nil && priority > priorityRootBase {
		priority = priorityRootBase + 1 + *request.ViewportRank
	}
	return priority, order
}

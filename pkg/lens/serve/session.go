package serve

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/lens"
)

const (
	priorityInteractive  = 0
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
	s.nextWaiter++
	waiterID := s.nextWaiter
	s.lastUsed = time.Now()
	revision := 0
	if len(revisions) > 0 {
		revision = revisions[0]
	}
	if existing := s.jobs[key]; existing != nil {
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
	if previousBackground {
		s.backgroundRunning--
	} else if s.runningForeground[previousPriority] <= 1 {
		delete(s.runningForeground, previousPriority)
	} else {
		s.runningForeground[previousPriority]--
	}
	if job.background {
		s.backgroundRunning++
	} else {
		s.runningForeground[priority]++
	}
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
		if job.background {
			s.backgroundRunning++
		} else {
			s.runningForeground[job.priority]++
		}
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
	candidate.cancel()
	candidate.cancel = nil
}

func (s *executionSession) nextJobLocked() int {
	foregroundIndex := s.bestQueuedLocked(false)
	backgroundIndex := s.bestQueuedLocked(true)
	if backgroundIndex >= 0 && s.backgroundReady && s.backgroundRunning == 0 && (s.maxConcurrency > 1 || foregroundIndex < 0) {
		return backgroundIndex
	}
	if foregroundIndex < 0 {
		return -1
	}
	queuedPriority := agedPriority(s.queue[foregroundIndex], time.Now(), s.backgroundReady)
	for runningPriority, count := range s.runningForeground {
		if count > 0 && queuedPriority > runningPriority {
			return -1
		}
	}
	return foregroundIndex
}

func (s *executionSession) bestQueuedLocked(background bool) int {
	best := -1
	now := time.Now()
	for index, job := range s.queue {
		if job.background != background {
			continue
		}
		jobPriority := agedPriority(job, now, s.backgroundReady)
		bestPriority := 0
		if best >= 0 {
			bestPriority = agedPriority(s.queue[best], now, s.backgroundReady)
		}
		if best < 0 || jobPriority < bestPriority ||
			(jobPriority == bestPriority && job.order < s.queue[best].order) ||
			(jobPriority == bestPriority && job.order == s.queue[best].order && job.sequence < s.queue[best].sequence) {
			best = index
		}
	}
	return best
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
	if len(job.waiters) == 0 {
		cancel()
	}
	s.mu.Unlock()
	value, err := job.run(ctx)
	cancel()

	s.mu.Lock()
	if s.jobs[job.key] == job {
		delete(s.jobs, job.key)
	}
	s.running--
	if job.background {
		s.backgroundRunning--
	} else if s.runningForeground[job.priority] <= 1 {
		delete(s.runningForeground, job.priority)
	} else {
		s.runningForeground[job.priority]--
	}
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

func (s *executionSession) enableBackground() {
	s.mu.Lock()
	s.backgroundReady = true
	s.dispatchLocked()
	s.mu.Unlock()
}

// cancelBackground drops every queued speculative job and interrupts running
// speculative work. Foreground/interactive jobs retain their own request
// lifecycle and are deliberately left alone.
func (s *executionSession) cancelBackground() {
	s.mu.Lock()
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
	s.mu.Unlock()
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
	case priority < priorityIntent:
		return "root"
	case priority < priorityIdlePrefetch:
		return "intent_prefetch"
	default:
		return "idle_prefetch"
	}
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
	h.sessionsMu.Lock()
	defer h.sessionsMu.Unlock()
	cutoff := time.Now().Add(-2 * h.workTimeout)
	for id, session := range h.sessions {
		if session.idleBefore(cutoff) {
			delete(h.sessions, id)
		}
	}
	if existing := h.sessions[snapshotID]; existing != nil {
		return existing
	}
	created := newExecutionSession(defaultConcurrency, h.workTimeout, func(metric Metric) {
		h.observeMetric(context.Background(), metric)
	})
	h.sessions[snapshotID] = created
	return created
}

func (h *Handlers) releaseSession(snapshotID string) {
	h.sessionsMu.Lock()
	session := h.sessions[snapshotID]
	delete(h.sessions, snapshotID)
	h.sessionsMu.Unlock()
	if session != nil {
		session.cancelBackground()
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

func (p panelPriorities) compare(left, right string) int {
	leftPriority, leftOrder := p.forPanel(left)
	rightPriority, rightOrder := p.forPanel(right)
	if leftPriority != rightPriority {
		return leftPriority - rightPriority
	}
	return leftOrder - rightOrder
}

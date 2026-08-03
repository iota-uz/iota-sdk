package serve

import (
	"context"
	"sync"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/lens"
	"github.com/iota-uz/iota-sdk/pkg/lens/panel"
)

const (
	priorityInteractive  = 0
	priorityRootBase     = 100
	priorityIntent       = 2000
	priorityIdlePrefetch = 3000
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
	background bool
	running    bool
	base       context.Context
	cancel     context.CancelFunc
	run        func(context.Context) (any, error)
	waiters    map[uint64]chan scheduledResult
}

type scheduledCall struct {
	result <-chan scheduledResult
	cancel func()
}

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
) scheduledCall {
	result := make(chan scheduledResult, 1)
	s.mu.Lock()
	s.nextWaiter++
	waiterID := s.nextWaiter
	s.lastUsed = time.Now()
	if existing := s.jobs[key]; existing != nil {
		s.reportLocked(Metric{Name: MetricRedundantWork, Value: 1, Labels: map[string]string{"class": priorityClass(priority)}})
		existing.waiters[waiterID] = result
		if !existing.running && priority < existing.priority {
			existing.priority = priority
			existing.order = order
			existing.background = priority >= priorityIntent
		}
		s.dispatchLocked()
		s.mu.Unlock()
		return scheduledCall{result: result, cancel: func() { s.detach(key, waiterID) }}
	}
	s.nextSequence++
	job := &scheduledJob{
		key: key, priority: priority, order: order, sequence: s.nextSequence,
		background: priority >= priorityIntent,
		base:       base,
		run:        run,
		waiters:    map[uint64]chan scheduledResult{waiterID: result},
	}
	s.jobs[key] = job
	s.queue = append(s.queue, job)
	s.dispatchLocked()
	s.mu.Unlock()
	return scheduledCall{result: result, cancel: func() { s.detach(key, waiterID) }}
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

func (s *executionSession) nextJobLocked() int {
	foregroundIndex := s.bestQueuedLocked(false)
	backgroundIndex := s.bestQueuedLocked(true)
	if backgroundIndex >= 0 && s.backgroundReady && s.backgroundRunning == 0 && (s.maxConcurrency > 1 || foregroundIndex < 0) {
		return backgroundIndex
	}
	if foregroundIndex < 0 {
		return -1
	}
	queuedPriority := s.queue[foregroundIndex].priority
	for runningPriority, count := range s.runningForeground {
		if count > 0 && queuedPriority > runningPriority {
			return -1
		}
	}
	return foregroundIndex
}

func (s *executionSession) bestQueuedLocked(background bool) int {
	best := -1
	for index, job := range s.queue {
		if job.background != background {
			continue
		}
		if best < 0 || job.priority < s.queue[best].priority ||
			(job.priority == s.queue[best].priority && job.order < s.queue[best].order) ||
			(job.priority == s.queue[best].priority && job.order == s.queue[best].order && job.sequence < s.queue[best].sequence) {
			best = index
		}
	}
	return best
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
	delete(s.jobs, job.key)
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

type panelPriority struct {
	priority int
	order    int
}

type panelPriorities map[string]panelPriority

func panelExecutionPriorities(spec lens.DashboardSpec) panelPriorities {
	result := make(panelPriorities)
	order := 0
	var add func(panel.Spec, int)
	add = func(candidate panel.Spec, row int) {
		if !candidate.Kind.IsContainer() {
			result[candidate.ID] = panelPriority{priority: priorityRootBase + row, order: order}
			order++
		}
		for _, child := range candidate.Children {
			add(child, row)
		}
	}
	for row, layout := range spec.Rows {
		for _, candidate := range layout.Panels {
			add(candidate, row)
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

package spotlight

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
	"github.com/sirupsen/logrus"
)

type Agent interface {
	Answer(ctx context.Context, req SearchRequest, hits []SearchHit) (*AgentAnswer, error)
}

const (
	defaultSearchTimeout       = 220 * time.Millisecond
	defaultAgentTimeout        = 80 * time.Millisecond
	defaultSearchCacheTTL      = 2 * time.Second
	defaultSearchCacheEntries  = 512
	defaultSearchLatencyBudget = 350 * time.Millisecond
	defaultBackgroundTick      = 5 * time.Second
)

type ServiceConfig struct {
	SearchTimeout            time.Duration
	AgentTimeout             time.Duration
	SearchCacheTTL           time.Duration
	SearchCacheMaxEntries    int
	SearchLatencyBudget      time.Duration
	AllowStaleCacheOnTimeout bool
	BackgroundIndexerTick    time.Duration
}

func DefaultServiceConfig() ServiceConfig {
	return ServiceConfig{
		SearchTimeout:            defaultSearchTimeout,
		AgentTimeout:             defaultAgentTimeout,
		SearchCacheTTL:           defaultSearchCacheTTL,
		SearchCacheMaxEntries:    defaultSearchCacheEntries,
		SearchLatencyBudget:      defaultSearchLatencyBudget,
		AllowStaleCacheOnTimeout: true,
		BackgroundIndexerTick:    defaultBackgroundTick,
	}
}

func (c ServiceConfig) normalized() ServiceConfig {
	cfg := c
	if cfg.SearchTimeout <= 0 {
		cfg.SearchTimeout = defaultSearchTimeout
	}
	if cfg.AgentTimeout <= 0 {
		cfg.AgentTimeout = defaultAgentTimeout
	}
	if cfg.SearchCacheTTL <= 0 {
		cfg.SearchCacheTTL = defaultSearchCacheTTL
	}
	if cfg.SearchCacheMaxEntries <= 0 {
		cfg.SearchCacheMaxEntries = defaultSearchCacheEntries
	}
	if cfg.SearchLatencyBudget <= 0 {
		cfg.SearchLatencyBudget = defaultSearchLatencyBudget
	}
	if cfg.BackgroundIndexerTick <= 0 {
		cfg.BackgroundIndexerTick = defaultBackgroundTick
	}
	return cfg
}

type Service interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	RegisterProvider(provider SearchProvider)
	SetAgent(agent Agent)
	ReindexTenant(ctx context.Context, tenantID uuid.UUID, language string) error
	Readiness(ctx context.Context) error
	Search(ctx context.Context, req SearchRequest) (SearchResponse, error)
}

type ServiceOption func(*SpotlightService)

func WithOutboxProcessor(processor OutboxProcessor) ServiceOption {
	return func(s *SpotlightService) {
		if processor == nil {
			return
		}
		s.outbox = processor
	}
}

func WithPrincipalResolver(resolver PrincipalResolver) ServiceOption {
	return func(s *SpotlightService) {
		if resolver == nil {
			return
		}
		s.acl = NewStrictACLEvaluator(resolver)
	}
}

func WithACLEvaluator(evaluator ACLEvaluator) ServiceOption {
	return func(s *SpotlightService) {
		if evaluator != nil {
			s.acl = evaluator
		}
	}
}

func WithRanker(ranker Ranker) ServiceOption {
	return func(s *SpotlightService) {
		if ranker != nil {
			s.ranker = ranker
		}
	}
}

func WithGrouper(grouper Grouper) ServiceOption {
	return func(s *SpotlightService) {
		if grouper != nil {
			s.grouper = grouper
		}
	}
}

func WithMetrics(metrics Metrics) ServiceOption {
	return func(s *SpotlightService) {
		if metrics != nil {
			s.metrics = metrics
		}
	}
}

func WithLogger(logger *logrus.Logger) ServiceOption {
	return func(s *SpotlightService) {
		if logger != nil {
			s.logger = logger
		}
	}
}

type SpotlightService struct {
	cfg      ServiceConfig
	registry *ProviderRegistry
	scope    ScopeStore
	acl      ACLEvaluator
	engine   IndexEngine
	pipeline *IndexerPipeline
	ranker   Ranker
	grouper  Grouper
	metrics  Metrics
	logger   *logrus.Logger

	mu    sync.RWMutex
	agent Agent

	outbox OutboxProcessor

	cacheMu     sync.RWMutex
	searchCache map[string]cachedSearchResponse

	indexQueue    chan indexTask
	enqueueOnce   sync.Map
	watchStarted  sync.Map
	lifecycleMu   sync.Mutex
	started       bool
	startedAtomic atomic.Bool
	bgCtx         context.Context
	bgCancel      context.CancelFunc
	wg            sync.WaitGroup
}

type indexTask struct {
	tenantID uuid.UUID
	language string
}

type cachedSearchResponse struct {
	response  SearchResponse
	expiresAt time.Time
	storedAt  time.Time
}

func NewService(engine IndexEngine, agent Agent, cfg ServiceConfig, opts ...ServiceOption) *SpotlightService {
	registry := NewProviderRegistry()
	scope := NewInMemoryScopeStore()
	acl := NewStrictACLEvaluator(NewComposablesPrincipalResolver())
	normalizedCfg := cfg.normalized()

	svc := &SpotlightService{
		cfg:         normalizedCfg,
		registry:    registry,
		scope:       scope,
		acl:         acl,
		engine:      engine,
		pipeline:    NewIndexerPipeline(registry, engine),
		ranker:      NewDefaultRanker(),
		grouper:     NewDefaultGrouper(),
		metrics:     NewNoopMetrics(),
		logger:      logrus.StandardLogger(),
		agent:       agent,
		searchCache: make(map[string]cachedSearchResponse),
		indexQueue:  make(chan indexTask, 256),
	}
	for _, opt := range opts {
		opt(svc)
	}
	return svc
}

func (s *SpotlightService) Start(_ context.Context) error {
	s.lifecycleMu.Lock()
	defer s.lifecycleMu.Unlock()
	if s.started {
		return nil
	}
	s.bgCtx, s.bgCancel = context.WithCancel(context.Background())
	s.started = true
	s.startedAtomic.Store(true)
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.runBackgroundIndexer(s.bgCtx, s.cfg.BackgroundIndexerTick)
	}()
	return nil
}

func (s *SpotlightService) Stop(ctx context.Context) error {
	s.lifecycleMu.Lock()
	if !s.started {
		s.lifecycleMu.Unlock()
		return nil
	}
	cancel := s.bgCancel
	s.started = false
	s.startedAtomic.Store(false)
	s.bgCancel = nil
	s.bgCtx = nil
	s.lifecycleMu.Unlock()

	if cancel != nil {
		cancel()
	}

	waitDone := make(chan struct{})
	go func() {
		defer close(waitDone)
		s.wg.Wait()
	}()

	if ctx == nil {
		ctx = context.Background()
	}
	select {
	case <-waitDone:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *SpotlightService) RegisterProvider(provider SearchProvider) {
	s.registry.Register(provider)
}

func (s *SpotlightService) SetAgent(agent Agent) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.agent = agent
}

func (s *SpotlightService) ReindexTenant(ctx context.Context, tenantID uuid.UUID, language string) error {
	const op serrors.Op = "spotlight.SpotlightService.ReindexTenant"

	scope, err := s.scope.Resolve(ctx, SearchRequest{
		TenantID: tenantID,
		Language: language,
		Intent:   SearchIntentMixed,
	})
	if err != nil {
		return serrors.E(op, err)
	}
	if err := s.pipeline.Sync(ctx, tenantID, language, "", 0, scope); err != nil {
		return serrors.E(op, err)
	}
	return nil
}

func (s *SpotlightService) Readiness(ctx context.Context) error {
	return s.engine.Health(ctx)
}

func (s *SpotlightService) Search(ctx context.Context, req SearchRequest) (SearchResponse, error) {
	startedAt := time.Now()
	telemetry := SearchTelemetry{
		Budget: s.cfg.SearchLatencyBudget,
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if !s.isStarted() {
		if err := s.Start(ctx); err != nil {
			telemetry.Err = err
			telemetry.TotalTook = time.Since(startedAt)
			telemetry.OverBudget = telemetry.TotalTook > telemetry.Budget
			s.metrics.OnSearch(req, telemetry)
			return SearchResponse{}, fmt.Errorf("service not started: %w", err)
		}
	}
	if req.TopK <= 0 {
		req.TopK = 20
	}
	if req.Intent == "" {
		req.Intent = SearchIntentMixed
	}
	req.Query = strings.TrimSpace(req.Query)
	req.Roles = dedupeAndSort(req.Roles)
	req.Permissions = dedupeAndSort(req.Permissions)

	if cached, ok := s.getCachedSearch(req, false); ok {
		telemetry.CacheHit = true
		telemetry.TotalTook = time.Since(startedAt)
		telemetry.OverBudget = telemetry.TotalTook > telemetry.Budget
		s.metrics.OnSearch(req, telemetry)
		return cached, nil
	}

	s.scheduleIndexRefresh(req.TenantID, req.Language)
	s.ensureProviderWatchers(req.TenantID, req.Language)

	searchCtx, cancelSearch := withTimeoutRespectingDeadline(ctx, s.cfg.SearchTimeout)
	engineStarted := time.Now()
	hits, err := s.engine.Search(searchCtx, req)
	telemetry.EngineTook = time.Since(engineStarted)
	searchCtxErr := searchCtx.Err()
	cancelSearch()
	if err != nil {
		if s.cfg.AllowStaleCacheOnTimeout && (errors.Is(err, context.DeadlineExceeded) || errors.Is(searchCtxErr, context.DeadlineExceeded)) {
			if stale, ok := s.getCachedSearch(req, true); ok {
				telemetry.CacheHit = true
				telemetry.CacheStale = true
				telemetry.TotalTook = time.Since(startedAt)
				telemetry.OverBudget = telemetry.TotalTook > telemetry.Budget
				s.metrics.OnSearch(req, telemetry)
				return stale, nil
			}
		}
		telemetry.Err = err
		telemetry.TotalTook = time.Since(startedAt)
		telemetry.OverBudget = telemetry.TotalTook > telemetry.Budget
		s.metrics.OnSearch(req, telemetry)
		return SearchResponse{}, err
	}
	telemetry.TotalHits = len(hits)

	aclStarted := time.Now()
	filtered := s.filterAuthorized(ctx, req, hits)
	telemetry.ACLTook = time.Since(aclStarted)
	telemetry.AuthorizedHits = len(filtered)
	rankStarted := time.Now()
	ranked := s.ranker.Rank(ctx, req, filtered)
	telemetry.RankTook = time.Since(rankStarted)
	groupStarted := time.Now()
	resp := s.grouper.Group(ctx, req, ranked)
	telemetry.GroupTook = time.Since(groupStarted)
	s.mu.RLock()
	agent := s.agent
	s.mu.RUnlock()
	if agent != nil {
		agentCtx, cancelAgent := withTimeoutRespectingDeadline(ctx, s.cfg.AgentTimeout)
		agentStarted := time.Now()
		agentAnswer, agentErr := agent.Answer(agentCtx, req, ranked)
		telemetry.AgentTook = time.Since(agentStarted)
		cancelAgent()
		if agentErr == nil {
			resp.Agent = agentAnswer
		} else if errors.Is(agentErr, ErrNoAgentAnswer) {
			// No-op: agent intentionally had no answer for this query.
		} else {
			s.logger.WithError(agentErr).WithFields(logrus.Fields{
				"tenant_id": req.TenantID.String(),
				"user_id":   req.UserID,
				"query":     req.Query,
			}).Warn("spotlight agent answer failed")
		}
	}
	s.setCachedSearch(req, resp)
	telemetry.TotalTook = time.Since(startedAt)
	telemetry.OverBudget = telemetry.TotalTook > telemetry.Budget
	if telemetry.OverBudget {
		s.logger.WithFields(logrus.Fields{
			"tenant_id":       req.TenantID.String(),
			"query":           req.Query,
			"budget_ms":       telemetry.Budget.Milliseconds(),
			"total_ms":        telemetry.TotalTook.Milliseconds(),
			"engine_ms":       telemetry.EngineTook.Milliseconds(),
			"acl_ms":          telemetry.ACLTook.Milliseconds(),
			"rank_ms":         telemetry.RankTook.Milliseconds(),
			"group_ms":        telemetry.GroupTook.Milliseconds(),
			"agent_ms":        telemetry.AgentTook.Milliseconds(),
			"cache_hit":       telemetry.CacheHit,
			"total_hits":      telemetry.TotalHits,
			"authorized_hits": telemetry.AuthorizedHits,
		}).Warn("spotlight search exceeded latency budget")
	}
	s.metrics.OnSearch(req, telemetry)
	return resp, nil
}

func (s *SpotlightService) scheduleIndexRefresh(tenantID uuid.UUID, language string) {
	if tenantID == uuid.Nil {
		return
	}
	key := tenantID.String() + ":" + language
	if _, loaded := s.enqueueOnce.LoadOrStore(key, struct{}{}); loaded {
		s.metrics.OnQueue(tenantID, language, false, len(s.indexQueue))
		return
	}
	select {
	case s.indexQueue <- indexTask{tenantID: tenantID, language: language}:
		s.metrics.OnQueue(tenantID, language, true, len(s.indexQueue))
	default:
		s.enqueueOnce.Delete(key)
		s.metrics.OnQueue(tenantID, language, false, len(s.indexQueue))
	}
}

func (s *SpotlightService) ensureProviderWatchers(tenantID uuid.UUID, language string) {
	if tenantID == uuid.Nil {
		return
	}
	s.lifecycleMu.Lock()
	bgCtx := s.bgCtx
	s.lifecycleMu.Unlock()
	if bgCtx == nil {
		return
	}
	for _, provider := range s.registry.All() {
		if !provider.Capabilities().SupportsWatch {
			continue
		}
		watchKey := provider.ProviderID() + ":" + tenantID.String() + ":" + language
		if _, loaded := s.watchStarted.LoadOrStore(watchKey, struct{}{}); loaded {
			continue
		}
		s.wg.Add(1)
		go func(ctx context.Context, p SearchProvider, t uuid.UUID, l, wk string) {
			defer s.wg.Done()
			s.runProviderWatch(ctx, p, t, l, wk)
		}(bgCtx, provider, tenantID, language, watchKey)
	}
}

func (s *SpotlightService) runProviderWatch(ctx context.Context, provider SearchProvider, tenantID uuid.UUID, language, watchKey string) {
	defer s.watchStarted.Delete(watchKey)
	changes, err := provider.Watch(ctx, ProviderScope{TenantID: tenantID, Language: language, TopK: 0})
	if err != nil {
		s.metrics.OnWatch(provider.ProviderID(), tenantID, "watch_start", err)
		s.logger.WithError(err).WithFields(logrus.Fields{
			"provider":  provider.ProviderID(),
			"tenant_id": tenantID.String(),
		}).Error("spotlight provider watch failed")
		return
	}
	s.metrics.OnWatch(provider.ProviderID(), tenantID, "watch_start", nil)
	if changes == nil {
		return
	}
	for {
		select {
		case <-ctx.Done():
			return
		case change, ok := <-changes:
			if !ok {
				return
			}
			s.applyDocumentEvent(ctx, tenantID, provider.ProviderID(), change)
			s.metrics.OnWatch(provider.ProviderID(), tenantID, string(change.Type), nil)
		}
	}
}

func (s *SpotlightService) applyDocumentEvent(ctx context.Context, tenantID uuid.UUID, providerID string, change DocumentEvent) {
	switch change.Type {
	case DocumentEventDelete:
		id := change.DocumentID
		if id == "" && change.Document != nil {
			id = change.Document.ID
		}
		if id == "" {
			return
		}
		if err := s.engine.Delete(ctx, []DocumentRef{{TenantID: tenantID, ID: id}}); err != nil {
			s.logger.WithError(err).WithFields(logrus.Fields{
				"provider":  providerID,
				"tenant_id": tenantID.String(),
				"doc_id":    id,
			}).Error("spotlight delete event failed")
			return
		}
		s.invalidateSearchCacheForTenant(tenantID)
	case DocumentEventCreate, DocumentEventUpdate:
		if change.Document == nil {
			return
		}
		doc := *change.Document
		if doc.TenantID == uuid.Nil {
			doc.TenantID = tenantID
		}
		if doc.Provider == "" {
			doc.Provider = providerID
		}
		if doc.Access.Visibility == "" {
			doc.Access.Visibility = VisibilityRestricted
		}
		if err := s.engine.Upsert(ctx, []SearchDocument{doc}); err != nil {
			s.logger.WithError(err).WithFields(logrus.Fields{
				"provider":  providerID,
				"tenant_id": tenantID.String(),
				"doc_id":    doc.ID,
			}).Error("spotlight upsert event failed")
			return
		}
		s.invalidateSearchCacheForTenant(tenantID)
	default:
		s.logger.WithFields(logrus.Fields{
			"provider":   providerID,
			"tenant_id":  tenantID.String(),
			"event_type": change.Type,
		}).Warn("spotlight unsupported document event type")
	}
}

func (s *SpotlightService) runBackgroundIndexer(ctx context.Context, tick time.Duration) {
	ticker := time.NewTicker(tick)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.pollOutbox(ctx)
			continue
		default:
		}

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.pollOutbox(ctx)
		case task := <-s.indexQueue:
			s.reindexTask(ctx, task)
		}
	}
}

func (s *SpotlightService) pollOutbox(ctx context.Context) {
	if s.outbox == nil {
		return
	}
	started := time.Now()
	if err := s.outbox.PollAndProcess(ctx); err != nil {
		s.metrics.OnOutboxPoll(time.Since(started), err)
		s.logger.WithError(err).Error("spotlight outbox poll failed")
		return
	}
	s.metrics.OnOutboxPoll(time.Since(started), nil)
}

func (s *SpotlightService) isStarted() bool {
	return s.startedAtomic.Load()
}

func (s *SpotlightService) filterAuthorized(ctx context.Context, req SearchRequest, hits []SearchHit) []SearchHit {
	if len(hits) == 0 {
		return []SearchHit{}
	}
	if acl, ok := s.acl.(BatchACLEvaluator); ok {
		return acl.FilterAuthorized(ctx, req, hits)
	}
	filtered := make([]SearchHit, 0, len(hits))
	for _, hit := range hits {
		if s.acl.CanRead(ctx, req, hit) {
			filtered = append(filtered, hit)
		}
	}
	return filtered
}

func withTimeoutRespectingDeadline(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if ctx == nil {
		return context.WithTimeout(context.Background(), timeout)
	}
	if deadline, ok := ctx.Deadline(); ok {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return context.WithCancel(ctx)
		}
		if remaining <= timeout {
			return context.WithCancel(ctx)
		}
	}
	return context.WithTimeout(ctx, timeout)
}

func dedupeAndSort(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	sort.Strings(out)
	return out
}

func (s *SpotlightService) getCachedSearch(req SearchRequest, allowStale bool) (SearchResponse, bool) {
	if s.cfg.SearchCacheTTL <= 0 || s.cfg.SearchCacheMaxEntries <= 0 {
		return SearchResponse{}, false
	}
	key := searchCacheKey(req)
	now := time.Now()
	s.cacheMu.RLock()
	entry, ok := s.searchCache[key]
	s.cacheMu.RUnlock()
	if !ok {
		return SearchResponse{}, false
	}
	if !allowStale && now.After(entry.expiresAt) {
		return SearchResponse{}, false
	}
	return entry.response, true
}

func (s *SpotlightService) setCachedSearch(req SearchRequest, resp SearchResponse) {
	if s.cfg.SearchCacheTTL <= 0 || s.cfg.SearchCacheMaxEntries <= 0 {
		return
	}
	key := searchCacheKey(req)
	now := time.Now()
	// O(n) eviction is intentional and bounded by SearchCacheMaxEntries (default: 512).
	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()
	if len(s.searchCache) >= s.cfg.SearchCacheMaxEntries {
		for cacheKey, entry := range s.searchCache {
			if now.After(entry.expiresAt) {
				delete(s.searchCache, cacheKey)
			}
		}
	}
	if len(s.searchCache) >= s.cfg.SearchCacheMaxEntries {
		var oldestKey string
		var oldestAt time.Time
		first := true
		for cacheKey, entry := range s.searchCache {
			if first || entry.storedAt.Before(oldestAt) {
				first = false
				oldestKey = cacheKey
				oldestAt = entry.storedAt
			}
		}
		if oldestKey != "" {
			delete(s.searchCache, oldestKey)
		}
	}
	s.searchCache[key] = cachedSearchResponse{
		response:  resp,
		expiresAt: now.Add(s.cfg.SearchCacheTTL),
		storedAt:  now,
	}
}

func (s *SpotlightService) invalidateSearchCacheForTenant(tenantID uuid.UUID) {
	if tenantID == uuid.Nil {
		return
	}
	prefix := tenantID.String() + searchCacheSeparator
	// O(n) tenant-key scan is bounded by SearchCacheMaxEntries (default: 512).
	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()
	for key := range s.searchCache {
		if strings.HasPrefix(key, prefix) {
			delete(s.searchCache, key)
		}
	}
}

func searchCacheKey(req SearchRequest) string {
	parts := make([]string, 0, 16)
	parts = append(parts,
		req.TenantID.String(),
		req.UserID,
		strings.ToLower(strings.TrimSpace(req.Query)),
		strings.ToLower(strings.TrimSpace(req.Language)),
		string(req.Intent),
		fmt.Sprintf("topk=%d", req.normalizedTopK()),
	)
	if len(req.Roles) > 0 {
		parts = append(parts, "roles="+strings.Join(dedupeAndSort(req.Roles), ","))
	}
	if len(req.Permissions) > 0 {
		parts = append(parts, "permissions="+strings.Join(dedupeAndSort(req.Permissions), ","))
	}
	if len(req.Filters) > 0 {
		keys := make([]string, 0, len(req.Filters))
		for key := range req.Filters {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			parts = append(parts, key+"="+req.Filters[key])
		}
	}
	return strings.Join(parts, searchCacheSeparator)
}

const searchCacheSeparator = "\x00"

func (s *SpotlightService) reindexTask(ctx context.Context, task indexTask) {
	defer s.enqueueOnce.Delete(task.tenantID.String() + ":" + task.language)
	if task.tenantID == uuid.Nil {
		return
	}
	started := time.Now()
	if err := s.ReindexTenant(ctx, task.tenantID, task.language); err != nil {
		s.metrics.OnReindex(task.tenantID, task.language, time.Since(started), err)
		s.logger.WithError(err).WithFields(logrus.Fields{
			"tenant_id": task.tenantID.String(),
			"language":  task.language,
		}).Error("spotlight indexing failed")
		return
	}
	s.invalidateSearchCacheForTenant(task.tenantID)
	s.metrics.OnReindex(task.tenantID, task.language, time.Since(started), nil)
}

// Package spotlight provides the core search, indexing, and streaming session
// primitives behind the Spotlight experience.
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
	lru "github.com/hashicorp/golang-lru/v2"
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
	defaultACLFanOutFactor     = 5
	defaultACLEngineMaxTopK    = 500
)

type ServiceConfig struct {
	SearchTimeout            time.Duration
	AgentTimeout             time.Duration
	SearchCacheTTL           time.Duration
	SearchCacheMaxEntries    int
	SearchLatencyBudget      time.Duration
	AllowStaleCacheOnTimeout bool
	BackgroundIndexerTick    time.Duration
	// ACLFanOutFactor controls over-fetching to absorb post-filter ACL drops.
	// Engine receives topK * factor candidates so the post-filter still has
	// enough rows to fill topK when many top hits get denied.
	ACLFanOutFactor int
	// ACLEngineMaxTopK is the absolute ceiling on the per-engine fetch size
	// after fan-out multiplication. Protects engine from runaway queries.
	ACLEngineMaxTopK int
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
		ACLFanOutFactor:          defaultACLFanOutFactor,
		ACLEngineMaxTopK:         defaultACLEngineMaxTopK,
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
	if cfg.ACLFanOutFactor <= 0 {
		cfg.ACLFanOutFactor = defaultACLFanOutFactor
	}
	if cfg.ACLEngineMaxTopK <= 0 {
		cfg.ACLEngineMaxTopK = defaultACLEngineMaxTopK
	}
	return cfg
}

// ProviderInfo describes a registered search provider.
type ProviderInfo struct {
	ID            string
	EntityTypes   []string
	IndexPriority int
	DocumentCount int64
}

// ServiceStats holds high-level service statistics.
type ServiceStats struct {
	Engine        IndexStats
	Providers     []ProviderInfo
	ProviderCount int
	Started       bool
}

type Service interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	RegisterProvider(provider SearchProvider)
	SetAgent(agent Agent)
	ReindexTenant(ctx context.Context, tenantID uuid.UUID, language string) error
	Readiness(ctx context.Context) error
	Search(ctx context.Context, req SearchRequest) (SearchResponse, error)
	CreateSession(ctx context.Context, req SearchRequest) (SearchSessionSnapshot, error)
	SubscribeSession(ctx context.Context, sessionID string, access SearchSessionAccess) (<-chan SearchSessionSnapshot, error)
	GetSessionSnapshot(sessionID string) (SearchSessionSnapshot, bool)
	CancelSession(sessionID string, access SearchSessionAccess)
	Stats(ctx context.Context) (*ServiceStats, error)
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

func WithQuickLinks(ql *QuickLinks) ServiceOption {
	return func(s *SpotlightService) {
		if ql != nil {
			s.quickLinks = ql
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

	outbox     OutboxProcessor
	quickLinks *QuickLinks

	// searchCache uses an LRU with TTL stored on the entry itself. The
	// hashicorp library handles eviction in O(1) and is thread-safe so
	// the previous cacheMu is no longer required. The hand-rolled
	// O(n) eviction (service.go:617 in the previous revision) was
	// hot-path under high QPS distinct queries; see #2810 §2.6.
	searchCache *lru.Cache[string, cachedSearchResponse]

	lifecycleMu   sync.Mutex
	started       bool
	startedAtomic atomic.Bool
	bgCtx         context.Context
	bgCancel      context.CancelFunc
	wg            sync.WaitGroup
	sessionsMu    sync.RWMutex
	sessions      map[string]*searchSession
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

	cache, err := lru.New[string, cachedSearchResponse](normalizedCfg.SearchCacheMaxEntries)
	if err != nil {
		// lru.New only errors on size <= 0; normalizedCfg already
		// guarantees a positive value, so this path is unreachable in
		// practice. Panic communicates the misconfiguration loudly
		// rather than silently disabling the cache.
		panic(fmt.Sprintf("spotlight: cannot init search cache: %v", err))
	}

	svc := &SpotlightService{
		cfg:         normalizedCfg,
		registry:    registry,
		scope:       scope,
		acl:         acl,
		engine:      engine,
		pipeline:    NewIndexerPipeline(registry, engine, nil),
		ranker:      NewDefaultRanker(),
		grouper:     NewDefaultGrouper(),
		metrics:     NewNoopMetrics(),
		logger:      logrus.StandardLogger(),
		agent:       agent,
		searchCache: cache,
		sessions:    make(map[string]*searchSession),
	}
	for _, opt := range opts {
		opt(svc)
	}
	svc.pipeline.logger = svc.logger
	return svc
}

func (s *SpotlightService) Start(_ context.Context) error {
	s.lifecycleMu.Lock()
	defer s.lifecycleMu.Unlock()
	if s.started {
		return nil
	}
	s.bgCtx, s.bgCancel = context.WithCancel(context.Background())
	bgCtx := s.bgCtx
	s.started = true
	s.startedAtomic.Store(true)
	s.wg.Add(1)
	go func(ctx context.Context) {
		defer s.wg.Done()
		s.runBackgroundIndexer(ctx, s.cfg.BackgroundIndexerTick)
	}(bgCtx)
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

	// Use rebuild session if engine supports it — builds a fresh index
	// and atomically swaps, avoiding slow delete-by-filter and writing
	// to a small (fast) index instead of a large (slow) one.
	rebuildable, ok := s.engine.(RebuildableIndexEngine)
	if !ok {
		if err := s.engine.DeleteTenant(ctx, tenantID); err != nil {
			return serrors.E(op, err)
		}
		return s.pipeline.Sync(ctx, tenantID, language, "", 0, scope)
	}

	rebuildStart := time.Now()
	session, err := rebuildable.StartRebuild(ctx)
	if err != nil {
		return serrors.E(op, err)
	}

	// Track commit success explicitly so the deferred Abort only fires on
	// abnormal exit. Previously Abort was inline-on-Sync-error only and a
	// panic between Sync and Commit (or a runaway Commit timeout) left
	// `spotlight_build_<schema>` orphaned. Issue #2810 §4.2 / §4.1.
	//
	// Abort uses a fresh background context so a cancelled / timed-out
	// request context cannot also block the cleanup that fixes the
	// orphan it was supposed to prevent.
	committed := false
	defer func() {
		if committed {
			return
		}
		abortCtx, cancelAbort := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancelAbort()
		if abortErr := session.Abort(abortCtx); abortErr != nil && s.logger != nil {
			s.logger.WithError(abortErr).
				WithField("tenant_id", tenantID.String()).
				Error("Spotlight build session abort failed")
		}
	}()

	buildPipeline := NewIndexerPipeline(s.registry, session.Engine(), s.logger)
	if err := buildPipeline.Sync(ctx, tenantID, language, "", 0, scope); err != nil {
		return serrors.E(op, err)
	}

	if err := session.Commit(ctx); err != nil {
		return serrors.E(op, err)
	}
	committed = true

	if s.logger != nil {
		s.logger.WithFields(logrus.Fields{
			"tenant_id":  tenantID.String(),
			"rebuild_ms": time.Since(rebuildStart).Milliseconds(),
		}).Info("Spotlight rebuild completed")
	}

	return nil
}

func (s *SpotlightService) Readiness(ctx context.Context) error {
	return s.engine.Health(ctx)
}

func (s *SpotlightService) Stats(ctx context.Context) (*ServiceStats, error) {
	const op serrors.Op = "spotlight.SpotlightService.Stats"

	engineStats, err := s.engine.Stats(ctx)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	providers := s.registry.All()
	infos := make([]ProviderInfo, 0, len(providers))
	for _, p := range providers {
		caps := p.Capabilities()
		infos = append(infos, ProviderInfo{
			ID:            p.ProviderID(),
			EntityTypes:   caps.EntityTypes,
			IndexPriority: caps.IndexPriority,
		})
	}

	if engineStats.ProviderDocumentCounts != nil {
		for i := range infos {
			if count, ok := engineStats.ProviderDocumentCounts[infos[i].ID]; ok {
				infos[i].DocumentCount = count
			}
		}
	}

	return &ServiceStats{
		Engine:        *engineStats,
		Providers:     infos,
		ProviderCount: len(infos),
		Started:       s.isStarted(),
	}, nil
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
	req = planRequest(req)
	req.Roles = dedupeAndSort(req.Roles)
	req.Permissions = dedupeAndSort(req.Permissions)

	if cached, ok := s.getCachedSearch(req, false); ok {
		telemetry.CacheHit = true
		telemetry.TotalTook = time.Since(startedAt)
		telemetry.OverBudget = telemetry.TotalTook > telemetry.Budget
		s.metrics.OnSearch(req, telemetry)
		return cached, nil
	}

	// Over-fetch from engine to absorb post-filter ACL drops (issue #2810
	// item 1.1). If we asked for exactly topK and the post-filter rejects
	// every candidate, the user sees an empty page even when matching docs
	// exist further down the ranking. Engine-side ACL via buildAccessFilter
	// already prunes most denials; this fan-out covers the long tail where
	// the application acl evaluator enforces something the doc fields don't.
	//
	// The hard ACLEngineMaxTopK applies even if the caller passed a TopK
	// larger than the cap; without the unconditional clamp below the
	// engine could receive a runaway value.
	originalTopK := req.TopK
	engineReq := req
	if originalTopK > 0 {
		fetch := originalTopK
		if s.cfg.ACLFanOutFactor > 1 {
			fetch = originalTopK * s.cfg.ACLFanOutFactor
		}
		if fetch > s.cfg.ACLEngineMaxTopK {
			fetch = s.cfg.ACLEngineMaxTopK
		}
		if fetch != originalTopK {
			engineReq.TopK = fetch
		}
	}

	searchCtx, cancelSearch := withTimeoutRespectingDeadline(ctx, s.cfg.SearchTimeout)
	engineStarted := time.Now()
	hits, err := s.engine.Search(searchCtx, engineReq)
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
	// Trim back to the caller-requested topK after the post-filter has had
	// a chance to consume the fan-out surplus.
	if originalTopK > 0 && len(filtered) > originalTopK {
		filtered = filtered[:originalTopK]
	}
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

func (s *SpotlightService) runBackgroundIndexer(ctx context.Context, tick time.Duration) {
	if ctx == nil {
		return
	}
	ticker := time.NewTicker(tick)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.pollOutbox(ctx)
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
	entry, ok := s.searchCache.Get(key)
	if !ok {
		return SearchResponse{}, false
	}
	if !allowStale && time.Now().After(entry.expiresAt) {
		// Expired entry is not removed eagerly; the LRU will reuse the
		// slot on next eviction. Removing here would require a write
		// lock for every miss, defeating the LRU's O(1) read path.
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
	// LRU eviction is O(1) and bounded by SearchCacheMaxEntries.
	// Hand-rolled scan + delete (the previous code) was a hot path under
	// high QPS distinct queries. Issue #2810 §2.6.
	s.searchCache.Add(key, cachedSearchResponse{
		response:  resp,
		expiresAt: now.Add(s.cfg.SearchCacheTTL),
		storedAt:  now,
	})
}

func searchCacheKey(req SearchRequest) string {
	parts := make([]string, 0, 16)
	// Cache key uses the caller's effective TopK (after defaulting 0→20).
	// We deliberately do NOT route through normalizedTopK() — that helper
	// clamps at 100 for engine safety, which would alias TopK=50, TopK=100,
	// and TopK=200 into the same key and serve a wrong-sized response on
	// cache hits once ACL fan-out raised the engine ceiling to 500.
	topK := req.TopK
	if topK <= 0 {
		topK = 20
	}
	parts = append(parts,
		req.TenantID.String(),
		req.UserID,
		strings.ToLower(strings.TrimSpace(req.Query)),
		strings.ToLower(strings.TrimSpace(req.Language)),
		string(req.Intent),
		string(req.Mode),
		fmt.Sprintf("topk=%d", topK),
	)
	if len(req.ExactTerms) > 0 {
		parts = append(parts, "exact="+strings.Join(ExpandExactTerms(req.ExactTerms...), ","))
	}
	if len(req.PreferredDomains) > 0 {
		domains := make([]string, 0, len(req.PreferredDomains))
		for _, domain := range req.PreferredDomains {
			domains = append(domains, string(domain))
		}
		sort.Strings(domains)
		parts = append(parts, "domains="+strings.Join(domains, ","))
	}
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

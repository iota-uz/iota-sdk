package spotlight

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
)

type Agent interface {
	Answer(ctx context.Context, req SearchRequest, hits []SearchHit) (*AgentAnswer, error)
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
		s.outbox = processor
	}
}

func WithPrincipalResolver(resolver PrincipalResolver) ServiceOption {
	return func(s *SpotlightService) {
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

type SpotlightService struct {
	registry *ProviderRegistry
	scope    ScopeStore
	acl      ACLEvaluator
	engine   IndexEngine
	pipeline *IndexerPipeline
	ranker   Ranker
	grouper  Grouper
	metrics  Metrics

	mu    sync.RWMutex
	agent Agent

	outbox OutboxProcessor

	indexQueue   chan indexTask
	enqueueOnce  sync.Map
	watchStarted sync.Map
	lifecycleMu  sync.Mutex
	started      bool
	bgCtx        context.Context
	bgCancel     context.CancelFunc
	wg           sync.WaitGroup
}

type indexTask struct {
	tenantID uuid.UUID
	language string
}

func NewService(engine IndexEngine, agent Agent, opts ...ServiceOption) *SpotlightService {
	registry := NewProviderRegistry()
	scope := NewInMemoryScopeStore()
	acl := NewStrictACLEvaluator(NewComposablesPrincipalResolver())

	svc := &SpotlightService{
		registry:   registry,
		scope:      scope,
		acl:        acl,
		engine:     engine,
		pipeline:   NewIndexerPipeline(registry, engine),
		ranker:     NewDefaultRanker(),
		grouper:    NewDefaultGrouper(),
		metrics:    NewNoopMetrics(),
		agent:      agent,
		indexQueue: make(chan indexTask, 256),
	}
	for _, opt := range opts {
		opt(svc)
	}
	return svc
}

func (s *SpotlightService) Start(ctx context.Context) error {
	s.lifecycleMu.Lock()
	defer s.lifecycleMu.Unlock()
	if s.started {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	s.bgCtx, s.bgCancel = context.WithCancel(ctx)
	s.started = true
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.runBackgroundIndexer(s.bgCtx, 5*time.Second)
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
	scope, err := s.scope.Resolve(ctx, SearchRequest{
		TenantID: tenantID,
		Language: language,
		Intent:   SearchIntentMixed,
	})
	if err != nil {
		return err
	}
	return s.pipeline.Sync(ctx, tenantID, language, "", 0, scope)
}

func (s *SpotlightService) Readiness(ctx context.Context) error {
	return s.engine.Health(ctx)
}

func (s *SpotlightService) Search(ctx context.Context, req SearchRequest) (SearchResponse, error) {
	_ = s.Start(context.Background())
	startedAt := time.Now()
	if req.TopK <= 0 {
		req.TopK = 20
	}
	if req.Intent == "" {
		req.Intent = SearchIntentMixed
	}

	s.scheduleIndexRefresh(req.TenantID, req.Language)
	s.ensureProviderWatchers(req.TenantID, req.Language)

	hits, err := s.engine.Search(ctx, req)
	if err != nil {
		s.metrics.OnSearch(req, 0, 0, time.Since(startedAt), err)
		return SearchResponse{}, err
	}

	filtered := make([]SearchHit, 0, len(hits))
	for _, hit := range hits {
		if s.acl.CanRead(ctx, req, hit) {
			filtered = append(filtered, hit)
		}
	}

	ranked := s.ranker.Rank(ctx, req, filtered)
	resp := s.grouper.Group(ctx, req, ranked)
	s.mu.RLock()
	agent := s.agent
	s.mu.RUnlock()
	if agent != nil {
		agentAnswer, agentErr := agent.Answer(ctx, req, ranked)
		if agentErr == nil {
			resp.Agent = agentAnswer
		}
	}
	s.metrics.OnSearch(req, len(hits), len(filtered), time.Since(startedAt), nil)
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
	for _, provider := range s.registry.All() {
		if !provider.Capabilities().SupportsWatch {
			continue
		}
		watchKey := provider.ProviderID() + ":" + tenantID.String() + ":" + language
		if _, loaded := s.watchStarted.LoadOrStore(watchKey, struct{}{}); loaded {
			continue
		}
		if s.bgCtx == nil {
			continue
		}
		s.wg.Add(1)
		go func(p SearchProvider, t uuid.UUID, l, wk string) {
			defer s.wg.Done()
			s.runProviderWatch(s.bgCtx, p, t, l, wk)
		}(provider, tenantID, language, watchKey)
	}
}

func (s *SpotlightService) runProviderWatch(ctx context.Context, provider SearchProvider, tenantID uuid.UUID, language, watchKey string) {
	defer s.watchStarted.Delete(watchKey)
	changes, err := provider.Watch(ctx, ProviderScope{TenantID: tenantID, Language: language, TopK: 0})
	if err != nil {
		s.metrics.OnWatch(provider.ProviderID(), tenantID, "watch_start", err)
		log.Printf("spotlight watch failed provider=%s tenant=%s err=%v", provider.ProviderID(), tenantID, err)
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
			log.Printf("spotlight delete event failed provider=%s tenant=%s id=%s err=%v", providerID, tenantID, id, err)
		}
	default:
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
			doc.Access.Visibility = VisibilityPublic
		}
		if err := s.engine.Upsert(ctx, []SearchDocument{doc}); err != nil {
			log.Printf("spotlight upsert event failed provider=%s tenant=%s id=%s err=%v", providerID, tenantID, doc.ID, err)
		}
	}
}

func (s *SpotlightService) runBackgroundIndexer(ctx context.Context, tick time.Duration) {
	ticker := time.NewTicker(tick)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case task := <-s.indexQueue:
			s.reindexTask(ctx, task)
		case <-ticker.C:
			if s.outbox != nil {
				started := time.Now()
				if err := s.outbox.PollAndProcess(ctx); err != nil {
					s.metrics.OnOutboxPoll(time.Since(started), err)
					log.Printf("spotlight outbox poll failed: %v", err)
				} else {
					s.metrics.OnOutboxPoll(time.Since(started), nil)
				}
			}
		}
	}
}

func (s *SpotlightService) reindexTask(ctx context.Context, task indexTask) {
	defer s.enqueueOnce.Delete(task.tenantID.String() + ":" + task.language)
	if task.tenantID == uuid.Nil {
		return
	}
	started := time.Now()
	if err := s.ReindexTenant(ctx, task.tenantID, task.language); err != nil {
		s.metrics.OnReindex(task.tenantID, task.language, time.Since(started), err)
		log.Printf("spotlight indexing failed tenant=%s: %v", task.tenantID, err)
		return
	}
	s.metrics.OnReindex(task.tenantID, task.language, time.Since(started), nil)
}

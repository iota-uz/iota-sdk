// Package spotlight manages multi-stage Spotlight search sessions, including
// lifecycle management, snapshots, and subscriber updates.
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
)

var ErrSessionNotFound = errors.New("spotlight session not found")

const (
	defaultSessionInitialWait = 120 * time.Millisecond
	defaultSessionTTL         = 30 * time.Second
	defaultFastStageTimeout   = 90 * time.Millisecond
	defaultIndexedStageTime   = 220 * time.Millisecond
	defaultExpandStageTime    = 600 * time.Millisecond
	defaultExpandProviderTopK = 8
	defaultExpandConcurrency  = 4
)

type searchSession struct {
	id string

	tenantID uuid.UUID
	userID   string

	mu          sync.RWMutex
	snapshot    SearchSessionSnapshot
	subscribers map[chan SearchSessionSnapshot]struct{}
	cancel      context.CancelFunc
	ready       chan struct{}
	readyOnce   sync.Once
}

func newSearchSession(id, query string, tenantID uuid.UUID, userID string, cancel context.CancelFunc) *searchSession {
	return &searchSession{
		id:          id,
		tenantID:    tenantID,
		userID:      userID,
		subscribers: make(map[chan SearchSessionSnapshot]struct{}),
		cancel:      cancel,
		ready:       make(chan struct{}),
		snapshot: SearchSessionSnapshot{
			ID:        id,
			Query:     query,
			Stages:    defaultStageStates(),
			Loading:   true,
			Completed: false,
			UpdatedAt: time.Now().UTC(),
		},
	}
}

func defaultStageStates() []SearchStageState {
	return []SearchStageState{
		{
			Stage:          SearchStageFast,
			Status:         SearchStageStatusPending,
			TotalSources:   1,
			PendingSources: 1,
		},
		{
			Stage:          SearchStageIndexed,
			Status:         SearchStageStatusPending,
			TotalSources:   1,
			PendingSources: 1,
		},
		{
			Stage:          SearchStageExpand,
			Status:         SearchStageStatusPending,
			TotalSources:   0,
			PendingSources: 0,
		},
	}
}

func (s *searchSession) snapshotValue() SearchSessionSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return cloneSnapshot(s.snapshot)
}

func (s *searchSession) update(mutator func(*SearchSessionSnapshot)) SearchSessionSnapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	mutator(&s.snapshot)
	s.snapshot.Version++
	s.snapshot.UpdatedAt = time.Now().UTC()
	snapshot := cloneSnapshot(s.snapshot)
	if len(snapshot.Response.Groups) > 0 || snapshot.Completed {
		s.readyOnce.Do(func() {
			close(s.ready)
		})
	}
	for ch := range s.subscribers {
		deliverSnapshot(ch, snapshot)
	}
	return snapshot
}

func (s *searchSession) subscribe(ctx context.Context) (<-chan SearchSessionSnapshot, func()) {
	ch := make(chan SearchSessionSnapshot, 8)
	s.mu.Lock()
	s.subscribers[ch] = struct{}{}
	current := cloneSnapshot(s.snapshot)
	s.mu.Unlock()
	deliverSnapshot(ch, current)
	cleanup := func() {
		s.mu.Lock()
		if _, exists := s.subscribers[ch]; exists {
			delete(s.subscribers, ch)
			close(ch)
		}
		s.mu.Unlock()
	}
	go func() {
		<-ctx.Done()
		cleanup()
	}()
	return ch, cleanup
}

func deliverSnapshot(ch chan SearchSessionSnapshot, snapshot SearchSessionSnapshot) {
	select {
	case ch <- snapshot:
	default:
		select {
		case <-ch:
		default:
		}
		select {
		case ch <- snapshot:
		default:
		}
	}
}

func cloneSnapshot(snapshot SearchSessionSnapshot) SearchSessionSnapshot {
	cloned := snapshot
	cloned.Stages = append([]SearchStageState(nil), snapshot.Stages...)
	cloned.Response = cloneResponse(snapshot.Response)
	return cloned
}

func cloneResponse(resp SearchResponse) SearchResponse {
	cloned := resp
	cloned.Groups = append([]SearchGroup(nil), resp.Groups...)
	for i := range cloned.Groups {
		cloned.Groups[i].Hits = append([]SearchHit(nil), resp.Groups[i].Hits...)
	}
	return cloned
}

func (s *SpotlightService) CreateSession(ctx context.Context, req SearchRequest) (SearchSessionSnapshot, error) {
	const op serrors.Op = "spotlight.SpotlightService.CreateSession"
	if ctx == nil {
		ctx = context.Background()
	}
	if !s.isStarted() {
		if err := s.Start(ctx); err != nil {
			return SearchSessionSnapshot{}, serrors.E(op, err)
		}
	}
	req.Query = strings.TrimSpace(req.Query)
	if req.Query == "" {
		return SearchSessionSnapshot{}, serrors.E(op, errors.New("search query is required"))
	}
	if req.TopK <= 0 {
		req.TopK = 30
	}
	sessionID := uuid.NewString()
	sessionCtx, cancel := context.WithTimeout(context.Background(), defaultSessionTTL)
	session := newSearchSession(sessionID, req.Query, req.TenantID, req.UserID, cancel)

	s.sessionsMu.Lock()
	s.sessions[sessionID] = session
	s.sessionsMu.Unlock()

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		time.AfterFunc(defaultSessionTTL, func() {
			s.deleteSession(sessionID)
		})
		s.runSearchSession(sessionCtx, session, req)
	}()

	waitCtx, waitCancel := context.WithTimeout(ctx, defaultSessionInitialWait)
	defer waitCancel()
	select {
	case <-session.ready:
	case <-waitCtx.Done():
	}
	return session.snapshotValue(), nil
}

func (s *SpotlightService) SubscribeSession(ctx context.Context, sessionID string, access SearchSessionAccess) (<-chan SearchSessionSnapshot, error) {
	session, ok := s.getSession(sessionID)
	if !ok {
		return nil, ErrSessionNotFound
	}
	if !session.allows(access) {
		return nil, ErrSessionNotFound
	}
	if ctx == nil {
		ctx = context.Background()
	}
	ch, cleanup := session.subscribe(ctx)
	if current, exists := s.getSession(sessionID); !exists || current != session || !current.allows(access) {
		cleanup()
		return nil, ErrSessionNotFound
	}
	return ch, nil
}

func (s *SpotlightService) GetSessionSnapshot(sessionID string) (SearchSessionSnapshot, bool) {
	session, ok := s.getSession(sessionID)
	if !ok {
		return SearchSessionSnapshot{}, false
	}
	return session.snapshotValue(), true
}

func (s *SpotlightService) CancelSession(sessionID string, access SearchSessionAccess) {
	session, ok := s.getSession(sessionID)
	if !ok {
		return
	}
	if !session.allows(access) {
		return
	}
	session.cancel()
	s.deleteSession(sessionID)
}

func (s *SpotlightService) getSession(sessionID string) (*searchSession, bool) {
	s.sessionsMu.RLock()
	defer s.sessionsMu.RUnlock()
	session, ok := s.sessions[sessionID]
	return session, ok
}

func (s *SpotlightService) deleteSession(sessionID string) {
	s.sessionsMu.Lock()
	session, ok := s.sessions[sessionID]
	if ok {
		delete(s.sessions, sessionID)
	}
	s.sessionsMu.Unlock()
	if !ok {
		return
	}
	session.mu.Lock()
	for ch := range session.subscribers {
		close(ch)
	}
	session.subscribers = map[chan SearchSessionSnapshot]struct{}{}
	session.mu.Unlock()
}

func (s *searchSession) allows(access SearchSessionAccess) bool {
	if s.tenantID != access.TenantID {
		return false
	}
	return s.userID == access.UserID
}

func (s *SpotlightService) runSearchSession(ctx context.Context, session *searchSession, req SearchRequest) {
	planned := planRequest(req)
	planned.Roles = dedupeAndSort(planned.Roles)
	planned.Permissions = dedupeAndSort(planned.Permissions)
	planned.Query = strings.TrimSpace(planned.Query)

	hitState := newSessionHitState()

	// In-memory fuzzy search for quick links (instant, no network hop)
	if s.quickLinks != nil {
		qlHits := s.quickLinks.FuzzySearch(planned.Query, planned)
		if len(qlHits) > 0 {
			merged := hitState.merge(planned, qlHits)
			session.update(func(snapshot *SearchSessionSnapshot) {
				snapshot.Response = s.grouper.Group(ctx, planned, merged)
			})
		}
	}

	if err := s.executeSearchStage(ctx, session, hitState, planned, SearchStageFast, sessionFastRequest(planned), nil); err != nil && ctx.Err() == nil {
		s.logger.WithError(err).WithField("search_id", session.id).Warn("spotlight fast stage failed")
	}
	if ctx.Err() != nil {
		return
	}

	if err := s.executeSearchStage(ctx, session, hitState, planned, SearchStageIndexed, sessionIndexedRequest(planned), nil); err != nil && ctx.Err() == nil {
		s.logger.WithError(err).WithField("search_id", session.id).Warn("spotlight indexed stage failed")
	}
	if ctx.Err() != nil {
		return
	}

	s.executeExpandStage(ctx, session, hitState, planned)
	if ctx.Err() != nil {
		return
	}

	session.update(func(snapshot *SearchSessionSnapshot) {
		snapshot.Loading = false
		snapshot.Completed = true
		for i := range snapshot.Stages {
			if snapshot.Stages[i].Status == SearchStageStatusRunning || snapshot.Stages[i].Status == SearchStageStatusPending {
				snapshot.Stages[i].Status = SearchStageStatusCompleted
				snapshot.Stages[i].PendingSources = 0
			}
		}
	})
}

func (s *SpotlightService) executeSearchStage(
	ctx context.Context,
	session *searchSession,
	state *sessionHitState,
	baseReq SearchRequest,
	stage SearchStage,
	stageReq SearchRequest,
	progress func(*SearchStageState),
) error {
	session.update(func(snapshot *SearchSessionSnapshot) {
		markStageRunning(snapshot, stage, 1)
		if progress != nil {
			stageState := stageRef(snapshot, stage)
			progress(stageState)
		}
	})

	stageTimeout := stageTimeoutFor(stage)
	stageCtx, cancel := withTimeoutRespectingDeadline(ctx, stageTimeout)
	defer cancel()

	resp, hits, err := s.searchStage(stageCtx, stageReq)
	if err != nil {
		session.update(func(snapshot *SearchSessionSnapshot) {
			stageState := stageRef(snapshot, stage)
			stageState.Status = SearchStageStatusFailed
			stageState.PendingSources = 0
			stageState.CompletedSources = stageState.TotalSources
			stageState.Error = err.Error()
			snapshot.Loading = !allStagesTerminal(snapshot.Stages)
			snapshot.Completed = !snapshot.Loading
		})
		return err
	}

	merged := state.merge(baseReq, hits)
	session.update(func(snapshot *SearchSessionSnapshot) {
		stageState := stageRef(snapshot, stage)
		stageState.Status = SearchStageStatusCompleted
		stageState.PendingSources = 0
		stageState.CompletedSources = stageState.TotalSources
		stageState.ResultCount = len(hits)
		snapshot.Response = s.grouper.Group(ctx, baseReq, merged)
		if len(resp.Groups) > 0 && len(snapshot.Response.Groups) == 0 {
			snapshot.Response = resp
		}
		snapshot.Loading = !allStagesTerminal(snapshot.Stages)
		snapshot.Completed = !snapshot.Loading
	})
	return nil
}

func (s *SpotlightService) executeExpandStage(ctx context.Context, session *searchSession, state *sessionHitState, baseReq SearchRequest) {
	providers := s.registry.All()
	if len(providers) == 0 {
		session.update(func(snapshot *SearchSessionSnapshot) {
			stageState := stageRef(snapshot, SearchStageExpand)
			stageState.Status = SearchStageStatusSkipped
			stageState.TotalSources = 0
			stageState.CompletedSources = 0
			stageState.PendingSources = 0
			snapshot.Loading = !allStagesTerminal(snapshot.Stages)
			snapshot.Completed = !snapshot.Loading
		})
		return
	}

	session.update(func(snapshot *SearchSessionSnapshot) {
		stageState := stageRef(snapshot, SearchStageExpand)
		stageState.Status = SearchStageStatusRunning
		stageState.TotalSources = len(providers)
		stageState.CompletedSources = 0
		stageState.PendingSources = len(providers)
		stageState.ResultCount = 0
		stageState.Error = ""
	})

	expandCtx, cancel := withTimeoutRespectingDeadline(ctx, defaultExpandStageTime)
	defer cancel()

	var completed atomic.Int64
	var resultCount atomic.Int64
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, defaultExpandConcurrency)

	for _, provider := range providers {
		wg.Add(1)
		go func(provider SearchProvider) {
			defer wg.Done()
			select {
			case semaphore <- struct{}{}:
			case <-expandCtx.Done():
				return
			}
			defer func() { <-semaphore }()

			stageReq := sessionProviderRequest(baseReq, provider.ProviderID())
			_, hits, err := s.searchStage(expandCtx, stageReq)
			done := int(completed.Add(1))
			if err == nil && len(hits) > 0 {
				merged := state.merge(baseReq, hits)
				resultCount.Add(int64(len(hits)))
				session.update(func(snapshot *SearchSessionSnapshot) {
					stageState := stageRef(snapshot, SearchStageExpand)
					stageState.CompletedSources = done
					stageState.PendingSources = max(0, stageState.TotalSources-done)
					stageState.ResultCount = int(resultCount.Load())
					snapshot.Response = s.grouper.Group(ctx, baseReq, merged)
				})
				return
			}
			session.update(func(snapshot *SearchSessionSnapshot) {
				stageState := stageRef(snapshot, SearchStageExpand)
				stageState.CompletedSources = done
				stageState.PendingSources = max(0, stageState.TotalSources-done)
				if err != nil && stageState.Error == "" {
					stageState.Error = fmt.Sprintf("%s: %v", provider.ProviderID(), err)
				}
			})
		}(provider)
	}
	wg.Wait()

	session.update(func(snapshot *SearchSessionSnapshot) {
		stageState := stageRef(snapshot, SearchStageExpand)
		if stageState.Status != SearchStageStatusFailed {
			stageState.Status = SearchStageStatusCompleted
		}
		stageState.CompletedSources = stageState.TotalSources
		stageState.PendingSources = 0
		stageState.ResultCount = int(resultCount.Load())
		snapshot.Loading = !allStagesTerminal(snapshot.Stages)
		snapshot.Completed = !snapshot.Loading
	})
}

func (s *SpotlightService) searchStage(ctx context.Context, req SearchRequest) (SearchResponse, []SearchHit, error) {
	const op serrors.Op = "spotlight.SpotlightService.searchStage"
	if req.TopK <= 0 {
		req.TopK = 20
	}
	req = planRequest(req)
	req.Roles = dedupeAndSort(req.Roles)
	req.Permissions = dedupeAndSort(req.Permissions)

	hits, err := s.engine.Search(ctx, req)
	if err != nil {
		return SearchResponse{}, nil, serrors.E(op, err)
	}
	filtered := s.filterAuthorized(ctx, req, hits)
	ranked := s.ranker.Rank(ctx, req, filtered)
	resp := s.grouper.Group(ctx, req, ranked)
	return resp, ranked, nil
}

type sessionHitState struct {
	mu   sync.Mutex
	hits map[string]SearchHit
}

func newSessionHitState() *sessionHitState {
	return &sessionHitState{hits: make(map[string]SearchHit)}
}

func (s *sessionHitState) merge(req SearchRequest, incoming []SearchHit) []SearchHit {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, hit := range incoming {
		existing, exists := s.hits[hit.Document.ID]
		if !exists || hit.FinalScore >= existing.FinalScore {
			s.hits[hit.Document.ID] = hit
			continue
		}
		if strings.TrimSpace(existing.Document.Description) == "" && strings.TrimSpace(hit.Document.Description) != "" {
			existing.Document.Description = hit.Document.Description
			existing.Document.Body = hit.Document.Body
			existing.Document.SearchText = hit.Document.SearchText
			s.hits[hit.Document.ID] = existing
		}
	}
	out := make([]SearchHit, 0, len(s.hits))
	for _, hit := range s.hits {
		out = append(out, hit)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].FinalScore == out[j].FinalScore {
			return out[i].Document.Title < out[j].Document.Title
		}
		return out[i].FinalScore > out[j].FinalScore
	})
	limit := req.normalizedTopK()
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}

func sessionFastRequest(req SearchRequest) SearchRequest {
	fast := req
	fast.TopK = min(8, req.normalizedTopK())
	switch req.Mode {
	case QueryModeLookup:
		fast.Mode = QueryModeLookup
	case QueryModeNavigate:
		fast.Mode = QueryModeNavigate
	case QueryModeExplore, QueryModeHelp:
		if len(req.ExactTerms) > 0 {
			fast.Mode = QueryModeLookup
		}
	default:
	}
	return fast
}

func sessionIndexedRequest(req SearchRequest) SearchRequest {
	indexed := req
	indexed.TopK = req.normalizedTopK()
	return indexed
}

func sessionProviderRequest(req SearchRequest, providerID string) SearchRequest {
	providerReq := req
	providerReq.TopK = min(defaultExpandProviderTopK, req.normalizedTopK())
	if providerReq.Filters == nil {
		providerReq.Filters = make(map[string]string, 1)
	} else {
		copied := make(map[string]string, len(providerReq.Filters)+1)
		for key, value := range providerReq.Filters {
			copied[key] = value
		}
		providerReq.Filters = copied
	}
	providerReq.Filters["provider"] = providerID
	return providerReq
}

func stageTimeoutFor(stage SearchStage) time.Duration {
	switch stage {
	case SearchStageFast:
		return defaultFastStageTimeout
	case SearchStageIndexed:
		return defaultIndexedStageTime
	case SearchStageExpand:
		return defaultExpandStageTime
	}
	return defaultExpandStageTime
}

func stageRef(snapshot *SearchSessionSnapshot, stage SearchStage) *SearchStageState {
	for i := range snapshot.Stages {
		if snapshot.Stages[i].Stage == stage {
			return &snapshot.Stages[i]
		}
	}
	snapshot.Stages = append(snapshot.Stages, SearchStageState{Stage: stage})
	return &snapshot.Stages[len(snapshot.Stages)-1]
}

func markStageRunning(snapshot *SearchSessionSnapshot, stage SearchStage, totalSources int) {
	stageState := stageRef(snapshot, stage)
	stageState.Status = SearchStageStatusRunning
	stageState.TotalSources = totalSources
	stageState.CompletedSources = 0
	stageState.PendingSources = totalSources
	stageState.ResultCount = 0
	stageState.Error = ""
}

func allStagesTerminal(stages []SearchStageState) bool {
	for _, stage := range stages {
		switch stage.Status {
		case SearchStageStatusCompleted, SearchStageStatusFailed, SearchStageStatusSkipped:
		case SearchStageStatusPending, SearchStageStatusRunning:
			return false
		default:
			return false
		}
	}
	return true
}

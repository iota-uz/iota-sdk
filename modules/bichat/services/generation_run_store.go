// Package services provides this package.
package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/bichat/domain"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
	"github.com/redis/go-redis/v9"
)

const (
	defaultGenerationRunStorePrefix = "bichat:generation-runs"
	defaultGenerationRunStoreTTL    = 2 * time.Hour
)

type generationRunStore interface {
	CreateRun(ctx context.Context, run domain.GenerationRun) error
	GetActiveRunBySession(ctx context.Context, tenantID uuid.UUID, sessionID uuid.UUID) (domain.GenerationRun, error)
	GetRunByID(ctx context.Context, tenantID uuid.UUID, runID uuid.UUID) (domain.GenerationRun, error)
	UpdateRunSnapshot(ctx context.Context, tenantID, sessionID, runID uuid.UUID, partialContent string, partialMetadata map[string]any) error
	CompleteRun(ctx context.Context, tenantID, sessionID, runID uuid.UUID) error
	CancelRun(ctx context.Context, tenantID, sessionID, runID uuid.UUID) error
	// FailRun is the system-initiated terminal transition. Used by workers
	// on unrecoverable errors and by the reaper on stale heartbeats.
	FailRun(ctx context.Context, tenantID, sessionID, runID uuid.UUID) error
	// RequestCancel flips the cancel flag on an active run without moving
	// it to a terminal status. The owning worker observes the flag on its
	// next heartbeat/snapshot tick and drives the actual CancelRun call.
	RequestCancel(ctx context.Context, tenantID, sessionID, runID uuid.UUID) error
	// Heartbeat refreshes LastHeartbeatAt so the reaper knows the run is
	// still progressing. Idempotent; no-op on terminal states.
	Heartbeat(ctx context.Context, tenantID, sessionID, runID uuid.UUID) error
}

type redisGenerationRunStoreConfig struct {
	RedisURL  string
	KeyPrefix string
	TTL       time.Duration
	Client    *redis.Client
}

type redisGenerationRunStore struct {
	client    *redis.Client
	keyPrefix string
	ttl       time.Duration
}

type persistedGenerationRun struct {
	ID             string         `json:"id"`
	SessionID      string         `json:"session_id"`
	TenantID       string         `json:"tenant_id"`
	UserID         int64          `json:"user_id"`
	Status         string         `json:"status"`
	PartialContent string         `json:"partial_content"`
	PartialMeta    map[string]any `json:"partial_metadata"`
	StartedAt      time.Time      `json:"started_at"`
	LastUpdatedAt  time.Time      `json:"last_updated_at"`
	// CancelRequested is flipped by Stop RPCs; the worker polls it.
	CancelRequested bool `json:"cancel_requested,omitempty"`
	// LastHeartbeatAt is refreshed by the worker; the reaper fails runs
	// whose heartbeat has gone stale. Zero-value means "never heartbeated"
	// (e.g. a queued-but-not-started run).
	LastHeartbeatAt time.Time `json:"last_heartbeat_at,omitempty"`
}

func newConfiguredGenerationRunStore() generationRunStore {
	redisURL, ok := os.LookupEnv("REDIS_URL")
	if !ok || strings.TrimSpace(redisURL) == "" {
		log.Printf("bichat generation run store disabled: REDIS_URL is missing or empty")
		return nil
	}

	store, err := newRedisGenerationRunStore(redisGenerationRunStoreConfig{
		RedisURL: redisURL,
	})
	if err != nil {
		log.Printf("bichat generation run store disabled: failed to initialize redis store: %v", err)
		return nil
	}

	return store
}

func newRedisGenerationRunStore(cfg redisGenerationRunStoreConfig) (generationRunStore, error) {
	prefix := strings.TrimSpace(cfg.KeyPrefix)
	if prefix == "" {
		prefix = defaultGenerationRunStorePrefix
	}

	ttl := cfg.TTL
	if ttl <= 0 {
		ttl = defaultGenerationRunStoreTTL
	}

	client := cfg.Client
	if client == nil {
		c, err := newRedisClient(cfg.RedisURL)
		if err != nil {
			return nil, err
		}
		client = c
	}

	return &redisGenerationRunStore{
		client:    client,
		keyPrefix: prefix,
		ttl:       ttl,
	}, nil
}

func (s *redisGenerationRunStore) CreateRun(ctx context.Context, run domain.GenerationRun) error {
	const op serrors.Op = "redisGenerationRunStore.CreateRun"

	if run == nil {
		return serrors.E(op, serrors.KindValidation, "run is required")
	}
	if run.TenantID() == uuid.Nil {
		return serrors.E(op, serrors.KindValidation, "tenant id is required")
	}
	if run.SessionID() == uuid.Nil {
		return serrors.E(op, serrors.KindValidation, "session id is required")
	}
	if run.ID() == uuid.Nil {
		return serrors.E(op, serrors.KindValidation, "run id is required")
	}

	record := persistedGenerationRun{
		ID:              run.ID().String(),
		SessionID:       run.SessionID().String(),
		TenantID:        run.TenantID().String(),
		UserID:          run.UserID(),
		Status:          string(domain.GenerationRunStatusStreaming),
		PartialContent:  run.PartialContent(),
		PartialMeta:     cloneMetadata(run.PartialMetadata()),
		StartedAt:       run.StartedAt().UTC(),
		LastUpdatedAt:   run.LastUpdatedAt().UTC(),
		CancelRequested: run.CancelRequested(),
		LastHeartbeatAt: run.LastHeartbeatAt().UTC(),
	}
	if record.StartedAt.IsZero() {
		record.StartedAt = time.Now().UTC()
	}
	if record.LastUpdatedAt.IsZero() {
		record.LastUpdatedAt = record.StartedAt
	}

	payload, err := json.Marshal(record)
	if err != nil {
		return serrors.E(op, "marshal run state", err)
	}

	sessionKey := s.sessionKey(run.TenantID(), run.SessionID())
	runKey := s.runKey(run.TenantID(), run.ID())

	created, err := s.client.SetNX(ctx, sessionKey, payload, s.ttl).Result()
	if err != nil {
		return serrors.E(op, "create run state", err)
	}
	if !created {
		return domain.ErrActiveRunExists
	}
	if err := s.client.Set(ctx, runKey, payload, s.ttl).Err(); err != nil {
		if _, rollbackErr := s.client.Del(ctx, sessionKey).Result(); rollbackErr != nil {
			return serrors.E(op, fmt.Errorf(
				"create run index via s.client.Set(ctx, %q) failed: %w; rollback s.client.Del(%q) failed: %v",
				runKey,
				err,
				sessionKey,
				rollbackErr,
			))
		}
		return serrors.E(op, "create run index", err)
	}
	return nil
}

func (s *redisGenerationRunStore) GetActiveRunBySession(ctx context.Context, tenantID uuid.UUID, sessionID uuid.UUID) (domain.GenerationRun, error) {
	const op serrors.Op = "redisGenerationRunStore.GetActiveRunBySession"

	record, found, err := s.loadRun(ctx, tenantID, sessionID)
	if err != nil {
		return nil, serrors.E(op, err)
	}
	if !found || record.Status != string(domain.GenerationRunStatusStreaming) {
		return nil, domain.ErrNoActiveRun
	}

	run, err := mapPersistedGenerationRunToDomain(record)
	if err != nil {
		return nil, serrors.E(op, "convert persisted run", err)
	}
	return run, nil
}

func (s *redisGenerationRunStore) GetRunByID(ctx context.Context, tenantID uuid.UUID, runID uuid.UUID) (domain.GenerationRun, error) {
	const op serrors.Op = "redisGenerationRunStore.GetRunByID"

	raw, err := s.client.Get(ctx, s.runKey(tenantID, runID)).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, domain.ErrRunNotFound
		}
		return nil, serrors.E(op, "get run by id", err)
	}

	var record persistedGenerationRun
	if err := json.Unmarshal(raw, &record); err != nil {
		return nil, serrors.E(op, "unmarshal run", err)
	}
	if record.PartialMeta == nil {
		record.PartialMeta = make(map[string]any)
	}

	run, err := mapPersistedGenerationRunToDomain(record)
	if err != nil {
		return nil, serrors.E(op, "convert persisted run", err)
	}
	return run, nil
}

// UpdateRunSnapshot performs a loadRun -> mutate -> saveRun cycle.
// This non-atomic flow is safe under the single-writer assumption: one
// runStreamLoop goroutine owns writes for a given session/run. Callers must not
// issue concurrent snapshot writes for the same session.
func (s *redisGenerationRunStore) UpdateRunSnapshot(ctx context.Context, tenantID, sessionID, runID uuid.UUID, partialContent string, partialMetadata map[string]any) error {
	const op serrors.Op = "redisGenerationRunStore.UpdateRunSnapshot"

	record, found, err := s.loadRun(ctx, tenantID, sessionID)
	if err != nil {
		return serrors.E(op, err)
	}
	if !found {
		return domain.ErrNoActiveRun
	}
	if record.ID != runID.String() || record.Status != string(domain.GenerationRunStatusStreaming) {
		return domain.ErrNoActiveRun
	}

	record.PartialContent = partialContent
	record.PartialMeta = cloneMetadata(partialMetadata)
	record.LastUpdatedAt = time.Now().UTC()

	if err := s.saveRun(ctx, tenantID, sessionID, record); err != nil {
		return serrors.E(op, "save run state", err)
	}
	return nil
}

func (s *redisGenerationRunStore) CompleteRun(ctx context.Context, tenantID, sessionID, runID uuid.UUID) error {
	return s.finishRun(ctx, tenantID, sessionID, runID, domain.GenerationRunStatusCompleted)
}

func (s *redisGenerationRunStore) CancelRun(ctx context.Context, tenantID, sessionID, runID uuid.UUID) error {
	return s.finishRun(ctx, tenantID, sessionID, runID, domain.GenerationRunStatusCancelled)
}

func (s *redisGenerationRunStore) FailRun(ctx context.Context, tenantID, sessionID, runID uuid.UUID) error {
	return s.finishRun(ctx, tenantID, sessionID, runID, domain.GenerationRunStatusFailed)
}

// RequestCancel flips the cancel flag on the active run and refreshes
// LastUpdatedAt. It is a no-op if the session has no active run or if the
// active run's id doesn't match runID — this keeps repeated Stop RPCs
// idempotent without leaking state.
func (s *redisGenerationRunStore) RequestCancel(ctx context.Context, tenantID, sessionID, runID uuid.UUID) error {
	const op serrors.Op = "redisGenerationRunStore.RequestCancel"

	record, found, err := s.loadRun(ctx, tenantID, sessionID)
	if err != nil {
		return serrors.E(op, err)
	}
	if !found {
		return nil
	}
	if record.ID != runID.String() {
		return nil
	}
	if record.Status != string(domain.GenerationRunStatusStreaming) {
		return nil
	}
	if record.CancelRequested {
		// Already requested; nothing to persist.
		return nil
	}
	record.CancelRequested = true
	record.LastUpdatedAt = time.Now().UTC()
	if err := s.saveRun(ctx, tenantID, sessionID, record); err != nil {
		return serrors.E(op, "persist cancel request", err)
	}
	return nil
}

// Heartbeat refreshes LastHeartbeatAt + LastUpdatedAt. Called from the
// worker on every streaming iteration so the reaper can detect wedged
// runs. The operation is racy-safe under the single-writer assumption
// (one worker owns a given run at a time), same as UpdateRunSnapshot.
// Idempotent no-op when the run is missing or not in streaming status;
// callers must not treat those cases as errors.
func (s *redisGenerationRunStore) Heartbeat(ctx context.Context, tenantID, sessionID, runID uuid.UUID) error {
	const op serrors.Op = "redisGenerationRunStore.Heartbeat"

	record, found, err := s.loadRun(ctx, tenantID, sessionID)
	if err != nil {
		return serrors.E(op, err)
	}
	if !found {
		return nil
	}
	if record.ID != runID.String() || record.Status != string(domain.GenerationRunStatusStreaming) {
		return nil
	}
	now := time.Now().UTC()
	record.LastHeartbeatAt = now
	record.LastUpdatedAt = now
	if err := s.saveRun(ctx, tenantID, sessionID, record); err != nil {
		return serrors.E(op, "persist heartbeat", err)
	}
	return nil
}

func (s *redisGenerationRunStore) finishRun(ctx context.Context, tenantID, sessionID, runID uuid.UUID, status domain.GenerationRunStatus) error {
	const op serrors.Op = "redisGenerationRunStore.finishRun"

	record, found, err := s.loadRun(ctx, tenantID, sessionID)
	if err != nil {
		return serrors.E(op, err)
	}
	if !found {
		return nil
	}
	if record.ID != runID.String() {
		return nil
	}

	if record.Status != string(domain.GenerationRunStatusStreaming) {
		return nil
	}
	record.Status = string(status)
	record.LastUpdatedAt = time.Now().UTC()
	if err := s.saveRunByID(ctx, tenantID, runID, record); err != nil {
		return serrors.E(op, "persist terminal run state", err)
	}
	if _, err := s.client.Del(ctx, s.sessionKey(tenantID, sessionID)).Result(); err != nil {
		return serrors.E(op, "delete active session run state", err)
	}
	return nil
}

func (s *redisGenerationRunStore) loadRun(ctx context.Context, tenantID, sessionID uuid.UUID) (persistedGenerationRun, bool, error) {
	key := s.sessionKey(tenantID, sessionID)
	raw, err := s.client.Get(ctx, key).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return persistedGenerationRun{}, false, nil
		}
		return persistedGenerationRun{}, false, err
	}

	var record persistedGenerationRun
	if err := json.Unmarshal(raw, &record); err != nil {
		return persistedGenerationRun{}, false, err
	}
	if record.PartialMeta == nil {
		record.PartialMeta = make(map[string]any)
	}

	return record, true, nil
}

func (s *redisGenerationRunStore) saveRun(ctx context.Context, tenantID, sessionID uuid.UUID, record persistedGenerationRun) error {
	const op serrors.Op = "redisGenerationRunStore.saveRun"

	payload, err := json.Marshal(record)
	if err != nil {
		return serrors.E(op, "marshal run state", err)
	}
	if err := s.client.Set(ctx, s.sessionKey(tenantID, sessionID), payload, s.ttl).Err(); err != nil {
		return serrors.E(op, "set run state", err)
	}
	runID, err := uuid.Parse(record.ID)
	if err != nil {
		return serrors.E(op, "invalid run id", err)
	}
	if err := s.client.Set(ctx, s.runKey(tenantID, runID), payload, s.ttl).Err(); err != nil {
		return serrors.E(op, "set run index", err)
	}
	return nil
}

func (s *redisGenerationRunStore) sessionKey(tenantID, sessionID uuid.UUID) string {
	return fmt.Sprintf("%s:%s:%s", s.keyPrefix, tenantID.String(), sessionID.String())
}

func (s *redisGenerationRunStore) runKey(tenantID, runID uuid.UUID) string {
	return fmt.Sprintf("%s:%s:run:%s", s.keyPrefix, tenantID.String(), runID.String())
}

func (s *redisGenerationRunStore) saveRunByID(ctx context.Context, tenantID, runID uuid.UUID, record persistedGenerationRun) error {
	const op serrors.Op = "redisGenerationRunStore.saveRunByID"

	payload, err := json.Marshal(record)
	if err != nil {
		return serrors.E(op, "marshal run state", err)
	}
	if err := s.client.Set(ctx, s.runKey(tenantID, runID), payload, s.ttl).Err(); err != nil {
		return serrors.E(op, "set run by id", err)
	}
	return nil
}

func mapPersistedGenerationRunToDomain(r persistedGenerationRun) (domain.GenerationRun, error) {
	id, err := uuid.Parse(r.ID)
	if err != nil {
		return nil, err
	}
	sessionID, err := uuid.Parse(r.SessionID)
	if err != nil {
		return nil, err
	}
	tenantID, err := uuid.Parse(r.TenantID)
	if err != nil {
		return nil, err
	}

	return domain.RehydrateGenerationRun(domain.GenerationRunSpec{
		ID:              id,
		SessionID:       sessionID,
		TenantID:        tenantID,
		UserID:          r.UserID,
		Status:          domain.GenerationRunStatus(r.Status),
		PartialContent:  r.PartialContent,
		PartialMetadata: cloneMetadata(r.PartialMeta),
		StartedAt:       r.StartedAt,
		LastUpdatedAt:   r.LastUpdatedAt,
		CancelRequested: r.CancelRequested,
		LastHeartbeatAt: r.LastHeartbeatAt,
	})
}

func cloneMetadata(in map[string]any) map[string]any {
	if len(in) == 0 {
		return make(map[string]any)
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = cloneAny(v)
	}
	return out
}

func cloneAny(v any) any {
	switch typed := v.(type) {
	case map[string]any:
		if len(typed) == 0 {
			return map[string]any{}
		}
		cloned := make(map[string]any, len(typed))
		for key, val := range typed {
			cloned[key] = cloneAny(val)
		}
		return cloned
	case []any:
		cloned := make([]any, len(typed))
		for i, item := range typed {
			cloned[i] = cloneAny(item)
		}
		return cloned
	default:
		return typed
	}
}

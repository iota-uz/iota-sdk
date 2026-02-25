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
	UpdateRunSnapshot(ctx context.Context, tenantID, sessionID, runID uuid.UUID, partialContent string, partialMetadata map[string]any) error
	CompleteRun(ctx context.Context, tenantID, sessionID, runID uuid.UUID) error
	CancelRun(ctx context.Context, tenantID, sessionID, runID uuid.UUID) error
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
		ID:             run.ID().String(),
		SessionID:      run.SessionID().String(),
		TenantID:       run.TenantID().String(),
		UserID:         run.UserID(),
		Status:         string(domain.GenerationRunStatusStreaming),
		PartialContent: run.PartialContent(),
		PartialMeta:    cloneMetadata(run.PartialMetadata()),
		StartedAt:      run.StartedAt().UTC(),
		LastUpdatedAt:  run.LastUpdatedAt().UTC(),
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

	created, err := s.client.SetNX(ctx, s.sessionKey(run.TenantID(), run.SessionID()), payload, s.ttl).Result()
	if err != nil {
		return serrors.E(op, "create run state", err)
	}
	if !created {
		return domain.ErrActiveRunExists
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
	return s.finishRun(ctx, tenantID, sessionID, runID)
}

func (s *redisGenerationRunStore) CancelRun(ctx context.Context, tenantID, sessionID, runID uuid.UUID) error {
	return s.finishRun(ctx, tenantID, sessionID, runID)
}

func (s *redisGenerationRunStore) finishRun(ctx context.Context, tenantID, sessionID, runID uuid.UUID) error {
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

	if _, err := s.client.Del(ctx, s.sessionKey(tenantID, sessionID)).Result(); err != nil {
		return serrors.E(op, "delete run state", err)
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
	return nil
}

func (s *redisGenerationRunStore) sessionKey(tenantID, sessionID uuid.UUID) string {
	return fmt.Sprintf("%s:%s:%s", s.keyPrefix, tenantID.String(), sessionID.String())
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

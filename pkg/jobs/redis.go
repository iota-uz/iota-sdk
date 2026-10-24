package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
	"github.com/redis/go-redis/v9"
)

const (
	redisJobKeyPrefix    = "jobs:job"
	redisResultKeyPrefix = "jobs:result"
	redisUserIndexPrefix = "jobs:user"
)

// RedisStore persists jobs in Redis: one string blob per job, one per stored
// result, and a per-user sorted-set index. All keys carry TTLs, so terminal
// jobs and their results expire without a sweeper.
type RedisStore struct {
	client     redis.UniversalClient
	retention  time.Duration
	staleAfter time.Duration
}

// StoreOption tunes a Store.
type StoreOption func(*storeOptions)

type storeOptions struct {
	staleAfter time.Duration
}

// WithStaleAfter sets how long an active job may go without a heartbeat
// before its TTL margin assumes the worker is gone. It must match the
// runner's StaleAfter so active jobs do not expire mid-flight.
func WithStaleAfter(d time.Duration) StoreOption {
	return func(o *storeOptions) { o.staleAfter = d }
}

// NewRedisStore creates a Redis-backed store. Retention controls the TTL of
// terminal jobs and result blobs; non-positive values fall back to
// DefaultRetention. The client must already be connected.
func NewRedisStore(client redis.UniversalClient, retention time.Duration, opts ...StoreOption) (*RedisStore, error) {
	const op serrors.Op = "jobs.NewRedisStore"
	if client == nil {
		return nil, serrors.E(op, "redis client is required")
	}
	if retention <= 0 {
		retention = DefaultRetention
	}
	o := storeOptions{staleAfter: DefaultStaleAfter}
	for _, opt := range opts {
		opt(&o)
	}
	return &RedisStore{client: client, retention: retention, staleAfter: o.staleAfter}, nil
}

func jobKey(id uuid.UUID) string {
	return fmt.Sprintf("%s:%s", redisJobKeyPrefix, id.String())
}

func resultKey(id uuid.UUID) string {
	return fmt.Sprintf("%s:%s", redisResultKeyPrefix, id.String())
}

func userIndexKey(tenantID uuid.UUID, userID uint) string {
	return fmt.Sprintf("%s:%s:%d", redisUserIndexPrefix, tenantID.String(), userID)
}

// encodedJob is the wire representation stored in the Redis hash.
type encodedJob struct {
	ID         string         `json:"id"`
	TenantID   string         `json:"tenant_id"`
	UserID     uint           `json:"user_id"`
	Kind       string         `json:"kind"`
	Params     map[string]any `json:"params"`
	Status     string         `json:"status"`
	Progress   int            `json:"progress"`
	Phase      string         `json:"phase"`
	Error      string         `json:"error"`
	ResultName string         `json:"result_name"`
	ResultURL  string         `json:"result_url"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
	StartedAt  time.Time      `json:"started_at"`
	FinishedAt time.Time      `json:"finished_at"`
}

func encodeJob(j Job) encodedJob {
	return encodedJob{
		ID:         j.ID.String(),
		TenantID:   j.TenantID.String(),
		UserID:     j.UserID,
		Kind:       j.Kind,
		Params:     j.Params,
		Status:     j.Status.String(),
		Progress:   j.Progress,
		Phase:      j.Phase,
		Error:      j.Error,
		ResultName: j.ResultName,
		ResultURL:  j.ResultURL,
		CreatedAt:  j.CreatedAt,
		UpdatedAt:  j.UpdatedAt,
		StartedAt:  j.StartedAt,
		FinishedAt: j.FinishedAt,
	}
}

func decodeJob(e encodedJob) (Job, error) {
	id, err := uuid.Parse(e.ID)
	if err != nil {
		return Job{}, fmt.Errorf("invalid job id %q: %w", e.ID, err)
	}
	tenantID, err := uuid.Parse(e.TenantID)
	if err != nil {
		return Job{}, fmt.Errorf("invalid tenant id %q: %w", e.TenantID, err)
	}
	if e.Params == nil {
		e.Params = map[string]any{}
	}
	return Job{
		ID:         id,
		TenantID:   tenantID,
		UserID:     e.UserID,
		Kind:       e.Kind,
		Params:     e.Params,
		Status:     Status(e.Status),
		Progress:   e.Progress,
		Phase:      e.Phase,
		Error:      e.Error,
		ResultName: e.ResultName,
		ResultURL:  e.ResultURL,
		CreatedAt:  e.CreatedAt,
		UpdatedAt:  e.UpdatedAt,
		StartedAt:  e.StartedAt,
		FinishedAt: e.FinishedAt,
	}, nil
}

// ttl returns the TTL for a job based on its state: terminal jobs expire with
// the retention window, active jobs get a wide margin past the stale
// threshold (heartbeats refresh it).
func (s *RedisStore) ttl(j Job) time.Duration {
	if j.Status.IsTerminal() {
		return s.retention
	}
	return s.staleAfter*2 + time.Minute
}

func (s *RedisStore) writeJob(ctx context.Context, j Job) error {
	const op serrors.Op = "jobs.RedisStore.writeJob"
	raw, err := json.Marshal(encodeJob(j))
	if err != nil {
		return serrors.E(op, err)
	}
	if err := s.client.Set(ctx, jobKey(j.ID), raw, s.ttl(j)).Err(); err != nil {
		return serrors.E(op, err)
	}
	s.indexUser(ctx, j)
	return nil
}

// indexUser records the job id in the per-user index, pruned by age and
// length. Errors are swallowed: a missing index degrades "my operations"
// listings, not correctness of individual jobs.
func (s *RedisStore) indexUser(ctx context.Context, j Job) {
	key := userIndexKey(j.TenantID, j.UserID)
	member := j.ID.String()
	score := float64(j.CreatedAt.UnixMilli())
	pipe := s.client.Pipeline()
	pipe.ZAdd(ctx, key, redis.Z{Score: score, Member: member})
	pipe.ZRemRangeByScore(ctx, key, "-inf", strconv.FormatInt(time.Now().Add(-s.retention).UnixMilli(), 10))
	pipe.ZRemRangeByRank(ctx, key, 0, -201)
	pipe.Expire(ctx, key, s.retention)
	_, _ = pipe.Exec(ctx)
}

func (s *RedisStore) Create(ctx context.Context, j Job) error {
	return s.writeJob(ctx, j)
}

func (s *RedisStore) Save(ctx context.Context, j Job) error {
	return s.writeJob(ctx, j)
}

func (s *RedisStore) Get(ctx context.Context, id uuid.UUID) (Job, bool, error) {
	const op serrors.Op = "jobs.RedisStore.Get"
	raw, err := s.client.Get(ctx, jobKey(id)).Bytes()
	if err != nil {
		if err == redis.Nil {
			return Job{}, false, nil
		}
		return Job{}, false, serrors.E(op, err)
	}
	var e encodedJob
	if err := json.Unmarshal(raw, &e); err != nil {
		return Job{}, false, serrors.E(op, err)
	}
	j, err := decodeJob(e)
	if err != nil {
		return Job{}, false, serrors.E(op, err)
	}
	return j, true, nil
}

func (s *RedisStore) ListByUser(ctx context.Context, tenantID uuid.UUID, userID uint, limit int) ([]Job, error) {
	const op serrors.Op = "jobs.RedisStore.ListByUser"
	ids, err := s.client.ZRevRange(ctx, userIndexKey(tenantID, userID), 0, int64(limit)-1).Result()
	if err != nil {
		return nil, serrors.E(op, err)
	}
	out := make([]Job, 0, len(ids))
	for _, raw := range ids {
		id, parseErr := uuid.Parse(raw)
		if parseErr != nil {
			continue
		}
		j, ok, getErr := s.Get(ctx, id)
		if getErr != nil {
			return nil, serrors.E(op, getErr)
		}
		// Expired/evicted jobs stay in the index until it is pruned; skip.
		if ok {
			out = append(out, j)
		}
	}
	return out, nil
}

func (s *RedisStore) Delete(ctx context.Context, id uuid.UUID) error {
	const op serrors.Op = "jobs.RedisStore.Delete"
	j, ok, err := s.Get(ctx, id)
	if err != nil {
		return serrors.E(op, err)
	}
	pipe := s.client.Pipeline()
	pipe.Del(ctx, jobKey(id))
	pipe.Del(ctx, resultKey(id))
	if ok {
		pipe.ZRem(ctx, userIndexKey(j.TenantID, j.UserID), j.ID.String())
	}
	if _, err := pipe.Exec(ctx); err != nil {
		return serrors.E(op, err)
	}
	return nil
}

func (s *RedisStore) SaveResult(ctx context.Context, id uuid.UUID, name string, data []byte) error {
	const op serrors.Op = "jobs.RedisStore.SaveResult"
	if err := s.client.Set(ctx, resultKey(id), data, s.retention).Err(); err != nil {
		return serrors.E(op, err)
	}
	return nil
}

func (s *RedisStore) GetResult(ctx context.Context, id uuid.UUID) (string, []byte, bool, error) {
	const op serrors.Op = "jobs.RedisStore.GetResult"
	j, ok, err := s.Get(ctx, id)
	if err != nil || !ok {
		return "", nil, false, serrors.E(op, err)
	}
	data, err := s.client.Get(ctx, resultKey(id)).Bytes()
	if err != nil {
		if err == redis.Nil {
			return "", nil, false, nil
		}
		return "", nil, false, serrors.E(op, err)
	}
	return j.ResultName, data, true, nil
}

func (s *RedisStore) Touch(ctx context.Context, id uuid.UUID, at time.Time) error {
	j, ok, err := s.Get(ctx, id)
	if err != nil || !ok {
		return err
	}
	j.UpdatedAt = at
	return s.writeJob(ctx, j)
}

// Close releases the underlying Redis client.
func (s *RedisStore) Close() error {
	return s.client.Close()
}

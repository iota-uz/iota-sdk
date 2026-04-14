package services

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRedisRunJobQueue_Enqueue_FirstCallWritesStreamEntry(t *testing.T) {
	t.Parallel()

	mr := miniredis.RunT(t)
	queue, err := NewRedisRunJobQueue(RedisRunJobQueueConfig{
		RedisURL: mr.Addr(),
		Stream:   "bichat:run:test-enqueue",
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = queue.Close() })

	payload := RunJobPayload{
		TenantID:  uuid.New(),
		SessionID: uuid.New(),
		UserID:    7,
		RequestID: uuid.New(),
		Content:   "hi",
	}

	runID, deduped, err := queue.Enqueue(context.Background(), payload)
	require.NoError(t, err)
	require.False(t, deduped, "first enqueue must not be marked deduped")
	require.NotEqual(t, uuid.Nil, runID, "queue must assign a run id when payload leaves it empty")

	length, xlenErr := queue.client.XLen(context.Background(), queue.stream).Result()
	require.NoError(t, xlenErr)
	assert.Equal(t, int64(1), length, "first enqueue must write exactly one stream entry")
}

func TestRedisRunJobQueue_Enqueue_DedupesOnSameRequestID(t *testing.T) {
	t.Parallel()

	mr := miniredis.RunT(t)
	queue, err := NewRedisRunJobQueue(RedisRunJobQueueConfig{
		RedisURL:  mr.Addr(),
		Stream:    "bichat:run:test-dedupe",
		DedupeTTL: 5 * time.Minute,
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = queue.Close() })

	requestID := uuid.New()
	base := RunJobPayload{
		TenantID:  uuid.New(),
		SessionID: uuid.New(),
		UserID:    1,
		RequestID: requestID,
		Content:   "duplicate",
	}

	firstID, dedupedFirst, err := queue.Enqueue(context.Background(), base)
	require.NoError(t, err)
	require.False(t, dedupedFirst)

	// Second enqueue with the SAME request_id must return the same run id
	// and be marked deduped. It must NOT add a new stream entry.
	secondID, dedupedSecond, err := queue.Enqueue(context.Background(), base)
	require.NoError(t, err)
	require.True(t, dedupedSecond, "duplicate enqueue must be flagged")
	assert.Equal(t, firstID, secondID, "duplicate enqueue must return the original run id")

	length, xlenErr := queue.client.XLen(context.Background(), queue.stream).Result()
	require.NoError(t, xlenErr)
	assert.Equal(t, int64(1), length, "duplicate enqueue must not produce a new stream entry")
}

func TestRedisRunJobQueue_Enqueue_DistinctRequestIDsProduceDistinctRuns(t *testing.T) {
	t.Parallel()

	mr := miniredis.RunT(t)
	queue, err := NewRedisRunJobQueue(RedisRunJobQueueConfig{
		RedisURL: mr.Addr(),
		Stream:   "bichat:run:test-distinct",
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = queue.Close() })

	tenant := uuid.New()
	session := uuid.New()

	first, _, err := queue.Enqueue(context.Background(), RunJobPayload{
		TenantID: tenant, SessionID: session, RequestID: uuid.New(), Content: "a",
	})
	require.NoError(t, err)
	second, _, err := queue.Enqueue(context.Background(), RunJobPayload{
		TenantID: tenant, SessionID: session, RequestID: uuid.New(), Content: "b",
	})
	require.NoError(t, err)

	assert.NotEqual(t, first, second, "distinct request ids must yield distinct run ids")

	length, xlenErr := queue.client.XLen(context.Background(), queue.stream).Result()
	require.NoError(t, xlenErr)
	assert.Equal(t, int64(2), length, "distinct request ids must produce two stream entries")
}

func TestRedisRunJobQueue_Enqueue_RejectsInvalidInput(t *testing.T) {
	t.Parallel()

	mr := miniredis.RunT(t)
	queue, err := NewRedisRunJobQueue(RedisRunJobQueueConfig{
		RedisURL: mr.Addr(),
		Stream:   "bichat:run:test-validation",
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = queue.Close() })

	tenant := uuid.New()
	session := uuid.New()
	reqID := uuid.New()

	cases := []struct {
		name    string
		payload RunJobPayload
	}{
		{"missing tenant", RunJobPayload{SessionID: session, RequestID: reqID}},
		{"missing session", RunJobPayload{TenantID: tenant, RequestID: reqID}},
		{"missing request id", RunJobPayload{TenantID: tenant, SessionID: session}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := queue.Enqueue(context.Background(), tc.payload)
			require.Error(t, err)
		})
	}
}

func TestRedisRunJobQueue_Enqueue_HonoursCallerSuppliedRunID(t *testing.T) {
	t.Parallel()

	mr := miniredis.RunT(t)
	queue, err := NewRedisRunJobQueue(RedisRunJobQueueConfig{
		RedisURL: mr.Addr(),
		Stream:   "bichat:run:test-caller-runid",
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = queue.Close() })

	requested := uuid.New()
	returned, _, err := queue.Enqueue(context.Background(), RunJobPayload{
		TenantID:  uuid.New(),
		SessionID: uuid.New(),
		RequestID: uuid.New(),
		RunID:     requested,
	})
	require.NoError(t, err)
	assert.Equal(t, requested, returned, "queue must preserve caller-supplied RunID")
}

func TestParseRunJobPayload_RoundTripsViaRedis(t *testing.T) {
	t.Parallel()

	mr := miniredis.RunT(t)
	queue, err := NewRedisRunJobQueue(RedisRunJobQueueConfig{
		RedisURL: mr.Addr(),
		Stream:   "bichat:run:test-roundtrip",
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = queue.Close() })

	original := RunJobPayload{
		TenantID:  uuid.New(),
		SessionID: uuid.New(),
		UserID:    42,
		RequestID: uuid.New(),
		Content:   "hello",
		UploadIDs: []int64{1, 2, 3},
		DebugMode: true,
	}
	_, _, err = queue.Enqueue(context.Background(), original)
	require.NoError(t, err)

	ctx := context.Background()
	entries, err := queue.client.XRange(ctx, queue.stream, "-", "+").Result()
	require.NoError(t, err)
	require.Len(t, entries, 1)

	parsed, err := ParseRunJobPayload(entries[0].Values)
	require.NoError(t, err)
	assert.Equal(t, original.TenantID, parsed.TenantID)
	assert.Equal(t, original.SessionID, parsed.SessionID)
	assert.Equal(t, original.UserID, parsed.UserID)
	assert.Equal(t, original.Content, parsed.Content)
	assert.Equal(t, original.UploadIDs, parsed.UploadIDs)
	assert.Equal(t, original.DebugMode, parsed.DebugMode)
	// Queue assigns a RunID when the caller omits one; round-trip must preserve it.
	assert.NotEqual(t, uuid.Nil, parsed.RunID)
}

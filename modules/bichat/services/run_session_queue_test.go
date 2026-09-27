package services

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestRunSessionQueue(t *testing.T) *RedisRunSessionQueue {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	q, err := NewRedisRunSessionQueue(RedisRunSessionQueueConfig{Client: client})
	require.NoError(t, err)
	return q
}

func TestRedisRunSessionQueue_PushAndPopPreservesFIFO(t *testing.T) {
	t.Parallel()

	q := newTestRunSessionQueue(t)
	tenant := uuid.New()
	session := uuid.New()

	contents := []string{"first", "second", "third"}
	for _, c := range contents {
		require.NoError(t, q.Push(context.Background(), tenant, session, QueuedRunJob{
			Payload: RunJobPayload{Content: c, RequestID: uuid.New()},
		}))
	}

	got := make([]string, 0, len(contents))
	for i := 0; i < len(contents); i++ {
		job, ok, err := q.Pop(context.Background(), tenant, session)
		require.NoError(t, err)
		require.True(t, ok)
		got = append(got, job.Payload.Content)
	}
	assert.Equal(t, contents, got, "queue must drain in push order")

	// Extra Pop on empty must report false without error.
	_, ok, err := q.Pop(context.Background(), tenant, session)
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestRedisRunSessionQueue_IsSessionScoped(t *testing.T) {
	t.Parallel()

	q := newTestRunSessionQueue(t)
	tenant := uuid.New()
	sessionA := uuid.New()
	sessionB := uuid.New()

	require.NoError(t, q.Push(context.Background(), tenant, sessionA, QueuedRunJob{
		Payload: RunJobPayload{Content: "A"},
	}))
	require.NoError(t, q.Push(context.Background(), tenant, sessionB, QueuedRunJob{
		Payload: RunJobPayload{Content: "B"},
	}))

	got, ok, err := q.Pop(context.Background(), tenant, sessionA)
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, "A", got.Payload.Content)

	// Session A queue is now empty — B must still have its entry.
	_, ok, err = q.Pop(context.Background(), tenant, sessionA)
	require.NoError(t, err)
	assert.False(t, ok)

	got, ok, err = q.Pop(context.Background(), tenant, sessionB)
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, "B", got.Payload.Content)
}

func TestRedisRunSessionQueue_LenReportsDepth(t *testing.T) {
	t.Parallel()

	q := newTestRunSessionQueue(t)
	tenant := uuid.New()
	session := uuid.New()

	initial, err := q.Len(context.Background(), tenant, session)
	require.NoError(t, err)
	assert.Equal(t, int64(0), initial, "new sessions start at zero depth")

	for i := 0; i < 3; i++ {
		require.NoError(t, q.Push(context.Background(), tenant, session, QueuedRunJob{
			Payload: RunJobPayload{Content: "x"},
		}))
	}
	n, err := q.Len(context.Background(), tenant, session)
	require.NoError(t, err)
	assert.Equal(t, int64(3), n)
}

func TestRedisRunSessionQueue_MaxLenDropsOldestEntries(t *testing.T) {
	t.Parallel()

	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	q, err := NewRedisRunSessionQueue(RedisRunSessionQueueConfig{
		Client: client,
		MaxLen: 2,
	})
	require.NoError(t, err)

	tenant := uuid.New()
	session := uuid.New()
	for _, c := range []string{"A", "B", "C", "D"} {
		require.NoError(t, q.Push(context.Background(), tenant, session, QueuedRunJob{
			Payload: RunJobPayload{Content: c},
		}))
	}

	// Only the last two entries must survive.
	n, err := q.Len(context.Background(), tenant, session)
	require.NoError(t, err)
	assert.Equal(t, int64(2), n)

	first, ok, err := q.Pop(context.Background(), tenant, session)
	require.NoError(t, err)
	require.True(t, ok)
	second, ok, err := q.Pop(context.Background(), tenant, session)
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, []string{"C", "D"}, []string{first.Payload.Content, second.Payload.Content},
		"MaxLen trim must drop oldest entries, preserving the newest two")
}

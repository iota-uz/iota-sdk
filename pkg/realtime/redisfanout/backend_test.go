package redisfanout_test

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/realtime"
	"github.com/iota-uz/iota-sdk/pkg/realtime/redisfanout"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

func TestTwoInstances(t *testing.T) {
	server := miniredis.RunT(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	a, err := redisfanout.New(ctx, "redis://"+server.Addr(), logrus.New())
	require.NoError(t, err)
	defer func() { require.NoError(t, a.Close()) }()
	b, err := redisfanout.New(ctx, "redis://"+server.Addr(), logrus.New())
	require.NoError(t, err)
	defer func() { require.NoError(t, b.Close()) }()
	first, second := make(chan realtime.Envelope, 2), make(chan realtime.Envelope, 2)
	require.NoError(t, a.Subscribe(ctx, func(_ context.Context, e realtime.Envelope) error { first <- e; return nil }))
	require.NoError(t, b.Subscribe(ctx, func(_ context.Context, e realtime.Envelope) error { second <- e; return nil }))
	tenant := uuid.New()
	e := realtime.Envelope{ID: uuid.New(), TenantID: tenant, Channel: realtime.UserChannel(tenant, 3), PublishedAt: time.Now().UTC(), Payload: []byte("notification.changed")}
	require.NoError(t, b.Publish(ctx, e))
	for _, ch := range []chan realtime.Envelope{first, second} {
		select {
		case got := <-ch:
			require.Equal(t, e, got)
		case <-time.After(time.Second):
			t.Fatal("fanout not received")
		}
	}
	require.NoError(t, a.Close())
	require.NoError(t, a.Close())
}
func TestInvalidConfiguredRedis(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	_, err := redisfanout.New(ctx, "redis://127.0.0.1:1", logrus.New())
	require.Error(t, err)
	_, err = redisfanout.New(ctx, "invalid", logrus.New())
	require.Error(t, err)
}

func TestSubscriberReconnects(t *testing.T) {
	server := miniredis.RunT(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	backend, err := redisfanout.New(ctx, "redis://"+server.Addr(), logrus.New())
	require.NoError(t, err)
	defer func() { require.NoError(t, backend.Close()) }()
	received := make(chan realtime.Envelope, 16)
	require.NoError(t, backend.Subscribe(ctx, func(_ context.Context, e realtime.Envelope) error { received <- e; return nil }))
	server.Close()
	require.Eventually(t, func() bool { return backend.Counters().Reconnect > 0 }, 3*time.Second, 10*time.Millisecond)
	require.NoError(t, server.Restart())
	tenant := uuid.New()
	require.Eventually(t, func() bool {
		e := realtime.Envelope{ID: uuid.New(), TenantID: tenant, Channel: realtime.UserChannel(tenant, 1), PublishedAt: time.Now()}
		_ = backend.Publish(ctx, e)
		select {
		case <-received:
			return true
		default:
			return false
		}
	}, 5*time.Second, 50*time.Millisecond)
}

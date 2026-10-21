package realtime_test

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/realtime"
	"github.com/iota-uz/iota-sdk/pkg/realtime/memory"
	"github.com/stretchr/testify/require"
)

func TestEnvelopeValidation(t *testing.T) {
	tenant := uuid.New()
	e := realtime.Envelope{ID: uuid.New(), TenantID: tenant, Channel: realtime.UserChannel(tenant, 1), PublishedAt: time.Now(), Payload: []byte("hello")}
	require.NoError(t, e.Validate())
	e.Channel = realtime.UserChannel(uuid.New(), 1)
	require.Error(t, e.Validate())
	e.Channel = realtime.UserChannel(tenant, 1)
	e.Payload = bytes.Repeat([]byte("x"), realtime.MaxPayloadBytes)
	require.ErrorIs(t, e.Validate(), realtime.ErrOversize)
}
func TestMemoryCancellation(t *testing.T) {
	b := memory.New()
	ctx, cancel := context.WithCancel(context.Background())
	received := make(chan realtime.Envelope, 1)
	require.NoError(t, b.Subscribe(ctx, func(_ context.Context, e realtime.Envelope) error { received <- e; return nil }))
	tenant := uuid.New()
	e := realtime.Envelope{ID: uuid.New(), TenantID: tenant, Channel: realtime.UserChannel(tenant, 1), PublishedAt: time.Now()}
	require.NoError(t, b.Publish(ctx, e))
	require.Equal(t, e, <-received)
	cancel()
	require.ErrorIs(t, b.Publish(ctx, e), context.Canceled)
	require.NoError(t, b.Close())
	require.NoError(t, b.Close())
}

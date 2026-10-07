package bootstrap

import (
	"context"
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/redisconfig"
	"github.com/iota-uz/iota-sdk/pkg/realtime/memory"
	"github.com/stretchr/testify/require"
)

func TestRealtimeBackendSelection(t *testing.T) {
	backend, err := newRealtimeBackend(context.Background(), &redisconfig.Config{}, nil)
	require.NoError(t, err)
	require.IsType(t, &memory.Backend{}, backend)
	require.NoError(t, backend.Close())
	_, err = newRealtimeBackend(context.Background(), &redisconfig.Config{URL: "redis://invalid:port"}, nil)
	require.Error(t, err)
}

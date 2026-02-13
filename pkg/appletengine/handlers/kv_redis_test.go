package handlers

import (
	"context"
	"testing"

	appletenginerpc "github.com/iota-uz/iota-sdk/pkg/appletengine/rpc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRedisKVStore_RequiresURL(t *testing.T) {
	t.Parallel()

	store, err := NewRedisKVStore("")
	require.Error(t, err)
	assert.Nil(t, store)
	assert.Contains(t, err.Error(), "redis url is required")
}

func TestRedisScopedKey_UsesTenantAndAppletScope(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ctx = appletenginerpc.WithTenantID(ctx, "tenant-1")
	ctx = appletenginerpc.WithAppletID(ctx, "bichat")

	assert.Equal(t, "applet:tenant-1:bichat:key1", redisScopedKey(ctx, "key1"))
}

package handlers

import (
	"context"
	"testing"

	appletenginerpc "github.com/iota-uz/iota-sdk/pkg/appletengine/rpc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewPostgresDBStore_RequiresPool(t *testing.T) {
	t.Parallel()

	store, err := NewPostgresDBStore(nil)
	require.Error(t, err)
	assert.Nil(t, store)
	assert.Contains(t, err.Error(), "postgres pool is required")
}

func TestTenantAndAppletFromContext_Defaults(t *testing.T) {
	t.Parallel()

	tenantID, appletID := tenantAndAppletFromContext(context.Background())
	assert.Equal(t, "default", tenantID)
	assert.Equal(t, "unknown", appletID)

	ctx := appletenginerpc.WithTenantID(context.Background(), "tenant-1")
	ctx = appletenginerpc.WithAppletID(ctx, "bichat")
	tenantID, appletID = tenantAndAppletFromContext(ctx)
	assert.Equal(t, "tenant-1", tenantID)
	assert.Equal(t, "bichat", appletID)
}

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

	tenantID, appletID, err := tenantAndAppletFromContext(context.Background())
	assert.Error(t, err)
	assert.Empty(t, tenantID)
	assert.Empty(t, appletID)
	assert.Contains(t, err.Error(), "tenant ID not found in context")

	ctx := appletenginerpc.WithTenantID(context.Background(), "tenant-1")
	ctx = appletenginerpc.WithAppletID(ctx, "bichat")
	tenantID, appletID, err = tenantAndAppletFromContext(ctx)
	require.NoError(t, err)
	assert.Equal(t, "tenant-1", tenantID)
	assert.Equal(t, "bichat", appletID)
}

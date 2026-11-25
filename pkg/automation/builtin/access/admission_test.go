package access_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/automation/builtin/access"
	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/automation/code"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors/httpapi"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
)

const useConnections = "Connections.Use"

// A run-as subject without the connection-use permission reaches no provider: neither an HTTP call nor an
// operation, and a node opening a connection is refused before it is opened.
func TestConnectionsNeedTheUsePermissionBeforeAnyProviderCall(t *testing.T) {
	kind := &fakeKind{access: connectors.CodeAccess{Fetch: true, Operations: []connectors.CodeOperation{{Name: "send"}}}}
	c, err := openWith(t, kind, connectors.StatusConnected)
	require.NoError(t, err)
	rec := &recorder{}
	g := &access.Gateway{Fetcher: rec, Connections: map[string]access.Connection{"crm": c}, Use: useConnections,
		Input: catalog.CallInput{RunAs: spi.Subject{}}}

	_, err = g.CallConnection(context.Background(), code.ConnectionRequest{Connection: "crm", Method: "fetch", Fetch: true, Path: "/x"})
	var refused *code.ConnectionRefusedError
	require.ErrorAs(t, err, &refused)
	assert.EqualValues(t, access.CodePermissionDenied, refused.Code)
	assert.Empty(t, rec.got.URL, "no request was sent")

	_, err = g.CallOperation(context.Background(), code.OperationRequest{Connection: "crm", Operation: "send", Args: []byte("null")})
	require.ErrorAs(t, err, &refused)
	assert.EqualValues(t, access.CodePermissionDenied, refused.Code)
	assert.Empty(t, kind.calls, "the operator was not called")

	g.Input.RunAs = spi.Subject{Permissions: []string{useConnections}}
	_, err = g.CallOperation(context.Background(), code.OperationRequest{Connection: "crm", Operation: "send", Args: []byte("null")})
	require.NoError(t, err)
	assert.Len(t, kind.calls, 1)

	opened := 0
	counting := countingConnections{opened: &opened}
	_, err = access.OpenFor(context.Background(), counting, catalog.CallInput{TenantID: uuid.New()}, []string{"crm"}, useConnections)
	var ce *catalog.CallError
	require.ErrorAs(t, err, &ce)
	assert.Equal(t, access.CodePermissionDenied, ce.Code)
	assert.Zero(t, opened)
}

type countingConnections struct{ opened *int }

func (c countingConnections) Open(context.Context, uuid.UUID, string) (access.Connection, error) {
	*c.opened++
	return access.Connection{}, nil
}

// A connection disabled or marked broken while a run holds it open is refused at its next call, before the
// provider is contacted.
func TestAnOpenConnectionIsCheckedAgainAtEveryCall(t *testing.T) {
	r := newStoreRig(t)
	conn := r.save(t, httpapi.Kind, "crm", connectors.Credentials{"base_url": "https://crm.example.com/v1", "auth": "bearer", "token": itfAPIToken})
	opened, err := access.OpenAll(r.ctx, r.conns, r.tenant, []string{"crm"})
	require.NoError(t, err)
	rec := &recorder{}
	g := &access.Gateway{Fetcher: rec, Connections: opened}
	call := func() error {
		rec.got = code.FetchRequest{}
		_, err := g.CallConnection(r.ctx, code.ConnectionRequest{Connection: "crm", Method: "fetch", Fetch: true, Path: "/deals"})
		return err
	}
	require.NoError(t, call())
	assert.NotEmpty(t, rec.got.URL)

	for status, want := range map[connectors.ConnectionStatus]string{
		connectors.StatusDisabled:    access.CodeConnectionDisabled,
		connectors.StatusNeedsReauth: access.CodeConnectionBroken,
		connectors.StatusError:       access.CodeConnectionBroken,
	} {
		require.NoError(t, r.store.SetStatus(r.ctx, r.tenant, conn.ID, status, ""))
		err := call()
		var refused *code.ConnectionRefusedError
		require.ErrorAs(t, err, &refused, status)
		assert.EqualValues(t, want, refused.Code, status)
		assert.Empty(t, rec.got.URL, "%s: nothing was sent", status)
	}

	require.NoError(t, r.store.SetStatus(r.ctx, r.tenant, conn.ID, connectors.StatusConnected, ""))
	require.NoError(t, call(), "connected again")
	require.NoError(t, r.store.Delete(r.ctx, r.tenant, conn.ID))
	err = call()
	var refused *code.ConnectionRefusedError
	require.ErrorAs(t, err, &refused)
	assert.Equal(t, code.ReasonConnectionMissing, refused.Code)
}

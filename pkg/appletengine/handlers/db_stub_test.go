package handlers

import (
	"testing"

	appletenginerpc "github.com/iota-uz/iota-sdk/pkg/appletengine/rpc"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDBStub_MethodShapeAndTenantIsolation(t *testing.T) {
	t.Parallel()

	registry := appletenginerpc.NewRegistry()
	stub := NewDBStub()
	require.NoError(t, stub.Register(registry, "bichat"))
	dispatcher := appletenginerpc.NewDispatcher(registry, nil, logrus.New())

	insertResp := callServerRPC(t, dispatcher, "tenant-a", "bichat.db.insert", map[string]any{
		"table": "messages",
		"value": map[string]any{"text": "hello"},
	})
	inserted := insertResp["result"].(map[string]any)
	docID, ok := inserted["_id"].(string)
	require.True(t, ok)
	require.NotEmpty(t, docID)
	assert.Equal(t, "messages", inserted["table"])

	getResp := callServerRPC(t, dispatcher, "tenant-a", "bichat.db.get", map[string]any{"id": docID})
	assert.Equal(t, docID, getResp["result"].(map[string]any)["_id"])

	queryResp := callServerRPC(t, dispatcher, "tenant-a", "bichat.db.query", map[string]any{"table": "messages"})
	records := queryResp["result"].([]any)
	require.Len(t, records, 1)

	filteredQuery := callServerRPC(t, dispatcher, "tenant-a", "bichat.db.query", map[string]any{
		"table": "messages",
		"query": map[string]any{
			"filters": []any{
				map[string]any{"field": "text", "op": "eq", "value": "hello"},
			},
		},
	})
	require.Len(t, filteredQuery["result"].([]any), 1)

	noMatchQuery := callServerRPC(t, dispatcher, "tenant-a", "bichat.db.query", map[string]any{
		"table": "messages",
		"query": map[string]any{
			"filters": []any{
				map[string]any{"field": "text", "op": "eq", "value": "missing"},
			},
		},
	})
	require.Len(t, noMatchQuery["result"].([]any), 0)

	otherTenantQuery := callServerRPC(t, dispatcher, "tenant-b", "bichat.db.query", map[string]any{"table": "messages"})
	assert.Len(t, otherTenantQuery["result"].([]any), 0)

	patchResp := callServerRPC(t, dispatcher, "tenant-a", "bichat.db.patch", map[string]any{
		"id":    docID,
		"value": map[string]any{"text": "patched"},
	})
	assert.Equal(t, "patched", patchResp["result"].(map[string]any)["value"].(map[string]any)["text"])

	replaceResp := callServerRPC(t, dispatcher, "tenant-a", "bichat.db.replace", map[string]any{
		"id":    docID,
		"value": map[string]any{"text": "replaced"},
	})
	assert.Equal(t, "replaced", replaceResp["result"].(map[string]any)["value"].(map[string]any)["text"])

	deleteResp := callServerRPC(t, dispatcher, "tenant-a", "bichat.db.delete", map[string]any{"id": docID})
	assert.Equal(t, true, deleteResp["result"])

	getAfterDelete := callServerRPC(t, dispatcher, "tenant-a", "bichat.db.get", map[string]any{"id": docID})
	assert.Nil(t, getAfterDelete["result"])
}

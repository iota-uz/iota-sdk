package mcpops_test

import (
	"encoding/json"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/automation/mcpops"
	"github.com/iota-uz/iota-sdk/pkg/automation/state"
	"github.com/iota-uz/iota-sdk/pkg/mcp"
	"github.com/iota-uz/iota-sdk/pkg/mcp/changes"
)

func TestStateOperationsOfTheMCP(t *testing.T) {
	e := newEnv(t)
	created := e.apply(agentID, e.prepare(agentID, map[string]any{"op": "create", "entity": "automation",
		"fields": map[string]any{"graph": noteGraph("Welcome", "hello")}}), "create-1")
	id := receiptResult(t, created).ID

	// The agent writes keys through guarded changes (it authors).
	n := 0
	prepareSet := func(key string, value any, extra map[string]any) prepared {
		fields := map[string]any{"id": id, "key": key, "value": value}
		for k, v := range extra {
			fields[k] = v
		}
		return e.prepare(agentID, map[string]any{"op": mcpops.OpStateSet, "entity": "automation", "fields": fields})
	}
	set := func(key string, value any, extra map[string]any) operation {
		n++
		return e.apply(agentID, prepareSet(key, value, extra), "set-"+key+strconv.Itoa(n))
	}
	first := prepareSet("chat:1", map[string]any{"task": "T-1"}, nil)
	firstOp := e.apply(agentID, first, "set-first")
	assert.Equal(t, changes.OperationSucceeded, firstOp.Status)
	set("chat:2", 5, map[string]any{"ttl": "2h"})
	set("other", "x", nil)

	// A repeated apply after a lost response returns the same operation and writes nothing again: the key
	// changed meanwhile keeps its newer value.
	set("chat:1", map[string]any{"task": "T-2"}, nil)
	replayed := e.apply(agentID, first, "set-first")
	assert.Equal(t, firstOp.OperationID, replayed.OperationID)
	var current struct {
		Value json.RawMessage `json:"value"`
	}
	e.ok(readerID, mcpops.ToolStateGet, map[string]any{"id": id, "key": "chat:1"}, &current)
	assert.JSONEq(t, `{"task":"T-2"}`, string(current.Value))
	set("chat:1", map[string]any{"task": "T-1"}, nil)

	type page struct {
		Items []struct {
			Key       string          `json:"key"`
			Value     json.RawMessage `json:"value"`
			ExpiresAt *string         `json:"expires_at"`
		} `json:"items"`
		NextCursor *string `json:"next_cursor"`
		Total      int     `json:"total"`
	}
	var list page
	e.ok(readerID, mcpops.ToolStateList, map[string]any{"id": id}, &list)
	require.Len(t, list.Items, 3, "a reader sees the keys")
	assert.Equal(t, 3, list.Total)
	assert.Equal(t, "chat:1", list.Items[0].Key)
	assert.JSONEq(t, `{"task":"T-1"}`, string(list.Items[0].Value))
	assert.NotNil(t, list.Items[1].ExpiresAt, "chat:2 expires")
	assert.Nil(t, list.Items[0].ExpiresAt)

	e.ok(readerID, mcpops.ToolStateList, map[string]any{"id": id, "search": "CHAT", "limit": 1}, &list)
	require.Len(t, list.Items, 1)
	require.NotNil(t, list.NextCursor)
	assert.Equal(t, 3, list.Total, "the total ignores the search")
	e.ok(readerID, mcpops.ToolStateList, map[string]any{"id": id, "search": "CHAT", "limit": 1, "cursor": *list.NextCursor}, &list)
	require.Len(t, list.Items, 1)
	assert.Equal(t, "chat:2", list.Items[0].Key)
	assert.Nil(t, list.NextCursor)

	var one struct {
		Key   string          `json:"key"`
		Value json.RawMessage `json:"value"`
	}
	e.ok(readerID, mcpops.ToolStateGet, map[string]any{"id": id, "key": "chat:2"}, &one)
	assert.JSONEq(t, `5`, string(one.Value))
	_ = e.failure(readerID, mcpops.ToolStateGet, map[string]any{"id": id, "key": "nope"}, mcp.CodeNotFound)

	// A reader may not write: the changes need Automations.Author.
	_ = e.failure(readerID, "prepare_changes", map[string]any{"changes": []any{map[string]any{"op": mcpops.OpStateSet, "entity": "automation",
		"fields": map[string]any{"id": id, "key": "x", "value": 1}}}}, mcp.CodeForbidden)

	// A value over the limit and a bad ttl are plan errors; nothing is written.
	big := make([]byte, state.MaxValueBytes+1)
	for i := range big {
		big[i] = 'x'
	}
	p := e.prepare(agentID, map[string]any{"op": mcpops.OpStateSet, "entity": "automation", "fields": map[string]any{"id": id, "key": "big", "value": string(big)}})
	require.NotEmpty(t, p.Errors)
	assert.Equal(t, state.CodeValueTooLarge, p.Errors[0].Code)
	p = e.prepare(agentID, map[string]any{"op": mcpops.OpStateSet, "entity": "automation", "fields": map[string]any{"id": id, "key": "k", "value": 1, "ttl": "30m"}})
	assert.Empty(t, p.Errors)
	assert.Equal(t, 3, e.count(`SELECT count(*) FROM automation.state`))

	// Delete: the key goes; a key that is not there is a plan error.
	del := func(key string) prepared {
		return e.prepare(agentID, map[string]any{"op": mcpops.OpStateDelete, "entity": "automation", "fields": map[string]any{"id": id, "key": key}})
	}
	e.apply(agentID, del("other"), "del-other")
	assert.Equal(t, 2, e.count(`SELECT count(*) FROM automation.state`))
	missing := del("other")
	require.NotEmpty(t, missing.Errors)
	assert.Equal(t, string(mcp.CodeNotFound), missing.Errors[0].Code)
	_ = e.failure(readerID, "prepare_changes", map[string]any{"changes": []any{map[string]any{"op": mcpops.OpStateDelete, "entity": "automation",
		"fields": map[string]any{"id": id, "key": "chat:1"}}}}, mcp.CodeForbidden)
}

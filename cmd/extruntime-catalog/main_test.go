package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/catalog"
)

func TestCommittedSnapshotIsCurrent(t *testing.T) {
	want, err := Render()
	require.NoError(t, err)
	have, err := os.ReadFile(filepath.Join("..", "..", DefaultOutput))
	require.NoError(t, err)
	require.True(t, bytes.Equal(have, want), "%s is stale: run `go run ./cmd/extruntime-catalog` and commit the result", DefaultOutput)
}

func TestSnapshotIsDeterministicAndComplete(t *testing.T) {
	a, err := Render()
	require.NoError(t, err)
	b, err := Render()
	require.NoError(t, err)
	require.Equal(t, a, b)

	var doc catalog.Snapshot
	require.NoError(t, json.Unmarshal(a, &doc))
	require.Equal(t, catalog.SnapshotSchema, doc.Schema)
	require.Equal(t, catalog.GeneratedHeader, doc.Generated)

	keyTypes := map[string]string{}
	events := []string{}
	lists := map[string][]string{}
	for _, s := range doc.Sources {
		require.NotNil(t, s.Resources, "%s: resources must serialize as an array", s.Namespace)
		lists[s.Namespace] = s.Lists
		for _, r := range s.Resources {
			switch r.Kind {
			case "entity":
				keyTypes[r.ID] = r.KeyType
			case "event":
				events = append(events, r.ID)
			}
		}
	}
	require.Equal(t, map[string]string{"core.user": "int64", "crm.client": "int64"}, keyTypes, "native entities carry their key type")
	require.Contains(t, events, "core.user.created.v1")
	require.Equal(t, []string{"crm.lists.clients"}, lists["crm"])
}

func TestCheckFlagReportsStaleFile(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "snap.json")
	require.NoError(t, os.WriteFile(file, []byte("{}\n"), 0o644))
	var out, errOut bytes.Buffer
	require.Error(t, run([]string{"-check", "-out", file}, &out, &errOut))

	require.NoError(t, run([]string{"-out", file}, &out, &errOut))
	require.NoError(t, run([]string{"-check", "-out", file}, &out, &errOut))
}

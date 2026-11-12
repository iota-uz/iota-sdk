package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/catalog"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/openapi"
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
	ids := map[string]string{}
	lists := map[string][]string{}
	for _, s := range doc.Sources {
		require.NotNil(t, s.Resources, "%s: resources must serialize as an array", s.Namespace)
		lists[s.Namespace] = s.Lists
		for _, r := range s.Resources {
			if r.Kind == manifest.ResEntity {
				keyTypes[r.ID] = r.KeyType
			}
			ids[r.ID] = string(r.Kind)
		}
	}
	require.Equal(t, map[string]string{"core.user": "int64", "crm.client": "int64"}, keyTypes, "native entities carry their key type")
	require.Equal(t, "event", ids["core.user.created.v1"])
	require.Equal(t, "event", ids["crm.client.created.v1"])
	require.Equal(t, "event", ids["crm.client.updated.v1"])
	require.Equal(t, "route", ids["crm.clients"])
	require.Equal(t, "command", ids["crm.commands.saveClient"], "the Go-implemented CRM contracts are in the offline kit")
	require.Equal(t, "operation", ids["crm.operations.upsertClient"])
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

func TestOpenAPIExportDescribesTheNativeCatalog(t *testing.T) {
	file := filepath.Join(t.TempDir(), "api.json")
	var out, errOut bytes.Buffer
	require.NoError(t, run([]string{"-openapi", file}, &out, &errOut))

	raw, err := os.ReadFile(file)
	require.NoError(t, err)
	require.NoError(t, openapi.Validate(raw))
	var doc struct {
		OpenAPI string                    `json:"openapi"`
		Paths   map[string]map[string]any `json:"paths"`
		Comps   struct {
			Schemas map[string]any `json:"schemas"`
		} `json:"components"`
	}
	require.NoError(t, json.Unmarshal(raw, &doc))
	require.Equal(t, "3.1.0", doc.OpenAPI)
	require.Contains(t, doc.Paths, "/_ext/rpc")
	require.Contains(t, doc.Paths, "/_ext/manage/packages")
	require.Contains(t, doc.Comps.Schemas, "rpc.request.crm.commands.saveClient", "the native commands are RPC variants")
	require.NotContains(t, doc.Comps.Schemas, "rpc.request.crm.operations.upsertClient", "operations are not callable from the browser")

	// the snapshot default is untouched, and -check works on the OpenAPI file
	require.NoError(t, run([]string{"-openapi", file, "-check"}, &out, &errOut))
	require.NoError(t, os.WriteFile(file, []byte("{}\n"), 0o644))
	require.Error(t, run([]string{"-openapi", file, "-check"}, &out, &errOut))

	again, err := RenderOpenAPI()
	require.NoError(t, err)
	second, err := RenderOpenAPI()
	require.NoError(t, err)
	require.Equal(t, again, second, "the export is deterministic")
}

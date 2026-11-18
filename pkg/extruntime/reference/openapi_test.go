package reference_test

import (
	"context"
	"encoding/json"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/openapi"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/reference"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
)

func contractDoc(t *testing.T, who spi.Subject) (map[string]any, []byte) {
	t.Helper()
	m, err := newBuilder(t).Model(context.Background(), who, "en")
	require.NoError(t, err)
	b := openapi.New(openapi.Info{Title: "t", Version: "1"})
	reference.AddContracts(b, m)
	raw, err := b.JSON()
	require.NoError(t, err)
	require.NoError(t, openapi.Validate(raw), "the document must be valid OpenAPI 3.1: %s", raw)
	var doc map[string]any
	require.NoError(t, json.Unmarshal(raw, &doc))
	return doc, raw
}

func sub(t *testing.T, doc map[string]any, path ...string) any {
	t.Helper()
	var cur any = doc
	for _, p := range path {
		m, ok := cur.(map[string]any)
		require.True(t, ok, "at %s", p)
		cur, ok = m[p]
		require.True(t, ok, "missing %s", p)
	}
	return cur
}

func TestContractsDocumentRPCAndEndpoints(t *testing.T) {
	t.Parallel()
	doc, _ := contractDoc(t, admin())

	paths := sub(t, doc, "paths").(map[string]any)
	require.Contains(t, paths, "/_ext/rpc")
	require.Contains(t, paths, "/_ext/endpoints/acme.loyalty/webhooks/stripe")
	require.Contains(t, paths, "/_ext/endpoints/acme.loyalty/export")

	hook := sub(t, doc, "paths", "/_ext/endpoints/acme.loyalty/webhooks/stripe", "post").(map[string]any)
	require.Equal(t, []any{}, hook["security"], "a public endpoint needs no session")
	require.Contains(t, hook["description"], "no request or response schema", "the untyped body is stated")
	require.Contains(t, hook, "requestBody")
	private := sub(t, doc, "paths", "/_ext/endpoints/acme.loyalty/export", "get").(map[string]any)
	require.Equal(t, []any{map[string]any{"sessionCookie": []any{}}}, private["security"])
	require.NotContains(t, private, "requestBody")

	rpc := sub(t, doc, "paths", "/_ext/rpc", "post").(map[string]any)
	require.Contains(t, rpc["description"], "Same-origin")
	req := sub(t, doc, "components", "schemas", "RpcRequest").(map[string]any)
	one := req["oneOf"].([]any)
	require.Len(t, one, 4, "the command, the atomic command and the two queries")
	require.Equal(t, "resource", req["discriminator"].(map[string]any)["propertyName"])
	require.Contains(t, sub(t, doc, "components", "schemas"), "rpc.value.acme.loyalty.award")
	award := sub(t, doc, "components", "schemas", "rpc.request.acme.loyalty.award").(map[string]any)
	require.Equal(t, []any{"kind", "resource", "input"}, award["required"])
	require.Contains(t, award["description"], "Loyalty.Manage")
	require.Equal(t, map[string]any{"const": "command"}, sub(t, doc, "components", "schemas", "rpc.request.acme.loyalty.award", "properties", "kind"))
	require.Equal(t, map[string]any{"const": "atomic"}, sub(t, doc, "components", "schemas", "rpc.request.acme.loyalty.settle", "properties", "kind"))
	require.Equal(t, map[string]any{"const": "query"}, sub(t, doc, "components", "schemas", "rpc.request.acme.loyalty.tiers", "properties", "kind"))
}

func TestContractsFollowTheReaderVisibility(t *testing.T) {
	t.Parallel()
	doc, raw := contractDoc(t, reader("en"))
	req := sub(t, doc, "components", "schemas", "RpcRequest").(map[string]any)
	require.Len(t, req["oneOf"], 2, "only the open queries")
	require.NotContains(t, string(raw), "acme.loyalty.award")
	paths := sub(t, doc, "paths").(map[string]any)
	require.Equal(t, []string{"/_ext/rpc"}, sortedKeys(paths), "package endpoints are listed for Extensions.Read only")

	doc, _ = contractDoc(t, spi.Subject{})
	paths = sub(t, doc, "paths").(map[string]any)
	require.NotContains(t, paths, "/_ext/endpoints/acme.loyalty/export")
	require.NotContains(t, paths, "/_ext/endpoints/acme.loyalty/webhooks/stripe")
}

func sortedKeys(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func TestRPCRequestSchemaAcceptsExamplesAndRejectsStrangers(t *testing.T) {
	t.Parallel()
	doc, _ := contractDoc(t, admin())
	m, err := newBuilder(t).Model(context.Background(), admin(), "en")
	require.NoError(t, err)
	award, _ := m.Lookup("acme.loyalty.award")

	// compile RpcRequest inside the whole document so its refs resolve
	sch := compileAt(t, doc, "#/components/schemas/RpcRequest")
	valid := `{"kind":"command","resource":"acme.loyalty.award","input":` + string(award.Input.Example) + `}`
	require.NoError(t, validates(t, sch, valid))
	for name, bad := range map[string]string{
		"wrong kind for the resource": `{"kind":"query","resource":"acme.loyalty.award","input":` + string(award.Input.Example) + `}`,
		"unknown resource":            `{"kind":"command","resource":"acme.loyalty.nope","input":{}}`,
		"invalid input":               `{"kind":"command","resource":"acme.loyalty.award","input":{"card":1,"points":"x"}}`,
		"missing input":               `{"kind":"command","resource":"acme.loyalty.award"}`,
		"operation is not callable":   `{"kind":"command","resource":"crm.operations.addNote","input":{"text":"x"}}`,
	} {
		require.Error(t, validates(t, sch, bad), name)
	}

	// success envelopes carry the output schema
	resp := compileAt(t, doc, "#/components/schemas/RpcResponse")
	require.NoError(t, validates(t, resp, `{"ok":true,"value":{"balance":"10.50"},"invocation":"i"}`))
	require.NoError(t, validates(t, resp, `{"ok":false,"error":{"kind":"domain","code":"acme.loyalty.notRegistered","params":{"id":"123e4567-e89b-12d3-a456-426614174000"}}}`))
	require.Error(t, validates(t, resp, `{"ok":true,"value":{"balance":10.5}}`), "decimal is a string")
	require.Error(t, validates(t, resp, `{"ok":false,"error":{"kind":"mystery","code":"x"}}`))
}

func newOpenAPIBuilder() *openapi.Builder {
	return openapi.New(openapi.Info{Title: "t", Version: "1"})
}

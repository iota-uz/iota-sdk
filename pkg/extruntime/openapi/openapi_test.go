package openapi_test

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/openapi"
)

type Base struct {
	ID uuid.UUID `json:"id"`
}

type Node struct {
	Base
	Name     string            `json:"name"`
	Note     string            `json:"note,omitempty"`
	Parent   *Node             `json:"parent,omitempty"`
	Children []Node            `json:"children"`
	At       time.Time         `json:"at"`
	Raw      json.RawMessage   `json:"raw"`
	Labels   map[string]string `json:"labels,omitempty"`
	Big      int64             `json:"big,string"`
	Owner    *Base             `json:"owner"`
	Data     []byte            `json:"data,omitempty"`
	Hidden   string            `json:"-"`
}

func TestRegistryDerivesTheJSONShapeOfAGoType(t *testing.T) {
	t.Parallel()
	reg := openapi.NewRegistry()
	ref := openapi.Of[Node](reg)
	require.Equal(t, "#/components/schemas/Node", ref["$ref"])

	node := reg.Schemas()["Node"]
	props := node["properties"].(openapi.Schema)
	require.Contains(t, props, "id", "embedded structs are flattened")
	require.NotContains(t, props, "Hidden")
	require.NotContains(t, props, "private")
	require.Equal(t, openapi.Schema{"type": "string", "format": "date-time"}, props["at"])
	require.Equal(t, openapi.Schema{}, props["raw"])
	require.Equal(t, openapi.Schema{"type": "string"}, props["big"], "the string option makes a number a string")
	require.Equal(t, openapi.Schema{"type": "string", "contentEncoding": "base64"}, props["data"])
	require.Equal(t, openapi.Ref("Node"), props["parent"], "recursive types refer to themselves")
	require.Contains(t, props["owner"], "oneOf", "a pointer without omitempty is required but nullable")
	require.Equal(t, []any{"at", "big", "children", "id", "name", "owner", "raw"}, node["required"])
}

func TestBuilderRendersAValidDocument(t *testing.T) {
	t.Parallel()
	b := openapi.New(openapi.Info{Title: "t", Version: "1", SessionCookie: "session"})
	b.Tag("x", "things")
	b.ErrorEnvelope("An error.", b.Schemas().Add("Err", openapi.Schema{"type": "object"}))
	b.Add(openapi.Operation{
		Method: http.MethodGet, Path: "/a/{id}", ID: "getA", Summary: "Get A", Permission: "A.Read", Tags: []string{"x"},
		Params:    []openapi.Param{{Name: "id", In: "path", Required: true}, {Name: "q", In: "query"}},
		Responses: []openapi.Response{{Status: 200, Schema: openapi.Of[Node](b.Schemas())}},
	})
	b.Add(openapi.Operation{Method: http.MethodPost, Path: "/a/{id}", ID: "postA", Public: true,
		Params:  []openapi.Param{{Name: "id", In: "path"}},
		Request: &openapi.Body{Schema: openapi.Of[Base](b.Schemas()), Required: true}, Responses: []openapi.Response{{Status: 204}}})
	raw, err := b.JSON()
	require.NoError(t, err)
	require.NoError(t, openapi.Validate(raw))

	var doc map[string]any
	require.NoError(t, json.Unmarshal(raw, &doc))
	get := doc["paths"].(map[string]any)["/a/{id}"].(map[string]any)["get"].(map[string]any)
	require.Equal(t, "A.Read", get["x-required-permission"])
	require.Contains(t, get["description"], "`A.Read`")
	require.Equal(t, []any{map[string]any{"sessionCookie": []any{}}}, get["security"])
	require.Equal(t, "session", doc["components"].(map[string]any)["securitySchemes"].(map[string]any)["sessionCookie"].(map[string]any)["name"])
	post := doc["paths"].(map[string]any)["/a/{id}"].(map[string]any)["post"].(map[string]any)
	require.Equal(t, []any{}, post["security"])
	require.Contains(t, get["responses"], "default")
}

func TestBuilderRefusesDuplicates(t *testing.T) {
	t.Parallel()
	b := openapi.New(openapi.Info{Title: "t", Version: "1"})
	b.Add(openapi.Operation{Method: "GET", Path: "/a", ID: "a"})
	b.Add(openapi.Operation{Method: "GET", Path: "/a", ID: "b"})
	_, err := b.JSON()
	require.ErrorContains(t, err, "declared twice")

	b = openapi.New(openapi.Info{Title: "t", Version: "1"})
	b.Add(openapi.Operation{Method: "GET", Path: "/a", ID: "same"})
	b.Add(openapi.Operation{Method: "GET", Path: "/b", ID: "same"})
	_, err = b.JSON()
	require.ErrorContains(t, err, "operationId")
}

func TestValidateRejectsBrokenDocuments(t *testing.T) {
	t.Parallel()
	for name, doc := range map[string]string{
		"not json":           `{`,
		"wrong version":      `{"openapi":"3.0.0","paths":{}}`,
		"dangling ref":       `{"openapi":"3.1.0","paths":{"/a":{"get":{"responses":{"200":{"description":"x","content":{"application/json":{"schema":{"$ref":"#/components/schemas/Nope"}}}}}}}},"components":{}}`,
		"bad schema":         `{"openapi":"3.1.0","paths":{},"components":{"schemas":{"S":{"type":"nope"}}}}`,
		"undeclared param":   `{"openapi":"3.1.0","paths":{"/a/{id}":{"get":{"responses":{"200":{"description":"x"}}}}}}`,
		"no responses":       `{"openapi":"3.1.0","paths":{"/a":{"get":{"summary":"x"}}}}`,
		"inline schema typo": `{"openapi":"3.1.0","paths":{"/a":{"post":{"requestBody":{"content":{"application/json":{"schema":{"minimum":"x"}}}},"responses":{"200":{"description":"x"}}}}}}`,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			require.Error(t, openapi.Validate([]byte(doc)))
		})
	}
}

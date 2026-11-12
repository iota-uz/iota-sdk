package reference

import (
	"net/http"
	"sort"
	"strings"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/openapi"
)

// OpenAPI tags of the contract operations.
const (
	TagRPC       = "rpc"
	TagEndpoints = "endpoints"
)

// ErrorKinds are the `kind` values of an RPC error.
var ErrorKinds = []any{"domain", "validation", "denied", "internal"}

// AddContracts adds the parts of the HTTP API that the reference describes to
// the OpenAPI document: the browser RPC (POST /_ext/rpc) with one request
// variant and one value schema per command and query the model holds, and the
// HTTP endpoints of the packages in the model. What the model omits for its
// reader is absent here too.
func AddContracts(b *openapi.Builder, m *Model) {
	b.Tag(TagRPC, "Browser transport that calls commands and queries as the signed-in user.")
	b.Tag(TagEndpoints, "HTTP endpoints published by activated extension packages.")
	addRPC(b, m)
	addEndpoints(b, m)
}

func addRPC(b *openapi.Builder, m *Model) {
	reg := b.Schemas()
	issue := reg.Add("RpcIssue", openapi.Schema{
		"type": "object",
		"properties": openapi.Schema{
			"code": openapi.Schema{"type": "string"}, "phase": openapi.Schema{"type": "string", "enum": []any{"input", "write"}},
			"source": openapi.Schema{"type": "string"}, "path": openapi.Schema{"type": "array", "items": openapi.Schema{}},
			"params": openapi.Schema{},
		},
		"required": []any{"code"},
	})
	rpcErr := reg.Add("RpcError", openapi.Schema{
		"type": "object",
		"properties": openapi.Schema{
			"kind":    openapi.Schema{"type": "string", "enum": ErrorKinds},
			"code":    openapi.Schema{"type": "string", "description": "Stable error code: a declared domain error, an issue code or a runtime code."},
			"message": openapi.Schema{"type": "string"},
			"params":  openapi.Schema{"description": "Parameters of a declared domain error."},
			"issues":  openapi.Schema{"type": "array", "items": issue},
		},
		"required": []any{"kind", "code"},
	})
	failure := reg.Add("RpcFailure", openapi.Schema{
		"type": "object",
		"properties": openapi.Schema{
			"ok":         openapi.Schema{"const": false},
			"error":      rpcErr,
			"invocation": openapi.Schema{"type": "string", "description": "The recorded invocation, for support."},
		},
		"required": []any{"ok", "error"},
	})

	generic := reg.Add("RpcSuccess", openapi.Schema{
		"type": "object",
		"properties": openapi.Schema{
			"ok": openapi.Schema{"const": true}, "value": openapi.Schema{"description": "The contract's output; see the request variant of the resource."},
			"invocation": openapi.Schema{"type": "string", "description": "The recorded invocation, for support."},
		},
		"required": []any{"ok", "value"},
	})
	var variants []any
	var successes []any
	mapping := map[string]any{}
	for _, ns := range m.Namespaces {
		for _, it := range ns.Items {
			kind := rpcKind(it)
			if kind == "" {
				continue
			}
			req := "rpc.request." + it.ID
			props := openapi.Schema{
				"kind":     openapi.Schema{"const": kind},
				"resource": openapi.Schema{"const": it.ID},
			}
			required := []any{"kind", "resource"}
			if it.Input != nil {
				props["input"] = it.Input.Schema
				if !nullableType(it.Input.Schema) {
					required = append(required, "input")
				}
			}
			ref := reg.Add(req, openapi.Schema{
				"title": it.ID, "description": contractDescription(it),
				"type": "object", "properties": props, "required": required, "additionalProperties": false,
			})
			variants = append(variants, ref)
			mapping[it.ID] = ref["$ref"]

			value := openapi.Schema{"type": "null"}
			if it.Output != nil {
				value = it.Output.Schema
			}
			valRef := reg.Add("rpc.value."+it.ID, value)
			successes = append(successes, reg.Add("rpc.response."+it.ID, openapi.Schema{
				"title": it.ID, "type": "object",
				"properties": openapi.Schema{
					"ok": openapi.Schema{"const": true}, "value": valRef,
					"invocation": openapi.Schema{"type": "string", "description": "The recorded invocation, for support."},
				},
				"required": []any{"ok", "value"},
			}))
		}
	}

	request := openapi.Schema{
		"type": "object",
		"properties": openapi.Schema{
			"kind":     openapi.Schema{"type": "string", "enum": []any{"query", "command", "atomic"}},
			"resource": openapi.Schema{"type": "string", "description": "The contract ID."},
			"input":    openapi.Schema{"description": "The input of the contract."},
		},
		"required": []any{"kind", "resource"}, "additionalProperties": false,
	}
	if len(successes) == 0 {
		successes = []any{generic}
	}
	response := openapi.Schema{"anyOf": append(append([]any(nil), successes...), failure)}
	if len(variants) > 0 {
		request = openapi.Schema{"oneOf": variants, "discriminator": openapi.Schema{"propertyName": "resource", "mapping": mapping}}
	}
	reqRef := reg.Add("RpcRequest", request)
	respRef := reg.Add("RpcResponse", response)

	statuses := []struct {
		code int
		text string
	}{
		{http.StatusBadRequest, "Malformed request."},
		{http.StatusUnauthorized, "No session."},
		{http.StatusForbidden, "The user lacks the permission, the resource cannot be called from the browser, or the request is cross-site."},
		{http.StatusNotFound, "Unknown resource."},
		{http.StatusConflict, "No implementation is active for the contract."},
		{http.StatusRequestEntityTooLarge, "The request body exceeds 1 MiB."},
		{http.StatusUnprocessableEntity, "A declared domain error or a validation failure; the error envelope carries the code, params and issues."},
		{http.StatusInternalServerError, "Internal error."},
		{http.StatusServiceUnavailable, "The extension runtime is not available."},
	}
	responses := []openapi.Response{{Status: http.StatusOK, Description: "The value the contract returned.", Schema: respRef}}
	for _, s := range statuses {
		responses = append(responses, openapi.Response{Status: s.code, Description: s.text, Schema: failure})
	}
	b.Add(openapi.Operation{
		Method: http.MethodPost, Path: RPCPath, ID: "extensions.rpc", Tags: []string{TagRPC},
		Summary: "Call a command or query",
		Description: "Invokes a command or query as the signed-in user, with the permissions the user holds now. " +
			"Same-origin browser transport: cross-site requests are refused. Operations are internal and cannot be called. " +
			"The request is one variant per callable contract, discriminated by `resource`.",
		Request:   &openapi.Body{Schema: reqRef, Required: true},
		Responses: responses,
	})
}

// nullableType reports whether a generated schema accepts null.
func nullableType(s openapi.Schema) bool {
	switch ty := s["type"].(type) {
	case []any:
		for _, x := range ty {
			if x == "null" {
				return true
			}
		}
	case nil:
		_, strict := s["not"]
		return !strict
	}
	return false
}

func contractDescription(it Item) string {
	var parts []string
	if it.Label != "" {
		parts = append(parts, it.Label+".")
	}
	if it.Description != "" {
		parts = append(parts, it.Description)
	}
	if it.Permission != "" {
		parts = append(parts, "Requires the permission `"+it.Permission+"`.")
	}
	if it.Package != "" {
		parts = append(parts, "Package "+it.Package+" "+it.Version+".")
	}
	if len(it.Errors) > 0 {
		parts = append(parts, "Declared errors: "+strings.Join(it.Errors, ", ")+".")
	}
	return strings.Join(parts, " ")
}

func addEndpoints(b *openapi.Builder, m *Model) {
	var all []EndpointDoc
	for _, ns := range m.Namespaces {
		all = append(all, ns.Endpoints...)
	}
	sort.SliceStable(all, func(i, j int) bool {
		if all[i].URL != all[j].URL {
			return all[i].URL < all[j].URL
		}
		return all[i].Method < all[j].Method
	})
	for _, e := range all {
		method := strings.ToUpper(e.Method)
		desc := "Endpoint " + e.ID + " of package " + e.Package + " " + e.Version + ". "
		if e.Public {
			desc += "Public: no session is needed; it runs as the trigger owner assigned for the tenant. "
		} else {
			desc += "Runs as the signed-in user. "
		}
		desc += "Endpoint specs declare no request or response schema: the package defines the body, and the handler's status, headers and body are returned as they are."
		op := openapi.Operation{
			Method: method, Path: e.URL, ID: "endpoint." + e.ID, Tags: []string{TagEndpoints},
			Summary: method + " " + e.Path, Description: desc, Public: e.Public,
			Responses: []openapi.Response{{Status: http.StatusOK, Description: "Whatever the handler returns (any status, headers and body)."}},
		}
		if method != http.MethodGet && method != http.MethodHead && method != http.MethodDelete {
			op.Request = &openapi.Body{ContentType: "*/*", Schema: openapi.Schema{}, Description: "Untyped: defined by the package."}
		}
		b.Add(op)
	}
}

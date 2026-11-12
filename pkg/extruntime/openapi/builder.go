// Package openapi builds OpenAPI 3.1 documents: a small builder over the
// operations of the extension runtime (the management API, the browser RPC,
// package endpoints) and a Registry that derives JSON Schemas from the Go
// types of the wire. OpenAPI 3.1 uses the JSON Schema 2020-12 dialect, so the
// contract schemas of the reference embed unchanged.
package openapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
)

// Version is the OpenAPI version of the generated documents.
const Version = "3.1.0"

// SchemaDialect is the JSON Schema dialect OpenAPI 3.1 documents declare.
const SchemaDialect = "https://json-schema.org/draft/2020-12/schema"

// SessionScheme is the security scheme of every operation that needs a session.
const SessionScheme = "sessionCookie"

// Info describes the API.
type Info struct {
	Title       string
	Version     string
	Description string
	// SessionCookie is the name of the session cookie (default "sid").
	SessionCookie string
}

// Param is a path, query or header parameter.
type Param struct {
	Name        string
	In          string // path | query | header
	Description string
	Required    bool
	Schema      Schema
}

// Body is a request body.
type Body struct {
	ContentType string // default application/json
	Schema      Schema
	Description string
	Required    bool
}

// Response is one response of an operation.
type Response struct {
	Status      int
	Description string
	ContentType string // default application/json; empty schema: no content
	Schema      Schema
	Headers     map[string]string // name -> description
}

// Operation is one HTTP operation.
type Operation struct {
	Method      string
	Path        string // OpenAPI form: /_ext/manage/packages/{ns}
	ID          string
	Summary     string
	Description string
	Tags        []string
	Params      []Param
	Request     *Body
	Responses   []Response
	// Public operations need no session; every other one needs the session
	// cookie.
	Public bool
	// Permission is the permission the operation requires, documented as
	// x-required-permission ("" when a session is enough).
	Permission string
	// Extensions are extra "x-" members of the operation.
	Extensions map[string]any
}

// Builder collects operations and components into a document.
type Builder struct {
	info    Info
	ops     []Operation
	tags    map[string]string
	schemas *Registry
	errSch  Schema
	errDesc string
	servers []string
}

// New returns a builder for the API.
func New(info Info) *Builder {
	if info.SessionCookie == "" {
		info.SessionCookie = "sid"
	}
	return &Builder{info: info, tags: map[string]string{}, schemas: NewRegistry()}
}

// Schemas is the component registry of the document.
func (b *Builder) Schemas() *Registry { return b.schemas }

// Tag declares a tag with its description.
func (b *Builder) Tag(name, description string) { b.tags[name] = description }

// Server adds a server URL (default: relative to the document's origin).
func (b *Builder) Server(url string) { b.servers = append(b.servers, url) }

// ErrorEnvelope sets the schema of the error body every operation may answer
// with (the "default" response).
func (b *Builder) ErrorEnvelope(description string, s Schema) { b.errDesc, b.errSch = description, s }

// Add appends an operation.
func (b *Builder) Add(op Operation) { b.ops = append(b.ops, op) }

// Operations lists the added operations.
func (b *Builder) Operations() []Operation { return append([]Operation(nil), b.ops...) }

// Document renders the OpenAPI document.
func (b *Builder) Document() (map[string]any, error) {
	paths := map[string]any{}
	seenIDs := map[string]bool{}
	for _, op := range b.ops {
		method := strings.ToLower(op.Method)
		item, _ := paths[op.Path].(map[string]any)
		if item == nil {
			item = map[string]any{}
			paths[op.Path] = item
		}
		if _, dup := item[method]; dup {
			return nil, fmt.Errorf("openapi: %s %s declared twice", op.Method, op.Path)
		}
		if op.ID != "" {
			if seenIDs[op.ID] {
				return nil, fmt.Errorf("openapi: operationId %q declared twice", op.ID)
			}
			seenIDs[op.ID] = true
		}
		item[method] = b.render(op)
	}
	components := map[string]any{
		"securitySchemes": map[string]any{
			SessionScheme: map[string]any{
				"type": "apiKey", "in": "cookie", "name": b.info.SessionCookie,
				"description": "The browser session cookie. State-changing requests must come from the application's own origin.",
			},
		},
	}
	if schemas := b.schemas.Schemas(); len(schemas) > 0 {
		components["schemas"] = schemas
	}
	if b.errSch != nil {
		components["responses"] = map[string]any{"Error": map[string]any{
			"description": b.errDesc,
			"content":     map[string]any{"application/json": map[string]any{"schema": b.errSch}},
		}}
	}
	info := map[string]any{"title": b.info.Title, "version": b.info.Version}
	if b.info.Description != "" {
		info["description"] = b.info.Description
	}
	doc := map[string]any{
		"openapi":           Version,
		"jsonSchemaDialect": SchemaDialect,
		"info":              info,
		"paths":             paths,
		"components":        components,
	}
	if len(b.servers) > 0 {
		servers := make([]any, len(b.servers))
		for i, s := range b.servers {
			servers[i] = map[string]any{"url": s}
		}
		doc["servers"] = servers
	}
	if len(b.tags) > 0 {
		names := make([]string, 0, len(b.tags))
		for n := range b.tags {
			names = append(names, n)
		}
		sort.Strings(names)
		tags := make([]any, len(names))
		for i, n := range names {
			tags[i] = map[string]any{"name": n, "description": b.tags[n]}
		}
		doc["tags"] = tags
	}
	return doc, nil
}

// JSON renders the document as indented JSON without HTML escaping.
func (b *Builder) JSON() ([]byte, error) {
	doc, err := b.Document()
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(doc); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (b *Builder) render(op Operation) map[string]any {
	o := map[string]any{}
	if op.ID != "" {
		o["operationId"] = op.ID
	}
	if op.Summary != "" {
		o["summary"] = op.Summary
	}
	desc := op.Description
	if op.Permission != "" {
		note := "Requires the permission `" + op.Permission + "`."
		if desc != "" {
			desc += "\n\n"
		}
		desc += note
		o["x-required-permission"] = op.Permission
	}
	if desc != "" {
		o["description"] = desc
	}
	if len(op.Tags) > 0 {
		o["tags"] = op.Tags
	}
	if len(op.Params) > 0 {
		params := make([]any, len(op.Params))
		for i, p := range op.Params {
			pm := map[string]any{"name": p.Name, "in": p.In, "required": p.Required || p.In == "path"}
			if p.Description != "" {
				pm["description"] = p.Description
			}
			if p.Schema != nil {
				pm["schema"] = p.Schema
			} else {
				pm["schema"] = Schema{"type": "string"}
			}
			params[i] = pm
		}
		o["parameters"] = params
	}
	if op.Request != nil {
		ct := op.Request.ContentType
		if ct == "" {
			ct = "application/json"
		}
		body := map[string]any{"required": op.Request.Required}
		if op.Request.Description != "" {
			body["description"] = op.Request.Description
		}
		media := map[string]any{}
		if op.Request.Schema != nil {
			media["schema"] = op.Request.Schema
		}
		body["content"] = map[string]any{ct: media}
		o["requestBody"] = body
	}
	responses := map[string]any{}
	for _, r := range op.Responses {
		resp := map[string]any{"description": r.Description}
		if resp["description"] == "" {
			resp["description"] = http.StatusText(r.Status)
		}
		if r.Schema != nil {
			ct := r.ContentType
			if ct == "" {
				ct = "application/json"
			}
			resp["content"] = map[string]any{ct: map[string]any{"schema": r.Schema}}
		}
		if len(r.Headers) > 0 {
			headers := map[string]any{}
			for name, d := range r.Headers {
				headers[name] = map[string]any{"description": d, "schema": Schema{"type": "string"}}
			}
			resp["headers"] = headers
		}
		responses[strconv.Itoa(r.Status)] = resp
	}
	if b.errSch != nil {
		responses["default"] = map[string]any{"$ref": "#/components/responses/Error"}
	}
	if len(responses) == 0 {
		responses["200"] = map[string]any{"description": "OK"}
	}
	o["responses"] = responses
	if op.Public {
		o["security"] = []any{}
	} else {
		o["security"] = []any{map[string]any{SessionScheme: []any{}}}
	}
	for k, v := range op.Extensions {
		o[k] = v
	}
	return o
}

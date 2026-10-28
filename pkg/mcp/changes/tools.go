package changes

import (
	"context"
	"encoding/json"
	"slices"
	"strings"

	"github.com/google/jsonschema-go/jsonschema"

	"github.com/iota-uz/iota-sdk/pkg/mcp"
)

const (
	ToolPrepare      = "prepare_changes"
	ToolApply        = "apply_changes"
	ToolOperationGet = "operation_get"
)

// Tools exposes the service through the shared registry. The change union in
// the prepare_changes schema is generated from the registered kinds.
func (s *Service) Tools() []mcp.Tool {
	return []mcp.Tool{
		&tool{spec: mcp.Spec{
			Name: ToolPrepare,
			Description: "Validate a batch of typed changes without writing anything and store an immutable plan. " +
				"Returns plan_id and plan_hash to pass to apply_changes, or the per-change errors.",
			Input:  s.prepareSchema(),
			Policy: s.cfg.ToolPolicy,
		}, call: s.callPrepare},
		&tool{spec: mcp.Spec{
			Name: ToolApply,
			Description: "Apply a prepared plan exactly once per idempotency_key. Only the plan reference is accepted, never a new payload. " +
				"On a lost response call operation_get with the same key instead of repeating the work.",
			Input: &jsonschema.Schema{
				Type:                 "object",
				AdditionalProperties: falseSchema(),
				Required:             []string{"plan_id", "plan_hash", "idempotency_key"},
				Properties: map[string]*jsonschema.Schema{
					"plan_id":         {Type: "string", MinLength: jsonschema.Ptr(1)},
					"plan_hash":       {Type: "string", MinLength: jsonschema.Ptr(1)},
					"idempotency_key": {Type: "string", MinLength: jsonschema.Ptr(1), MaxLength: jsonschema.Ptr(255)},
					"approval_ref":    {Type: "string", Description: "Reference issued by the approval page after a person approved the plan."},
				},
			},
			Annotations: mcp.Annotations{Destructive: true, Idempotent: true},
			Policy:      s.cfg.ToolPolicy,
		}, call: s.callApply},
		&tool{spec: mcp.Spec{
			Name:        ToolOperationGet,
			Description: "Read the state and receipt of an apply by exactly one of operation_id or idempotency_key.",
			Input: &jsonschema.Schema{
				Type:                 "object",
				AdditionalProperties: falseSchema(),
				Properties: map[string]*jsonschema.Schema{
					"operation_id":    {Type: "string", MinLength: jsonschema.Ptr(1)},
					"idempotency_key": {Type: "string", MinLength: jsonschema.Ptr(1)},
				},
				OneOf: []*jsonschema.Schema{{Required: []string{"operation_id"}}, {Required: []string{"idempotency_key"}}},
			},
			Annotations: mcp.Annotations{ReadOnly: true, Idempotent: true},
			Policy:      s.cfg.ToolPolicy,
		}, call: s.callOperationGet},
	}
}

func (s *Service) prepareSchema() *jsonschema.Schema {
	keys := make([]string, 0, len(s.kinds))
	for key := range s.kinds {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	defs := make(map[string]*jsonschema.Schema, len(keys))
	branches := make([]*jsonschema.Schema, 0, len(keys))
	ops := map[string]bool{}
	entities := map[string]bool{}
	for _, key := range keys {
		k := s.kinds[key]
		ops[k.Op()], entities[k.Entity()] = true, true
		variant := &jsonschema.Schema{
			Description:          k.Description(),
			Type:                 "object",
			AdditionalProperties: falseSchema(),
			Required:             []string{"op", "entity"},
			Properties: map[string]*jsonschema.Schema{
				"op":     {Const: jsonValue(k.Op())},
				"entity": {Const: jsonValue(k.Entity())},
				"ref":    {Type: "string", Description: "Caller label echoed in the receipt."},
			},
		}
		if k.Existing() {
			variant.Required = append(variant.Required, "id", "expected_revision")
			variant.Properties["id"] = &jsonschema.Schema{Type: "string", MinLength: jsonschema.Ptr(1)}
			variant.Properties["expected_revision"] = &jsonschema.Schema{Type: "string", MinLength: jsonschema.Ptr(1)}
		}
		if fields := k.Fields(); fields != nil {
			variant.Required = append(variant.Required, "fields")
			variant.Properties["fields"] = fields
		}
		defs[key] = variant
		branches = append(branches, &jsonschema.Schema{
			If: &jsonschema.Schema{
				Required: []string{"op", "entity"},
				Properties: map[string]*jsonschema.Schema{
					"op":     {Const: jsonValue(k.Op())},
					"entity": {Const: jsonValue(k.Entity())},
				},
			},
			Then: &jsonschema.Schema{Ref: "#/$defs/" + key},
		})
	}
	item := &jsonschema.Schema{
		Type:     "object",
		Required: []string{"op", "entity"},
		Properties: map[string]*jsonschema.Schema{
			"op":     {Enum: enum(ops)},
			"entity": {Enum: enum(entities)},
		},
		AllOf: branches,
	}
	return &jsonschema.Schema{
		Type:                 "object",
		Defs:                 defs,
		AdditionalProperties: falseSchema(),
		Required:             []string{"changes"},
		Properties: map[string]*jsonschema.Schema{
			"changes": {Type: "array", MinItems: jsonschema.Ptr(1), MaxItems: jsonschema.Ptr(s.cfg.MaxChanges), Items: item},
			"reason":  {Type: "string", MaxLength: jsonschema.Ptr(2000)},
		},
	}
}

type tool struct {
	spec mcp.Spec
	call func(context.Context, json.RawMessage) (any, error)
}

func (t *tool) Spec() mcp.Spec { return t.spec }

func (t *tool) Call(ctx context.Context, input json.RawMessage) (any, error) {
	return t.call(ctx, input)
}

func (s *Service) callPrepare(ctx context.Context, input json.RawMessage) (any, error) {
	var in struct {
		Changes []Change `json:"changes"`
		Reason  string   `json:"reason"`
	}
	if err := json.Unmarshal(input, &in); err != nil {
		return nil, mcp.Fail(mcp.CodeInvalidInput, mcp.RecoveryFixInput, "input does not match the tool schema")
	}
	return s.Prepare(ctx, in.Changes, in.Reason)
}

func (s *Service) callApply(ctx context.Context, input json.RawMessage) (any, error) {
	var in ApplyInput
	if err := json.Unmarshal(input, &in); err != nil {
		return nil, mcp.Fail(mcp.CodeInvalidInput, mcp.RecoveryFixInput, "input does not match the tool schema")
	}
	return s.Apply(ctx, in)
}

func (s *Service) callOperationGet(ctx context.Context, input json.RawMessage) (any, error) {
	var in struct {
		OperationID    string `json:"operation_id"`
		IdempotencyKey string `json:"idempotency_key"`
	}
	if err := json.Unmarshal(input, &in); err != nil {
		return nil, mcp.Fail(mcp.CodeInvalidInput, mcp.RecoveryFixInput, "input does not match the tool schema")
	}
	return s.OperationGet(ctx, strings.TrimSpace(in.OperationID), strings.TrimSpace(in.IdempotencyKey))
}

func falseSchema() *jsonschema.Schema { return &jsonschema.Schema{Not: &jsonschema.Schema{}} }

func jsonValue(v any) *any { return &v }

func enum(set map[string]bool) []any {
	values := make([]string, 0, len(set))
	for v := range set {
		values = append(values, v)
	}
	slices.Sort(values)
	out := make([]any, len(values))
	for i, v := range values {
		out[i] = v
	}
	return out
}

// Package graph is the automation graph format: one JSON document is the source of truth of a graph
// version. It holds the format (Graph, Decode, JSONSchema), the catalog types a product registers (events,
// node types, subjects), the save-time validator and the pure interpreter of one step and of dry run.
//
// The package is pure: no database, no IO, no SDK modules. Engines, editors and catalogs build on it.
package graph

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"regexp"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// Graph is the content of one immutable version. Its id, version number, enabled flag and layout are kept
// by its owner, outside the version.
type Graph struct {
	Name        string    `json:"name"`                  // 1..120 after trim
	Description string    `json:"description,omitempty"` // ≤ 2000
	RunAs       RunAs     `json:"run_as"`                // owner (default) | initiator
	Triggers    []Trigger `json:"triggers"`              // 1..40
	Nodes       []Node    `json:"nodes"`                 // ≤ 200
	Edges       []Edge    `json:"edges"`                 // ≤ 500
}

// RunAs is whose rights a run has.
type RunAs string

const (
	RunAsOwner     RunAs = "owner"
	RunAsInitiator RunAs = "initiator"
)

// Trigger starts a run on an event of the catalog.
type Trigger struct {
	ID     string         `json:"id"`               // ^[a-z][a-z0-9_]{0,39}$
	Event  string         `json:"event"`            // an event key of the catalog
	Filter string         `json:"filter,omitempty"` // CEL bool, ≤ 2000
	Params map[string]any `json:"params,omitempty"` // by EventSpec.TriggerParams or Match
}

// Node is one step of the graph: a node type of the catalog with its params.
type Node struct {
	ID     string         `json:"id"` // the trigger id pattern; one namespace with triggers
	Type   string         `json:"type"`
	Params map[string]any `json:"params"`
}

// Edge connects an outcome of a node (or a trigger's `fired`) to a node or to `ignore`.
type Edge struct {
	From EdgeFrom `json:"from"`
	To   string   `json:"to"` // a node id or "ignore"
}

// EdgeFrom is the outcome an edge leaves from.
type EdgeFrom struct {
	Node    string `json:"node"`    // a node or trigger id
	Outcome string `json:"outcome"` // "fired" for a trigger
}

// Layout is where an editor draws each node and trigger, by id. Not part of a version: moving a node makes
// no version.
type Layout map[string]XY

// XY is a place on the canvas.
type XY struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

const (
	// Ignore is the edge target that ends a run on an outcome on purpose.
	Ignore = "ignore"
	// TriggerOutcome is the one outcome of a trigger.
	TriggerOutcome = "fired"
	// MaxLayoutPlaces is the most places a layout holds: one per node (200) and trigger (40).
	MaxLayoutPlaces = 240
)

var idRe = regexp.MustCompile(`^[a-z][a-z0-9_]{0,39}$`)

// IsID reports whether s is a valid node, trigger or outcome id.
func IsID(s string) bool { return idRe.MatchString(s) }

// GraphError is one reason a graph, a layout or a catalog is refused.
type GraphError struct {
	Path    string `json:"path"` // "nodes[2].params.expression", "edges[0].to", "graph"
	Code    string `json:"code"`
	Message string `json:"message"`
	Line    int    `json:"line,omitempty"` // for CEL: the position inside the expression
	Column  int    `json:"column,omitempty"`
}

func (e GraphError) Error() string { return e.Path + ": " + e.Message }

// Error codes. The list is open: behaviors and product rules add their own.
const (
	CodeInvalid             = "invalid"
	CodeUnknownEvent        = "unknown_event"
	CodeUnknownNodeType     = "unknown_node_type"
	CodeUnsupportedBehavior = "unsupported_behavior"
	CodeUnknownOutcome      = "unknown_outcome"
	CodeDuplicateID         = "duplicate_id"
	CodeUnconnected         = "unconnected"
	CodeUnreachable         = "unreachable"
	CodeCycleWithoutWait    = "cycle_without_wait"
	CodeExpressionType      = "expression_type"
	CodeExpressionSyntax    = "expression_syntax"
	CodeParams              = "params"
	CodeLimit               = "limit"
)

const graphSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "title": "Automation graph",
  "description": "The content of one immutable version of an automation graph.",
  "type": "object",
  "properties": {
    "name": {"type": "string", "pattern": "^[\\t\\n\\v\\f\\r \\x{85}\\p{Z}]*[^\\t\\n\\v\\f\\r \\x{85}\\p{Z}](?:[\\s\\S]{0,118}[^\\t\\n\\v\\f\\r \\x{85}\\p{Z}])?[\\t\\n\\v\\f\\r \\x{85}\\p{Z}]*$", "description": "1 to 120 characters (code points) once whitespace at the edges is trimmed (Unicode White_Space, as Go strings.TrimSpace); the graph keeps the trimmed name. The pattern is RE2 syntax."},
    "description": {"type": "string", "maxLength": 2000},
    "run_as": {"enum": ["owner", "initiator"], "default": "owner"},
    "triggers": {
      "type": "array", "minItems": 1, "maxItems": 40,
      "items": {
        "type": "object",
        "properties": {
          "id": {"$ref": "#/$defs/id"},
          "event": {"$ref": "#/$defs/key"},
          "filter": {"type": "string", "maxLength": 2000, "description": "CEL of type bool."},
          "params": {"type": "object", "description": "By the event's trigger_params or match fields."}
        },
        "required": ["id", "event"],
        "additionalProperties": false
      }
    },
    "nodes": {
      "type": "array", "maxItems": 200,
      "items": {
        "type": "object",
        "properties": {
          "id": {"$ref": "#/$defs/id"},
          "type": {"$ref": "#/$defs/key"},
          "params": {"type": "object", "description": "By the node type's params schema."}
        },
        "required": ["id", "type", "params"],
        "additionalProperties": false
      }
    },
    "edges": {
      "type": "array", "maxItems": 500,
      "items": {
        "type": "object",
        "properties": {
          "from": {
            "type": "object",
            "properties": {
              "node": {"$ref": "#/$defs/id"},
              "outcome": {"type": "string", "minLength": 1, "maxLength": 40}
            },
            "required": ["node", "outcome"],
            "additionalProperties": false
          },
          "to": {"$ref": "#/$defs/id", "description": "A node id, or ignore."}
        },
        "required": ["from", "to"],
        "additionalProperties": false
      }
    }
  },
  "required": ["name", "triggers", "nodes", "edges"],
  "additionalProperties": false,
  "$defs": {
    "id": {"type": "string", "pattern": "^[a-z][a-z0-9_]{0,39}$"},
    "key": {"type": "string", "minLength": 1, "maxLength": 120}
  }
}`

// JSONSchema is the graph format as JSON Schema, for clients in other languages.
func JSONSchema() json.RawMessage { return json.RawMessage(graphSchema) }

var (
	compiledGraph     *jsonschema.Schema
	compiledGraphOnce sync.Once
)

func graphValidator() *jsonschema.Schema {
	compiledGraphOnce.Do(func() {
		s, err := compileSchema(json.RawMessage(graphSchema))
		if err != nil {
			panic(fmt.Sprintf("automation graph schema: %v", err))
		}
		compiledGraph = s
	})
	return compiledGraph
}

// Decode reads a graph strictly: unknown keys, wrong types, lengths and limits are errors with their paths.
// Numbers inside params are json.Number.
func Decode(raw []byte) (Graph, []GraphError) {
	var doc any
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	if err := dec.Decode(&doc); err != nil {
		return Graph{}, []GraphError{{Path: "graph", Code: CodeInvalid, Message: "not JSON: " + err.Error()}}
	}
	if _, err := dec.Token(); err != io.EOF {
		return Graph{}, []GraphError{{Path: "graph", Code: CodeInvalid, Message: "unexpected data after the graph"}}
	}
	if errs := structureErrors(doc); len(errs) > 0 {
		return Graph{}, errs
	}
	o := doc.(map[string]any)
	g := Graph{Name: TrimName(o["name"].(string)), RunAs: RunAsOwner}
	if d, ok := o["description"].(string); ok {
		g.Description = d
	}
	if r, ok := o["run_as"].(string); ok {
		g.RunAs = RunAs(r)
	}
	for _, t := range o["triggers"].([]any) {
		tm := t.(map[string]any)
		tr := Trigger{ID: tm["id"].(string), Event: tm["event"].(string)}
		if f, ok := tm["filter"].(string); ok {
			tr.Filter = f
		}
		if p, ok := tm["params"].(map[string]any); ok {
			tr.Params = p
		}
		g.Triggers = append(g.Triggers, tr)
	}
	g.Nodes = []Node{}
	for _, n := range o["nodes"].([]any) {
		nm := n.(map[string]any)
		g.Nodes = append(g.Nodes, Node{ID: nm["id"].(string), Type: nm["type"].(string), Params: nm["params"].(map[string]any)})
	}
	g.Edges = []Edge{}
	for _, e := range o["edges"].([]any) {
		em := e.(map[string]any)
		from := em["from"].(map[string]any)
		g.Edges = append(g.Edges, Edge{From: EdgeFrom{Node: from["node"].(string), Outcome: from["outcome"].(string)}, To: em["to"].(string)})
	}
	if errs := nameErrors(g.Name); len(errs) > 0 {
		return Graph{}, errs
	}
	return g, nil
}

// structureErrors checks a decoded document against the format's schema. A string name is checked by
// nameErrors (invalid when blank, limit when too long), not by the pattern that mirrors that rule for clients.
func structureErrors(doc any) []GraphError {
	found := validateInstance(graphValidator(), doc)
	out := make([]GraphError, 0, len(found)+1)
	nameIsString := false
	if o, ok := doc.(map[string]any); ok {
		if name, ok := o["name"].(string); ok {
			nameIsString = true
			out = append(out, nameErrors(name)...)
		}
	}
	for _, i := range found {
		path := pathOf("", i.Path)
		if path == "name" && nameIsString {
			continue
		}
		code := CodeInvalid
		if i.Limit {
			code = CodeLimit
		}
		if path == "" {
			path = "graph"
		}
		out = append(out, GraphError{Path: path, Code: code, Message: i.Message})
		if len(out) >= maxErrors {
			break
		}
	}
	return out
}

// TrimName is a graph name as the graph keeps it: whitespace at the edges trimmed (strings.TrimSpace; the
// schema's name pattern uses the same class).
func TrimName(name string) string { return strings.TrimSpace(name) }

func nameErrors(name string) []GraphError {
	n := utf8.RuneCountInString(TrimName(name))
	if n == 0 {
		return []GraphError{{Path: "name", Code: CodeInvalid, Message: "a graph has a name"}}
	}
	if n > 120 {
		return []GraphError{{Path: "name", Code: CodeLimit, Message: "a name has at most 120 characters"}}
	}
	return nil
}

// ValidateLayout checks a layout: at most 240 places, ids of the id pattern, finite coordinates.
func ValidateLayout(l Layout) []GraphError {
	var out []GraphError
	if len(l) > MaxLayoutPlaces {
		out = append(out, GraphError{Path: "layout", Code: CodeLimit, Message: fmt.Sprintf("at most %d places, one per node and trigger", MaxLayoutPlaces)})
	}
	for _, id := range sortedKeys(l) {
		xy := l[id]
		if !IsID(id) {
			out = append(out, GraphError{Path: "layout." + id, Code: CodeInvalid, Message: "not a node or trigger id"})
		}
		if math.IsNaN(xy.X) || math.IsInf(xy.X, 0) || math.IsNaN(xy.Y) || math.IsInf(xy.Y, 0) {
			out = append(out, GraphError{Path: "layout." + id, Code: CodeInvalid, Message: "coordinates are finite numbers"})
		}
	}
	return out
}

// LayoutOf keeps only the places of the graph's triggers and nodes: what an owner keeps after a save.
func LayoutOf(g Graph, l Layout) Layout {
	out := Layout{}
	for _, t := range g.Triggers {
		if xy, ok := l[t.ID]; ok {
			out[t.ID] = xy
		}
	}
	for _, n := range g.Nodes {
		if xy, ok := l[n.ID]; ok {
			out[n.ID] = xy
		}
	}
	return out
}

// FreeID is a readable id for a new trigger or node: base, else base_2, base_3, … — the first one the graph
// does not use. Existing ids are never renamed: schedules, run keys and journals name them.
func FreeID(g Graph, base string) string {
	taken := map[string]bool{}
	for _, t := range g.Triggers {
		taken[t.ID] = true
	}
	for _, n := range g.Nodes {
		taken[n.ID] = true
	}
	if !taken[base] {
		return base
	}
	for n := 2; ; n++ {
		if id := fmt.Sprintf("%s_%d", base, n); !taken[id] {
			return id
		}
	}
}

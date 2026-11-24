package aichat

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/automation/expr"
	"github.com/iota-uz/iota-sdk/pkg/automation/graph"
)

// Bounds of the timeout param (the engine reads it: the call's deadline).
const (
	MinTimeout     = time.Second
	MaxTimeout     = 5 * time.Minute
	DefaultTimeout = time.Minute
)

// TimeoutSchema is the JSON Schema property of the reserved timeout param: 60s, 2m or milliseconds.
const TimeoutSchema = `{"type": ["string", "integer"], "pattern": "^[1-9][0-9]{0,5}(s|m)$", "x-max-ms": 300000, "default": "60s",
      "description": "the deadline of the call, repeats of a provider's failure included: 1s to 5m (60s, 2m, or milliseconds)"}`

// Spec is a ParamsSpec by JSON Schema with checks the schema cannot say: it runs Check on the params the
// schema accepted.
type Spec struct {
	Schema graph.ParamsSpec
	Check  func(params map[string]any) []graph.Issue
}

// NewSpec compiles a schema; it panics on an invalid schema written in Go.
func NewSpec(schema string, check func(params map[string]any) []graph.Issue) Spec {
	return Spec{Schema: graph.MustSchemaParams(schema), Check: check}
}

func (s Spec) JSONSchema() json.RawMessage { return s.Schema.JSONSchema() }

func (s Spec) Parse(raw map[string]any) (map[string]any, []graph.Issue) {
	params, issues := s.Schema.Parse(raw)
	if len(issues) > 0 {
		return nil, issues
	}
	if s.Check != nil {
		if issues := s.Check(params); len(issues) > 0 {
			return nil, issues
		}
	}
	return params, nil
}

// Issue is a refusal of one param with a graph error code (empty: params).
func Issue(param, code, format string, args ...any) graph.Issue {
	return graph.Issue{Path: []any{param}, Code: code, Message: fmt.Sprintf(format, args...)}
}

// CheckTimeout refuses a timeout under MinTimeout (the schema bounds it above).
func CheckTimeout(params map[string]any) []graph.Issue {
	if v, ok := params["timeout"]; ok && v != nil {
		if ms, ok := graph.DurationMs(v); !ok || time.Duration(ms)*time.Millisecond < MinTimeout || time.Duration(ms)*time.Millisecond > MaxTimeout {
			return []graph.Issue{Issue("timeout", "", "a timeout is 1s to 5m")}
		}
	}
	return nil
}

// CheckModelParam refuses a model that is not an exact id. An empty model is left to CompleteParams.
func CheckModelParam(params map[string]any) []graph.Issue {
	if m, _ := params["model"].(string); m != "" {
		if err := CheckModel(m); err != nil {
			return []graph.Issue{Issue("model", "", "%v", err)}
		}
	}
	return nil
}

// Int reads an integer param.
func Int(v any) (int, bool) {
	f, ok := expr.Number(v)
	if !ok || f != float64(int(f)) {
		return 0, false
	}
	return int(f), true
}

// Float reads a number param.
func Float(v any) (float64, bool) { return expr.Number(v) }

// CompleteModel returns params with the model set to def when it is absent or empty; the argument is not
// changed. Params of another shape are returned as they are.
func CompleteModel(params map[string]any, def string) map[string]any {
	if m, set := params["model"]; set && m != nil {
		if str, isStr := m.(string); !isStr || str != "" {
			return params // a model, or a value the params check refuses
		}
	}
	out := make(map[string]any, len(params)+1)
	for k, v := range params {
		out[k] = v
	}
	out["model"] = def
	return out
}

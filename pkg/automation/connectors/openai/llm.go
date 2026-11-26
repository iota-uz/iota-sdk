package openai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/big"
	"strconv"
	"strings"

	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors/internal/aichat"
	"github.com/iota-uz/iota-sdk/pkg/automation/engine"
	"github.com/iota-uz/iota-sdk/pkg/automation/expr"
	"github.com/iota-uz/iota-sdk/pkg/automation/graph"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/openaiconfig"
)

// Formats of the node's answer.
const (
	FormatText = "text"
	FormatJSON = "json"
)

// Outcomes of the node.
const (
	OutcomeDone   = "done"
	OutcomeFailed = "failed"
)

// ErrorModelNoVision is the graph error code of a node that sends images to a model without vision;
// ErrorModelNoStructuredOutput that of a JSON answer from a model that has no strict structured output.
const (
	ErrorModelNoVision           = "model_no_vision"
	ErrorModelNoStructuredOutput = "model_no_structured_output"
)

// Limits of the params.
const (
	DefaultMaxTokens = 2000
	MaxMaxTokens     = 32000
	maxSchemaBytes   = 64 << 10
)

// LLMParams are the params of ai.llm (the reserved "connection" param is added by the catalog): the model
// (an exact id; empty: the default model, written into the version when the graph is saved), the system
// and user prompts (templates; the prompt is not empty once rendered), the answer's format, the JSON
// Schema of a JSON answer, the bounds of the answer and the deadline of the call.
//
// TimeoutMs is the deadline of the call in milliseconds, repeats included (the engine reads it), 1000 to
// 300000, default 60000. Images are the upload ids (at most 8, 5 MB each) sent with the prompt: a list, or
// a CEL expression of one; the model must take images.
type LLMParams struct {
	Connection  string
	Model       string
	System      string
	Prompt      string
	Format      string
	Schema      json.RawMessage
	MaxTokens   int
	Temperature *float64
	TimeoutMs   int
	Images      any // []string, or a CEL expression of one
}

// LLMOutput is nodes.<id>.output of a node that took its done outcome. JSON is typed from Schema.
type LLMOutput struct {
	Text      string          `json:"text,omitempty"` // format text
	JSON      json.RawMessage `json:"json,omitempty"` // format json
	Model     string          `json:"model"`
	Usage     Usage           `json:"usage"`
	LatencyMs int             `json:"latency_ms"`
}

const llmParamsSchema = `{
  "type": "object",
  "required": ["prompt"],
  "properties": {
    "model":       {"type": "string", "maxLength": 128, "description": "an exact model id (no alias such as latest); left out: the default model"},
    "system":      {"type": "string", "maxLength": 50000, "description": "the system prompt (a template)"},
    "prompt":      {"type": "string", "minLength": 1, "maxLength": 100000, "description": "the prompt (a template)"},
    "format":      {"type": "string", "enum": ["text", "json"], "default": "text"},
    "schema":      {"type": "object", "description": "the JSON Schema of the answer, for format json"},
    "max_tokens":  {"type": "integer", "minimum": 1, "maximum": 32000, "default": 2000},
    "temperature": {"type": "number", "minimum": 0, "maximum": 2},
    "timeout_ms":  ` + aichat.TimeoutSchema + `,
    "images":      {"description": "the upload ids to send with the prompt (at most 8, 5 MB each): a list, or a CEL expression of one",
                    "oneOf": [
                      {"type": "array", "maxItems": 8, "items": {"type": "string", "minLength": 1, "maxLength": 64}},
                      {"type": "string", "minLength": 1, "maxLength": 2000}]}
  },
  "additionalProperties": false
}`

type llmNode struct {
	o      options
	client *aichat.Client
	// unauthorized is called when the provider refuses the key of a connection.
	unauthorized func(ctx context.Context, in catalog.CallInput)
	params       graph.ParamsSpec
}

var (
	_ catalog.CallNode      = (*llmNode)(nil)
	_ catalog.CompletedNode = (*llmNode)(nil)
	_ engine.DecisionNode   = (*llmNode)(nil)
)

func newLLMNode(o options, client *aichat.Client, unauthorized func(context.Context, catalog.CallInput)) *llmNode {
	n := &llmNode{o: o, client: client, unauthorized: unauthorized}
	n.params = aichat.NewSpec(llmParamsSchema, n.check)
	return n
}

// check is what the params schema cannot say: the model is an exact id and takes the images, the answer's
// schema fits the format and is a JSON Schema the node can type and enforce.
func (n *llmNode) check(p map[string]any) []graph.Issue {
	issues := aichat.CheckModelParam(p)
	issues = append(issues, aichat.CheckTimeout(p)...)
	format, _ := p["format"].(string)
	schema, hasSchema := p["schema"]
	switch {
	case format == FormatJSON && !hasSchema:
		issues = append(issues, aichat.Issue("schema", "", "a JSON answer needs a schema"))
	case format != FormatJSON && hasSchema:
		issues = append(issues, aichat.Issue("schema", "", "a schema is for format json"))
	case hasSchema:
		raw, err := json.Marshal(schema)
		if err != nil || len(raw) > maxSchemaBytes {
			issues = append(issues, aichat.Issue("schema", "", "the schema is a JSON Schema of at most %d KB", maxSchemaBytes>>10))
			break
		}
		if _, err := expr.FieldFromJSONSchema(raw); err != nil {
			issues = append(issues, aichat.Issue("schema", "", "%v", err))
			break
		}
		if _, err := aichat.Prepare(raw); err != nil {
			issues = append(issues, aichat.Issue("schema", "", "%v", err))
		}
	}
	if format == FormatJSON {
		model, _ := p["model"].(string)
		if model == "" {
			model = n.o.defaultModel
		}
		if aichat.NoStructuredOutput(model) {
			issues = append(issues, aichat.Issue("model", ErrorModelNoStructuredOutput, "the model %s cannot answer in strict structured output; choose a newer model or format text", model))
		}
	}
	if hasImages(p) {
		model, _ := p["model"].(string)
		if model == "" {
			model = n.o.defaultModel
		}
		if !n.o.vision(model) {
			issues = append(issues, aichat.Issue("images", ErrorModelNoVision, "the model %s does not take images; choose a model with vision or remove images", model))
		}
	}
	return issues
}

// hasImages reports whether the images param names any: a list that is not empty, or an expression that is
// not blank or an empty list.
func hasImages(p map[string]any) bool {
	switch x := p["images"].(type) {
	case []any:
		return len(x) > 0
	case string:
		s := strings.TrimSpace(x)
		return s != "" && s != "[]"
	}
	return false
}

func (n *llmNode) Spec() graph.NodeSpec {
	return graph.NodeSpec{
		Type:        NodeLLM,
		Title:       "LLM",
		Description: "Generates text, or JSON checked against a schema, with an OpenAI model. The call has no effect outside the graph, so it is repeated when its outcome is unknown.",
		Behavior:    graph.BehaviorCall,
		Params:      n.params,
		Templates:   []string{"system", "prompt"},
		Expressions: []graph.ExprParam{{Param: "images", Type: expr.TypeList}},
		Values:      []string{"images"},
		Outcomes:    []string{OutcomeDone, OutcomeFailed},
		DoneOutcome: OutcomeDone, FailedOutcome: OutcomeFailed,
		// A dry run calls the model for real: the decision has no effect, and a cached dry run reuses it.
		DryRun: graph.DryRunExecute,
		Output: expr.Fields{
			{Name: "text", Field: expr.OptionalOf(expr.StringField)},
			{Name: "json", Field: expr.DynField},
			{Name: "model", Field: expr.StringField},
			{Name: "usage", Field: usageField},
			{Name: "latency_ms", Field: expr.IntField},
		},
		// output.json takes its type from the schema param.
		OutputFrom: &graph.ParamRef{Param: "schema", Field: "json"},
	}
}

var usageField = expr.ObjectOf(expr.Fields{
	{Name: "input_tokens", Field: expr.IntField},
	{Name: "output_tokens", Field: expr.IntField},
	{Name: "cost_usd", Field: expr.OptionalOf(expr.DoubleField)},
})

func (n *llmNode) Requires() []catalog.ConnectorKind { return []catalog.ConnectorKind{Kind} }

// Idempotent is true: the call has no external effect, so the engine repeats it when its outcome is
// unknown. A repeat is paid again and recorded.
func (n *llmNode) Idempotent() bool { return true }

// CompleteParams writes the default model into a node saved without one: a saved version names its model,
// and a later change of the default does not change it.
func (n *llmNode) CompleteParams(p map[string]any) map[string]any {
	return aichat.CompleteModel(p, n.o.defaultModel)
}

// DecisionKey is the key of the decision in the cache of a dry run: the rendered prompts, the images, the
// model and the params that shape the answer.
func (n *llmNode) DecisionKey(in catalog.CallInput) (string, bool) {
	r, err := n.read(in.Params)
	if err != nil {
		return "", false
	}
	params := map[string]any{"format": r.format, "max_tokens": r.maxTokens}
	if r.schema != nil {
		params["schema"] = r.schema
	}
	if r.temperature != nil {
		params["temperature"] = *r.temperature
	}
	// The model ids belong to an endpoint: the answer of one endpoint is not the answer of another.
	params["endpoint"], params["connection"] = endpointIdentity(in), connectionID(in)
	return engine.DecisionKey(r.prompt, map[string]any{"system": r.system, "images": r.images}, r.model, params)
}

// llmCall is the params of a call, read.
type llmCall struct {
	model, system, prompt, format string
	schema                        json.RawMessage
	maxTokens                     int
	temperature                   *float64
	images                        []string
}

// read takes the rendered params of a call. An error is an invalid request.
func (n *llmNode) read(p map[string]any) (llmCall, error) {
	var r llmCall
	r.model, _ = p["model"].(string)
	if r.model == "" {
		r.model = n.o.defaultModel
	}
	if err := aichat.CheckModel(r.model); err != nil {
		return r, err
	}
	r.system, _ = p["system"].(string)
	r.prompt, _ = p["prompt"].(string)
	if strings.TrimSpace(r.prompt) == "" {
		return r, errors.New("the prompt is empty once rendered")
	}
	r.format, _ = p["format"].(string)
	if r.format == "" {
		r.format = FormatText
	}
	if r.format != FormatText && r.format != FormatJSON {
		return r, errors.New("format is text or json")
	}
	if r.format == FormatJSON {
		raw, err := json.Marshal(p["schema"])
		if err != nil || p["schema"] == nil {
			return r, errors.New("a JSON answer needs a schema")
		}
		r.schema = raw
	}
	r.maxTokens = DefaultMaxTokens
	if v, ok := p["max_tokens"]; ok && v != nil {
		m, ok := aichat.Int(v)
		if !ok || m < 1 || m > MaxMaxTokens {
			return r, fmt.Errorf("max_tokens is 1 to %d", MaxMaxTokens)
		}
		r.maxTokens = m
	}
	if v, ok := p["temperature"]; ok && v != nil {
		t, ok := aichat.Float(v)
		if !ok || t < 0 || t > 2 {
			return r, errors.New("temperature is 0 to 2")
		}
		r.temperature = &t
	}
	if v, ok := p["images"]; ok && v != nil {
		list, ok := v.([]any)
		if !ok {
			return r, errors.New("images is a list of upload ids")
		}
		for _, e := range list {
			id, ok := uploadID(e)
			if !ok {
				return r, errors.New("images is a list of upload ids")
			}
			r.images = append(r.images, id)
		}
		if len(r.images) > MaxImages {
			return r, fmt.Errorf("at most %d images", MaxImages)
		}
	}
	return r, nil
}

// endpointIdentity is the effective API of a call: the connection's base URL, or OpenAI's.
func endpointIdentity(in catalog.CallInput) string {
	if u := strings.TrimRight(strings.TrimSpace(in.Credentials["base_url"]), "/"); u != "" {
		return u
	}
	return openaiconfig.DefaultBaseURL
}

// connectionID is the non-secret identity of the connection a call runs with: a tenant's own key and the
// platform key may reach different accounts.
func connectionID(in catalog.CallInput) string {
	if in.Connection == nil {
		return ""
	}
	return in.Connection.ID
}

// uploadID is an upload id of a list the engine rendered: a string, or a whole number (a fraction is no id,
// and is not rounded to one).
func uploadID(v any) (string, bool) {
	switch id := v.(type) {
	case string:
		return id, true
	case int:
		return strconv.Itoa(id), true
	case int64:
		return strconv.FormatInt(id, 10), true
	case json.Number:
		// Integrality is decided on the decimal text, not on a float: 7.9999999999999999 rounds to 8 as a float64.
		if len(id) > 64 || hugeExponent(id.String()) {
			return "", false // no upload id is this long, and a huge exponent is not expanded
		}
		r, ok := new(big.Rat).SetString(id.String())
		if !ok || !r.IsInt() || !r.Num().IsInt64() {
			return "", false
		}
		return strconv.FormatInt(r.Num().Int64(), 10), true
	case float64:
		return wholeNumber(id)
	}
	return "", false
}

// hugeExponent reports whether a decimal text has an exponent too large for an id (the value would be
// expanded by big.Rat before it is judged): an id has at most 10 digits, so any exponent past 30 is refused.
func hugeExponent(text string) bool {
	i := strings.IndexAny(text, "eE")
	if i < 0 {
		return false
	}
	exp, err := strconv.Atoi(strings.TrimPrefix(text[i+1:], "+"))
	return err != nil || exp > 30 || exp < -30
}

func wholeNumber(f float64) (string, bool) {
	if f != math.Trunc(f) || math.Abs(f) > 1<<53 {
		return "", false
	}
	return strconv.FormatInt(int64(f), 10), true
}

func invalid(msg string) error {
	return &catalog.CallError{Code: catalog.CallInvalidRequest, Message: msg}
}

func (n *llmNode) Execute(ctx context.Context, in catalog.CallInput) (catalog.CallResult, error) {
	r, err := n.read(in.Params)
	if err != nil {
		return catalog.CallResult{}, invalid(err.Error())
	}
	ep, err := endpointOf(in.Credentials, in.Connection != nil && in.Connection.Platform)
	if err != nil {
		if errors.Is(err, errNoKey) {
			return catalog.CallResult{}, &catalog.CallError{Code: catalog.CallUnauthorized, Message: err.Error()}
		}
		return catalog.CallResult{}, invalid(err.Error())
	}
	if len(r.images) > 0 && !n.o.vision(r.model) {
		return catalog.CallResult{}, invalid(fmt.Sprintf("the model %s does not take images", r.model))
	}
	if r.format == FormatJSON && aichat.NoStructuredOutput(r.model) {
		return catalog.CallResult{}, invalid(fmt.Sprintf("the model %s cannot answer in strict structured output", r.model))
	}
	images, err := loadImages(ctx, n.o.images, in.TenantID, r.images)
	if err != nil {
		var re *readError
		if errors.As(err, &re) {
			return catalog.CallResult{}, &catalog.CallError{Code: catalog.CallUnavailable, Message: re.Error(), Retryable: true}
		}
		return catalog.CallResult{}, invalid(err.Error())
	}
	req := aichat.Request{Model: r.model, System: r.system, Prompt: r.prompt, Images: images, MaxTokens: r.maxTokens, Temperature: r.temperature}
	var schema *aichat.Schema
	if r.format == FormatJSON {
		if schema, err = aichat.Prepare(r.schema); err != nil {
			return catalog.CallResult{}, invalid(err.Error())
		}
		req.JSON = &aichat.JSONFormat{Name: "result", Schema: schema.Strict}
	}

	started := n.o.now()
	resp, fail := n.client.Complete(ctx, ep, req)
	// Whatever the answer is, the tokens it reports were spent.
	aichat.RecordUsage(ctx, n.o.log, in, resp.Usage, started)
	if fail != nil {
		if fail.Err != nil && fail.Err.Code == catalog.CallUnauthorized && n.unauthorized != nil {
			n.unauthorized(ctx, in)
		}
		return fail.Result()
	}
	latency := n.o.now().Sub(started)

	out := map[string]any{
		"model": firstNonEmpty(resp.Model, r.model),
		"usage": map[string]any{
			"input_tokens": int64(resp.Usage.InputTokens), "output_tokens": int64(resp.Usage.OutputTokens), "cost_usd": cost(resp.Usage.CostUSD),
		},
		"latency_ms": latency.Milliseconds(),
	}
	if r.format == FormatText {
		if strings.TrimSpace(resp.Text) == "" {
			return catalog.CallResult{}, invalid("the model answered with no text")
		}
		out["text"] = resp.Text
		return catalog.CallResult{Outcome: OutcomeDone, Output: out}, nil
	}
	value, err := n.checkJSON(resp, schema)
	if err != nil {
		return catalog.CallResult{}, invalid(err.Error())
	}
	out["json"] = value
	return catalog.CallResult{Outcome: OutcomeDone, Output: out}, nil
}

func cost(c *float64) any {
	if c == nil {
		return nil
	}
	return *c
}

// checkJSON reads the answer as JSON of the schema. The answer is checked here whatever the provider's
// strict mode promised; one that does not fit is an invalid request, and is not asked for again.
func (n *llmNode) checkJSON(resp aichat.Response, s *aichat.Schema) (any, error) {
	if resp.FinishReason == "length" {
		return nil, errors.New("the JSON answer was cut off at max_tokens")
	}
	value, err := aichat.DecodeJSON(resp.Text)
	if err != nil {
		return nil, fmt.Errorf("the answer is not JSON: %w", err)
	}
	if value, err = s.Unwrap(value); err != nil {
		return nil, err
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, errors.New("the answer is not JSON")
	}
	if err := graph.ValidateJSON(s.Closed, raw); err != nil {
		return nil, fmt.Errorf("the answer does not match the schema: %v", err)
	}
	return value, nil
}

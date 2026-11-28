package aichat

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors"
	"github.com/iota-uz/iota-sdk/pkg/automation/expr"
	"github.com/iota-uz/iota-sdk/pkg/automation/graph"
)

// OpComplete is the code operation of the AI connectors (env.connections.<name>.complete).
const OpComplete = "complete"

// CompleteOperation is the declaration of the operation complete for the editor.
func CompleteOperation() connectors.CodeOperation {
	return connectors.CodeOperation{
		Name:        OpComplete,
		Description: "Ask the model: a text answer, or JSON (json: true, or a JSON Schema the answer must fit).",
		Params:      `{ prompt: string; system?: string; model?: string; json?: boolean | object; temperature?: number; max_tokens?: number }`,
		Result:      `{ text?: string; json?: unknown; model: string; usage: { input_tokens: number; output_tokens: number } }`,
	}
}

// Limits of the arguments of complete.
const (
	codeMaxPrompt  = 100000
	codeMaxSystem  = 50000
	codeMaxSchema  = 64 << 10
	codeDefMax     = 2000
	codeMaxMax     = 32000
	jsonOnlyPrompt = "Answer with one valid JSON value and nothing else: no prose and no code fence."
)

// CodeCompleter is what an AI connector gives the operation: its client, its log and clock, the default model
// of the call, and what to do when the provider refuses the key.
type CodeCompleter struct {
	Client *Client
	Log    *slog.Logger
	Now    func() time.Time
	// DefaultModel is the model of a call that names none.
	DefaultModel string
	// Unauthorized is called when the provider refuses the connection's key (mark it needs_reauth).
	Unauthorized func(ctx context.Context, in catalog.CallInput)
}

type completeArgs struct {
	Prompt      string          `json:"prompt"`
	System      string          `json:"system"`
	Model       string          `json:"model"`
	JSON        json.RawMessage `json:"json"`
	Temperature *float64        `json:"temperature"`
	MaxTokens   *int            `json:"max_tokens"`
}

func refusal(code, msg string, retryable bool) error {
	return &connectors.CodeOperationError{Code: code, Message: msg, Retryable: retryable}
}

func invalidArgs(format string, a ...any) error {
	return refusal("invalid_args", fmt.Sprintf(format, a...), false)
}

// Complete runs one completion for code. Its usage is recorded on the call's connection as the AI nodes do;
// neither the result nor an error carries the key.
func (c CodeCompleter) Complete(ctx context.Context, ep Endpoint, call connectors.CodeCall) (any, error) {
	in := call.Input
	args, err := readCompleteArgs(call.Args)
	if err != nil {
		return nil, err
	}
	model := args.Model
	if model == "" {
		model = c.DefaultModel
	}
	if !modelIDRe.MatchString(model) || len(model) > maxModelLen {
		return nil, invalidArgs("model is a model id: letters, digits and . _ - : / @ +, at most %d characters", maxModelLen)
	}
	if args.Temperature != nil && !TakesTemperature(model) {
		return nil, invalidArgs("the model %s takes no temperature; omit temperature or choose another model", model)
	}
	req := Request{Model: model, System: args.System, Prompt: args.Prompt, MaxTokens: codeDefMax, Temperature: args.Temperature}
	if args.MaxTokens != nil {
		req.MaxTokens = *args.MaxTokens
	}
	var schema *Schema
	asJSON := false
	switch j := bytes.TrimSpace(args.JSON); {
	case len(j) == 0, string(j) == "null", string(j) == "false":
	case string(j) == "true":
		asJSON = true
		req.System = strings.TrimSpace(req.System + "\n\n" + jsonOnlyPrompt)
	case j[0] == '{':
		asJSON = true
		if schema, err = prepareCodeSchema(j); err != nil {
			return nil, invalidArgs("json: %v", err)
		}
		if NoStructuredOutput(model) {
			return nil, invalidArgs("the model %s cannot answer in strict structured output; choose a newer model, or json: true", model)
		}
		req.JSON = &JSONFormat{Name: "result", Schema: schema.Strict}
	default:
		return nil, invalidArgs("json is true, false or a JSON Schema object")
	}

	if !in.Deadline.IsZero() {
		var cancel context.CancelFunc
		ctx, cancel = context.WithDeadline(ctx, in.Deadline)
		defer cancel()
	}
	now := c.Now
	if now == nil {
		now = time.Now
	}
	started := now()
	resp, fail := c.Client.Complete(ctx, ep, req)
	// Whatever the answer is, the tokens it reports were spent.
	RecordUsage(ctx, c.Log, in, resp.Usage, started)
	if fail != nil {
		return nil, c.failure(ctx, in, fail)
	}
	out := map[string]any{
		"model": firstNonEmpty(resp.Model, model),
		"usage": map[string]any{"input_tokens": int64(resp.Usage.InputTokens), "output_tokens": int64(resp.Usage.OutputTokens)},
	}
	if !asJSON {
		if strings.TrimSpace(resp.Text) == "" {
			return nil, refusal("invalid_response", "the model answered with no text", false)
		}
		out["text"] = resp.Text
		return out, nil
	}
	value, err := checkCodeJSON(resp, schema)
	if err != nil {
		return nil, refusal("invalid_response", err.Error(), false)
	}
	out["json"] = value
	return out, nil
}

func readCompleteArgs(raw json.RawMessage) (completeArgs, error) {
	var a completeArgs
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || string(raw) == "null" {
		return a, invalidArgs("prompt is required")
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&a); err != nil {
		return a, invalidArgs("the argument is not an object of prompt, system, model, json, temperature and max_tokens: %s", strings.TrimPrefix(err.Error(), "json: "))
	}
	switch {
	case strings.TrimSpace(a.Prompt) == "":
		return a, invalidArgs("prompt is required")
	case len(a.Prompt) > codeMaxPrompt:
		return a, invalidArgs("prompt is at most %d characters", codeMaxPrompt)
	case len(a.System) > codeMaxSystem:
		return a, invalidArgs("system is at most %d characters", codeMaxSystem)
	case a.Temperature != nil && (*a.Temperature < 0 || *a.Temperature > 2):
		return a, invalidArgs("temperature is 0 to 2")
	case a.MaxTokens != nil && (*a.MaxTokens < 1 || *a.MaxTokens > codeMaxMax):
		return a, invalidArgs("max_tokens is 1 to %d", codeMaxMax)
	}
	return a, nil
}

// prepareCodeSchema checks a JSON Schema of code and prepares it for the provider's strict mode.
func prepareCodeSchema(raw json.RawMessage) (*Schema, error) {
	if len(raw) > codeMaxSchema {
		return nil, fmt.Errorf("the schema is at most %d KB", codeMaxSchema>>10)
	}
	if _, err := expr.FieldFromJSONSchema(raw); err != nil {
		return nil, err
	}
	return Prepare(raw)
}

// checkCodeJSON reads the answer as JSON, of the schema when there is one.
func checkCodeJSON(resp Response, s *Schema) (any, error) {
	if resp.FinishReason == "length" {
		return nil, errors.New("the JSON answer was cut off at max_tokens")
	}
	text := strings.TrimSpace(resp.Text)
	if s == nil {
		text = stripFence(text)
	}
	value, err := DecodeJSON(text)
	if err != nil {
		return nil, fmt.Errorf("the answer is not JSON: %w", err)
	}
	if s == nil {
		return value, nil
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

// stripFence takes the code fence a model put around its JSON off.
func stripFence(s string) string {
	if !strings.HasPrefix(s, "```") || !strings.HasSuffix(s, "```") || len(s) < 6 {
		return s
	}
	inner := s[3 : len(s)-3]
	if i := strings.IndexByte(inner, '\n'); i >= 0 && !strings.ContainsAny(inner[:i], "{[") {
		inner = inner[i+1:]
	}
	return strings.TrimSpace(inner)
}

// failure sorts a provider's failure into a refusal the code may handle.
func (c CodeCompleter) failure(ctx context.Context, in catalog.CallInput, f *Failure) error {
	e := f.Err
	switch e.Code {
	case catalog.CallUnauthorized:
		if c.Unauthorized != nil {
			c.Unauthorized(ctx, in)
		}
		return refusal("provider_refused", e.Message, false)
	case catalog.CallRateLimited:
		return refusal("rate_limited", e.Message, true)
	case catalog.CallUnavailable:
		return refusal("unavailable", e.Message, e.Retryable)
	case catalog.CallRejected, catalog.CallInvalidRequest:
		return refusal("provider_refused", e.Message, false)
	}
	return refusal("provider_error", e.Message, e.Retryable)
}

func firstNonEmpty(list ...string) string {
	for _, s := range list {
		if s = strings.TrimSpace(s); s != "" {
			return s
		}
	}
	return ""
}

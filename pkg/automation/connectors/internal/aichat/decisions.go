package aichat

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
)

// OpenRouterDecisionsURL is OpenRouter's Decisions API, which serves TypeSafe Jev: typed answers with a
// probability per choice and the provider's confidence, instead of a chat completion.
const OpenRouterDecisionsURL = "https://openrouter.ai/api/alpha/decisions"

// DecisionRequest is one choice question to the Decisions API.
type DecisionRequest struct {
	Model string // an exact version: typesafe/jev-1.14
	// State is the data to decide on: a JSON object.
	State map[string]any
	// Instructions says how to decide; Criteria are the choices, by name, with their descriptions.
	Instructions string
	Criteria     []Criterion
}

// Criterion is one choice of a decision.
type Criterion struct{ Name, Description string }

// decisionQuestion is the name the request gives its one question; the answer comes back under it.
const decisionQuestion = "decision"

// DecisionResponse is a typed choice with its distribution.
type DecisionResponse struct {
	Model    string
	Provider string
	Choice   string
	// Probabilities are the provider's, by choice; Confidence is its own confidence in the choice.
	Probabilities map[string]float64
	Confidence    *float64
	Usage         Usage
}

type decisionBody struct {
	Model     string                    `json:"model"`
	State     map[string]any            `json:"state"`
	Questions map[string]map[string]any `json:"questions"`
}

// decisionReply is the answer read loosely: only its envelope is typed, so a malformed answer inside it
// is told from a malformed body, and the usage it reports is kept either way.
type decisionReply struct {
	Model    string          `json:"model"`
	Provider string          `json:"provider"`
	Answers  json.RawMessage `json:"answers"`
	Usage    json.RawMessage `json:"usage"`
}

type decisionAnswer struct {
	Type          *string         `json:"type"`
	Choice        json.RawMessage `json:"choice"`
	Probabilities json.RawMessage `json:"probabilities"`
	Confidence    json.RawMessage `json:"confidence"`
}

// Decide asks the Decisions API one choice question. As in Complete, a refusal or an answer that cannot
// be used comes back with the Response (its usage is spent) and a Failure.
func (c *Client) Decide(ctx context.Context, ep Endpoint, r DecisionRequest) (DecisionResponse, *Failure) {
	criteria := make(map[string]any, len(r.Criteria))
	for _, cr := range r.Criteria {
		criteria[cr.Name] = cr.Description
	}
	body, err := json.Marshal(decisionBody{Model: r.Model, State: r.State, Questions: map[string]map[string]any{
		decisionQuestion: {"type": "choice", "instructions": r.Instructions, "criteria": criteria},
	}})
	if err != nil {
		return DecisionResponse{}, fail(catalog.CallInvalidRequest, "the request is not JSON", false)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, ep.BaseURL, bytes.NewReader(body))
	if err != nil {
		return DecisionResponse{}, fail(catalog.CallInvalidRequest, "the endpoint URL is not valid", false)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+ep.APIKey)
	resp, err := c.httpClient(ep).Do(req)
	if err != nil {
		return DecisionResponse{}, networkFailure(ctx, err, ep.APIKey)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxResponse+1))
	if err != nil {
		return DecisionResponse{}, networkFailure(ctx, err, ep.APIKey)
	}
	if len(raw) > maxResponse {
		return DecisionResponse{}, fail(catalog.CallUnavailable, "the provider's answer is too large", false)
	}
	if resp.StatusCode != http.StatusOK {
		return DecisionResponse{}, decisionStatusFailure(resp, raw, ep.APIKey)
	}
	var dr decisionReply
	if err := json.Unmarshal(raw, &dr); err != nil {
		// Not a JSON object at all (a proxy's error page): the provider may answer properly next time.
		return DecisionResponse{}, fail(catalog.CallUnavailable, "the provider's answer is not JSON", true)
	}
	out := DecisionResponse{Model: dr.Model, Provider: dr.Provider}
	// The tokens were spent whatever the decision looks like: the usage is read first and on its own.
	var u struct {
		InputTokens  int      `json:"input_tokens"`
		OutputTokens int      `json:"output_tokens"`
		Cost         *float64 `json:"cost"`
	}
	if len(dr.Usage) > 0 && json.Unmarshal(dr.Usage, &u) == nil {
		out.Usage = Usage{InputTokens: u.InputTokens, OutputTokens: u.OutputTokens, CostUSD: u.Cost}
	}
	invalid := func(msg string) (DecisionResponse, *Failure) {
		return out, fail(catalog.CallInvalidRequest, msg, false)
	}
	var answers map[string]json.RawMessage
	if json.Unmarshal(dr.Answers, &answers) != nil {
		return invalid("the provider's answers are malformed")
	}
	rawAnswer, ok := answers[decisionQuestion]
	if !ok {
		return invalid("the provider's answer has no decision")
	}
	var a decisionAnswer
	if err := json.Unmarshal(rawAnswer, &a); err != nil {
		return invalid("the provider's decision is malformed")
	}
	switch {
	case a.Type != nil && *a.Type == "refusal":
		return out, fail(catalog.CallRejected, "the provider declined the decision", false)
	case a.Type == nil || *a.Type != "choice":
		return invalid("the provider's answer is not a choice")
	}
	// A choice answer carries its choice, its distribution and the provider's confidence in it: each is of
	// its type, and a value that is not a number is not a probability (decoded into numbers a null would be 0).
	if json.Unmarshal(a.Choice, &out.Choice) != nil || out.Choice == "" {
		return invalid("the provider's choice is not a string")
	}
	var probs map[string]json.RawMessage
	if json.Unmarshal(a.Probabilities, &probs) != nil || len(probs) == 0 {
		return invalid("the provider's answer has no probabilities")
	}
	out.Probabilities = make(map[string]float64, len(probs))
	for name, rawProb := range probs {
		v, ok := wireNumber(rawProb)
		if !ok {
			return invalid("the provider's probability of a choice is not a number")
		}
		out.Probabilities[name] = v
	}
	conf, ok := wireNumber(a.Confidence)
	if !ok {
		return invalid("the provider's answer has no confidence")
	}
	out.Confidence = &conf
	return out, nil
}

// wireNumber reads a JSON number; a null, a string or a missing value is not one.
func wireNumber(raw json.RawMessage) (float64, bool) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || raw[0] == '"' || bytes.Equal(raw, []byte("null")) {
		return 0, false
	}
	var v float64
	if json.Unmarshal(raw, &v) != nil {
		return 0, false
	}
	return v, true
}

// decisionStatusFailure sorts a non-200 answer of the Decisions API: its error body carries a numeric code
// and a message, no OpenAI-style type.
func decisionStatusFailure(resp *http.Response, raw []byte, key string) *Failure {
	f := statusFailure(resp, raw, key)
	if resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusRequestEntityTooLarge {
		// A request the provider refuses (a bad model version, too much state): repeating cannot help.
		f.Err.Code = catalog.CallInvalidRequest
		if resp.StatusCode == http.StatusRequestEntityTooLarge {
			f.Err.Message = strings.TrimSpace("the input is too large for the model: " + f.Err.Message)
		}
	}
	if resp.StatusCode == 524 || resp.StatusCode == 529 {
		f.Err.Code, f.Err.Retryable = catalog.CallUnavailable, true
	}
	return f
}

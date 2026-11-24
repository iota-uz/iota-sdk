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

type decisionReply struct {
	Model    string `json:"model"`
	Provider string `json:"provider"`
	Answers  map[string]struct {
		Type          string             `json:"type"`
		Choice        string             `json:"choice"`
		Probabilities map[string]float64 `json:"probabilities"`
		Confidence    *float64           `json:"confidence"`
	} `json:"answers"`
	Usage *struct {
		InputTokens  int      `json:"input_tokens"`
		OutputTokens int      `json:"output_tokens"`
		Cost         *float64 `json:"cost"`
	} `json:"usage"`
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
		return DecisionResponse{}, fail(catalog.CallUnavailable, "the provider's answer is not JSON", true)
	}
	out := DecisionResponse{Model: dr.Model, Provider: dr.Provider}
	if dr.Usage != nil {
		out.Usage = Usage{InputTokens: dr.Usage.InputTokens, OutputTokens: dr.Usage.OutputTokens, CostUSD: dr.Usage.Cost}
	}
	a, ok := dr.Answers[decisionQuestion]
	switch {
	case !ok:
		return out, fail(catalog.CallInvalidRequest, "the provider's answer has no decision", false)
	case a.Type == "refusal":
		return out, fail(catalog.CallRejected, "the provider declined the decision", false)
	case a.Type != "" && a.Type != "choice":
		return out, fail(catalog.CallInvalidRequest, "the provider's answer is not a choice", false)
	}
	out.Choice, out.Probabilities, out.Confidence = a.Choice, a.Probabilities, a.Confidence
	return out, nil
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

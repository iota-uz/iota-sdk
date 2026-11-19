package testenv

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func contextWithTimeoutMs(r *http.Request, timeoutMs int) (context.Context, context.CancelFunc) {
	return context.WithTimeout(r.Context(), time.Duration(timeoutMs)*time.Millisecond)
}

// Control is the scenario surface shared by Go callers, the CLI and HTTP
// clients. Local wraps a Registry in-process; NewClient talks to NewHandler.
type Control interface {
	Definitions(context.Context) ([]Definition, error)
	ReserveScope(context.Context, string) error
	Prepare(context.Context, Input) (Result, error)
	Dispose(context.Context, string) error
	Manifest(context.Context, string) (Manifest, error)
	Replay(context.Context, string, Manifest) (Result, error)
}

// Local exposes a registry as a Control. ReserveScope uses the environment
// namespace rule when the registry is environment-bound; otherwise the local
// caller is the owner and allocates the scope directly.
func Local(r *Registry) Control { return localControl{r} }

type localControl struct{ r *Registry }

func (l localControl) Definitions(context.Context) ([]Definition, error) {
	return l.r.Definitions(), nil
}
func (l localControl) ReserveScope(_ context.Context, id string) error {
	if l.r.environmentID != "" {
		return l.r.ReserveScope(id)
	}
	if l.r.owned(id) == nil {
		return nil
	}
	return l.r.AllowScope(id)
}
func (l localControl) Prepare(ctx context.Context, input Input) (Result, error) {
	return l.r.Prepare(ctx, input)
}
func (l localControl) Dispose(ctx context.Context, id string) error { return l.r.Dispose(ctx, id) }
func (l localControl) Manifest(ctx context.Context, id string) (Manifest, error) {
	return l.r.Manifest(ctx, id)
}
func (l localControl) Replay(ctx context.Context, id string, m Manifest) (Result, error) {
	return l.r.Replay(ctx, id, m)
}

// Client calls an environment's HTTP control routes. It verifies that every
// answer comes from the expected environment when EnvironmentID is set.
type Client struct {
	BaseURL       string
	Token         string
	EnvironmentID string
	HTTP          *http.Client
}

// NewClient returns an HTTP Control. token is the runtime control credential.
func NewClient(baseURL, token, environmentID string) (*Client, error) {
	if len(token) < 32 {
		return nil, failure("invalid_input", "a control token of at least 32 bytes required")
	}
	if _, err := url.Parse(baseURL); err != nil || baseURL == "" {
		return nil, failure("invalid_input", "invalid base URL")
	}
	return &Client{BaseURL: strings.TrimRight(baseURL, "/"), Token: token, EnvironmentID: environmentID, HTTP: &http.Client{Timeout: 150 * time.Second}}, nil
}

func (c *Client) do(ctx context.Context, method, path string, body, target any) error {
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return failure("invalid_input", err.Error())
		}
		reader = bytes.NewReader(encoded)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, reader)
	if err != nil {
		return failure("invalid_input", err.Error())
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	response, err := c.HTTP.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return failure("timeout", err.Error())
		}
		return &Error{Code: "execution_failed", Message: err.Error(), cause: err}
	}
	defer func() { _ = response.Body.Close() }()
	payload, err := io.ReadAll(io.LimitReader(response.Body, 8<<20))
	if err != nil {
		return &Error{Code: "execution_failed", Message: err.Error(), cause: err}
	}
	if response.StatusCode >= 300 {
		var typed Error
		if json.Unmarshal(payload, &typed) == nil && typed.Code != "" {
			return &typed
		}
		return &Error{Code: fmt.Sprintf("http_%d", response.StatusCode), Message: strings.TrimSpace(string(payload))}
	}
	if c.EnvironmentID != "" && response.Header.Get("X-Test-Environment-Id") != c.EnvironmentID {
		return failure("scope_conflict", "control endpoint belongs to another environment")
	}
	if target == nil {
		return nil
	}
	if err := json.Unmarshal(payload, target); err != nil {
		return &Error{Code: "execution_failed", Message: "decode response: " + err.Error()}
	}
	return nil
}

func scopePath(scopeID string) string { return "/__test__/scopes/" + url.PathEscape(scopeID) }

func (c *Client) Definitions(ctx context.Context) ([]Definition, error) {
	var out []Definition
	return out, c.do(ctx, http.MethodGet, "/__test__/scenarios", nil, &out)
}
func (c *Client) Controls(ctx context.Context) (ControlsDescription, error) {
	var out ControlsDescription
	return out, c.do(ctx, http.MethodGet, "/__test__/controls", nil, &out)
}
func (c *Client) ReserveScope(ctx context.Context, id string) error {
	return c.do(ctx, http.MethodPost, "/__test__/scopes", map[string]string{"scopeId": id}, nil)
}
func (c *Client) Prepare(ctx context.Context, input Input) (Result, error) {
	var out Result
	return out, c.do(ctx, http.MethodPost, "/__test__/scenarios/prepare", input, &out)
}
func (c *Client) Dispose(ctx context.Context, id string) error {
	return c.do(ctx, http.MethodDelete, scopePath(id), nil, nil)
}
func (c *Client) Manifest(ctx context.Context, id string) (Manifest, error) {
	var out Manifest
	return out, c.do(ctx, http.MethodGet, scopePath(id)+"/manifest", nil, &out)
}
func (c *Client) Replay(ctx context.Context, id string, m Manifest) (Result, error) {
	var out Result
	return out, c.do(ctx, http.MethodPost, scopePath(id)+"/replay", m, &out)
}

// AdvanceClock moves the dedicated environment clock and returns the new time.
func (c *Client) AdvanceClock(ctx context.Context, scopeID string, d time.Duration) (time.Time, error) {
	var out struct {
		Now time.Time `json:"now"`
	}
	err := c.do(ctx, http.MethodPost, scopePath(scopeID)+"/clock/advance", map[string]int64{"durationMs": d.Milliseconds()}, &out)
	return out.Now, err
}

// RunJob starts a registered job and returns its execution ID.
func (c *Client) RunJob(ctx context.Context, scopeID, job string, refs map[string]EntityRef) (string, error) {
	var out struct {
		ExecutionID string `json:"executionId"`
	}
	err := c.do(ctx, http.MethodPost, scopePath(scopeID)+"/jobs", map[string]any{"job": job, "refs": refs}, &out)
	return out.ExecutionID, err
}

// WaitJob waits for a condition. Failed and timeout outcomes return a typed
// error; the WaitResult is decoded from its details when present.
func (c *Client) WaitJob(ctx context.Context, scopeID, executionID, condition string, timeoutMs int) (WaitResult, error) {
	var out WaitResult
	err := c.do(ctx, http.MethodPost, scopePath(scopeID)+"/executions/"+url.PathEscape(executionID)+"/wait", map[string]any{"condition": condition, "timeoutMs": timeoutMs}, &out)
	if typed, ok := err.(*Error); ok && typed.Details != nil {
		if encoded, encodeErr := json.Marshal(typed.Details); encodeErr == nil {
			_ = json.Unmarshal(encoded, &out)
		}
	}
	return out, err
}

func providerPath(scopeID, provider string) string {
	return scopePath(scopeID) + "/providers/" + url.PathEscape(provider)
}

// ScriptProvider appends scripted steps for provider in scopeID.
func (c *Client) ScriptProvider(ctx context.Context, scopeID, provider string, script ProviderScript) error {
	return c.do(ctx, http.MethodPost, providerPath(scopeID, provider)+"/scripts", script, nil)
}

// ProviderCalls returns recorded calls.
func (c *Client) ProviderCalls(ctx context.Context, scopeID, provider string) ([]ProviderCall, error) {
	var out struct {
		Calls []ProviderCall `json:"calls"`
	}
	err := c.do(ctx, http.MethodGet, providerPath(scopeID, provider)+"/calls", nil, &out)
	return out.Calls, err
}

// WaitHeld waits until a call is held at gate.
func (c *Client) WaitHeld(ctx context.Context, scopeID, provider, gate string, timeoutMs int) error {
	return c.do(ctx, http.MethodPost, providerPath(scopeID, provider)+"/gates/"+url.PathEscape(gate)+"/wait", map[string]int{"timeoutMs": timeoutMs}, nil)
}

// ReleaseGate releases a hold gate.
func (c *Client) ReleaseGate(ctx context.Context, scopeID, provider, gate string) error {
	return c.do(ctx, http.MethodPost, providerPath(scopeID, provider)+"/gates/"+url.PathEscape(gate)+"/release", nil, nil)
}

// ForgetProviders removes the scope's own scripts and calls.
func (c *Client) ForgetProviders(ctx context.Context, scopeID string) error {
	return c.do(ctx, http.MethodDelete, scopePath(scopeID)+"/providers", nil, nil)
}

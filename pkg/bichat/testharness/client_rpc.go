package testharness

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/bichat/types"
)

type NotAuthenticatedRedirectError struct {
	StatusCode  int
	Location    string
	ContentType string
	BodySnippet string
}

func (e *NotAuthenticatedRedirectError) Error() string {
	loc := e.Location
	if loc == "" {
		loc = "<missing>"
	}
	ct := e.ContentType
	if ct == "" {
		ct = "<missing>"
	}
	return fmt.Sprintf("not authenticated (redirect): status=%d location=%s content_type=%s", e.StatusCode, loc, ct)
}

type UnexpectedContentTypeError struct {
	EndpointURL  string
	ContentType  string
	BodySnippet  string
	StatusCode   int
	ExpectedHint string
}

func (e *UnexpectedContentTypeError) Error() string {
	ct := e.ContentType
	if ct == "" {
		ct = "<missing>"
	}
	return fmt.Sprintf("unexpected content-type: status=%d content_type=%s expected=%s", e.StatusCode, ct, e.ExpectedHint)
}

type HTTPStatusError struct {
	EndpointURL  string
	StatusCode   int
	ContentType  string
	BodySnippet  string
	ExpectedHint string
}

func (e *HTTPStatusError) Error() string {
	ct := e.ContentType
	if ct == "" {
		ct = "<missing>"
	}
	return fmt.Sprintf("http status %d content_type=%s", e.StatusCode, ct)
}

type RPCMethodError struct {
	Method  string
	Code    string
	Message string
	Details any
}

func (e *RPCMethodError) Error() string {
	return fmt.Sprintf("rpc method %q failed: code=%s message=%s", e.Method, e.Code, e.Message)
}

type RPCClient struct {
	httpClient   *http.Client
	endpointURL  string
	cookieName   string
	sessionToken string
}

func NewRPCClient(cfg Config) *RPCClient {
	return &RPCClient{
		httpClient:   http.DefaultClient,
		endpointURL:  strings.TrimRight(cfg.ServerURL, "/") + cfg.RPCEndpointPath,
		cookieName:   cfg.CookieName,
		sessionToken: cfg.SessionToken,
	}
}

func (c *RPCClient) WithHTTPClient(client *http.Client) *RPCClient {
	if client != nil {
		c.httpClient = client
	}
	return c
}

type rpcRequest struct {
	ID     string `json:"id"`
	Method string `json:"method"`
	Params any    `json:"params"`
}

type rpcResponse struct {
	ID     string          `json:"id"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details any    `json:"details,omitempty"`
}

func (c *RPCClient) Do(ctx context.Context, method string, params any, out any) error {
	if c.httpClient == nil {
		return errors.New("http client is nil")
	}

	method = strings.TrimSpace(method)
	if method == "" {
		return errors.New("rpc method is required")
	}

	body := rpcRequest{
		ID:     uuid.NewString(),
		Method: method,
		Params: params,
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(body); err != nil {
		return fmt.Errorf("encode rpc request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpointURL, &buf)
	if err != nil {
		return fmt.Errorf("create rpc request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.cookieName != "" && c.sessionToken != "" {
		req.Header.Set("Cookie", fmt.Sprintf("%s=%s", c.cookieName, c.sessionToken))
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("rpc request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		snippet := readSnippet(resp.Body, 512)
		return &NotAuthenticatedRedirectError{
			StatusCode:  resp.StatusCode,
			Location:    resp.Header.Get("Location"),
			ContentType: resp.Header.Get("Content-Type"),
			BodySnippet: snippet,
		}
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &HTTPStatusError{
			EndpointURL: c.endpointURL,
			StatusCode:  resp.StatusCode,
			ContentType: resp.Header.Get("Content-Type"),
			BodySnippet: readSnippet(resp.Body, 512),
		}
	}

	ct := resp.Header.Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		return &UnexpectedContentTypeError{
			EndpointURL:  c.endpointURL,
			StatusCode:   resp.StatusCode,
			ContentType:  ct,
			BodySnippet:  readSnippet(resp.Body, 512),
			ExpectedHint: "application/json",
		}
	}

	var rpcResp rpcResponse
	if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
		return fmt.Errorf("decode rpc response: %w", err)
	}
	if rpcResp.Error != nil {
		return &RPCMethodError{
			Method:  method,
			Code:    strings.TrimSpace(rpcResp.Error.Code),
			Message: strings.TrimSpace(rpcResp.Error.Message),
			Details: rpcResp.Error.Details,
		}
	}
	if out == nil {
		return nil
	}
	if len(rpcResp.Result) == 0 {
		return errors.New("rpc response missing result")
	}
	if err := json.Unmarshal(rpcResp.Result, out); err != nil {
		return fmt.Errorf("decode rpc result: %w", err)
	}
	return nil
}

func readSnippet(r io.Reader, limit int64) string {
	if r == nil {
		return ""
	}
	b, _ := io.ReadAll(io.LimitReader(r, limit))
	s := strings.TrimSpace(string(b))
	if s == "" {
		return ""
	}
	return s
}

type ToolCall struct {
	ID         string `json:"id,omitempty"`
	Name       string `json:"name"`
	Arguments  string `json:"arguments"`
	Result     string `json:"result,omitempty"`
	Error      string `json:"error,omitempty"`
	DurationMS int64  `json:"durationMs,omitempty"`
}

type RPCSession struct {
	Session RPCSessionInfo        `json:"session"`
	Turns   []RPCConversationTurn `json:"turns"`
}

type RPCSessionInfo struct {
	ID string `json:"id"`
}

type RPCConversationTurn struct {
	ID            string            `json:"id"`
	AssistantTurn *RPCAssistantTurn `json:"assistantTurn,omitempty"`
}

type RPCAssistantTurn struct {
	ID        string         `json:"id"`
	Role      string         `json:"role,omitempty"`
	Content   string         `json:"content"`
	ToolCalls []ToolCall     `json:"toolCalls,omitempty"`
	Debug     *RPCDebugTrace `json:"debug,omitempty"`
}

type RPCDebugTrace struct {
	Usage *RPCDebugUsage `json:"usage,omitempty"`
}

type RPCDebugUsage struct {
	PromptTokens     int     `json:"promptTokens"`
	CompletionTokens int     `json:"completionTokens"`
	TotalTokens      int     `json:"totalTokens"`
	CachedTokens     int     `json:"cachedTokens"`
	Cost             float64 `json:"cost"`
}

func (u *RPCDebugUsage) ToDebugUsage() *types.DebugUsage {
	if u == nil {
		return nil
	}
	return &types.DebugUsage{
		PromptTokens:     u.PromptTokens,
		CompletionTokens: u.CompletionTokens,
		TotalTokens:      u.TotalTokens,
		CachedTokens:     u.CachedTokens,
		Cost:             u.Cost,
	}
}

func (c *RPCClient) CreateSession(ctx context.Context, title string) (uuid.UUID, error) {
	var out struct {
		Session struct {
			ID string `json:"id"`
		} `json:"session"`
	}
	if err := c.Do(ctx, "bichat.session.create", map[string]any{"title": title}, &out); err != nil {
		return uuid.Nil, err
	}
	return uuid.Parse(strings.TrimSpace(out.Session.ID))
}

func (c *RPCClient) GetSession(ctx context.Context, sessionID uuid.UUID) (*RPCSession, error) {
	var out RPCSession
	if err := c.Do(ctx, "bichat.session.get", map[string]any{"id": sessionID.String()}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

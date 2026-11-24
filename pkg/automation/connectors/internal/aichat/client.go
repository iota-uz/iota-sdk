package aichat

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"

	"code.dny.dev/ssrf"

	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
)

const (
	// maxResponse bounds the body read from a provider.
	maxResponse = 8 << 20
	// maxRetryAfter bounds a Retry-After the provider asks for; the engine repeats only within the node's
	// deadline anyway.
	maxRetryAfter = time.Hour
	// maxMessage bounds the provider text kept in a CallError message.
	maxMessage = 200
)

// Endpoint is an OpenAI-compatible API.
type Endpoint struct {
	BaseURL string // https://api.openai.com/v1
	APIKey  string
	// OpenRouter: ask the provider to report the cost and to route only to backends that support the
	// request's structured output.
	OpenRouter bool
	// Guarded: BaseURL comes from a tenant, so the connection is made only to public addresses (checked
	// when the connection is made, after DNS), on port 443 or 8443.
	Guarded bool
}

// Image is a picture of a request.
type Image struct {
	MediaType string
	Data      []byte
}

// JSONFormat asks for the answer as JSON of a schema in the provider's strict mode.
type JSONFormat struct {
	Name   string
	Schema json.RawMessage
}

// Request is one chat completion.
type Request struct {
	Model       string
	System      string
	Prompt      string
	Images      []Image
	MaxTokens   int
	Temperature *float64
	JSON        *JSONFormat // nil: text
}

// Usage is what a response reports it spent.
type Usage struct {
	InputTokens  int      `json:"input_tokens"`
	OutputTokens int      `json:"output_tokens"`
	CostUSD      *float64 `json:"cost_usd"` // nil: the provider did not report it
}

// Response is the answer of a provider.
type Response struct {
	Text         string
	Model        string // the model the provider reports; empty when it does not
	FinishReason string
	Usage        Usage
}

// Failure is how a call that did not give an answer ended.
type Failure struct {
	Err *catalog.CallError
	// Retry: the provider asked to repeat the call (429); RetryAfter is its Retry-After, 0 when it gave
	// none.
	Retry      bool
	RetryAfter time.Duration
}

// Result is the failure as a node's Execute returns it: a Retry-After the provider gave is a result the
// engine repeats after; any other failure is a CallError.
func (f *Failure) Result() (catalog.CallResult, error) {
	if f.Retry && f.RetryAfter > 0 {
		return catalog.CallResult{RetryAfter: f.RetryAfter}, nil
	}
	return catalog.CallResult{}, f.Err
}

// Client sends requests. The zero value uses the default transport.
type Client struct {
	// Transport, when set, carries every request (tests point it at a fake provider); the SSRF guard of a
	// guarded endpoint is then the transport's business.
	Transport http.RoundTripper
}

var guardedTransport = func() http.RoundTripper {
	g := ssrf.New(ssrf.WithPorts(443, 8443))
	d := &net.Dialer{Timeout: 10 * time.Second, Control: g.Safe}
	return &http.Transport{
		DialContext:           d.DialContext,
		ForceAttemptHTTP2:     true,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 5 * time.Minute,
		MaxIdleConnsPerHost:   4,
		IdleConnTimeout:       60 * time.Second,
	}
}()

func (c *Client) httpClient(ep Endpoint) *http.Client {
	rt := c.Transport
	if rt == nil {
		rt = http.DefaultTransport
		if ep.Guarded {
			rt = guardedTransport
		}
	}
	return &http.Client{
		Transport: rt,
		// A redirect would send the key to another host.
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
}

// CheckBaseURL reports why s cannot be the base URL of an endpoint: it is not an https URL, or it carries
// credentials, a query or a fragment.
func CheckBaseURL(s string) error {
	u, err := url.Parse(s)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return errors.New("the base URL is an https URL without credentials, query or fragment")
	}
	return nil
}

func (ep Endpoint) url(path string) string { return strings.TrimRight(ep.BaseURL, "/") + path }

// useCompletionTokens reports whether the endpoint is OpenAI's own, whose newer models take
// max_completion_tokens and refuse max_tokens; compatible endpoints know max_tokens.
func (ep Endpoint) useCompletionTokens() bool {
	u, err := url.Parse(ep.BaseURL)
	return err == nil && strings.EqualFold(u.Hostname(), "api.openai.com")
}

type chatRequest struct {
	Model               string          `json:"model"`
	Messages            []chatMessage   `json:"messages"`
	MaxTokens           int             `json:"max_tokens,omitempty"`
	MaxCompletionTokens int             `json:"max_completion_tokens,omitempty"`
	Temperature         *float64        `json:"temperature,omitempty"`
	ResponseFormat      *responseFormat `json:"response_format,omitempty"`
	Usage               *usageOption    `json:"usage,omitempty"`
	Provider            *providerOption `json:"provider,omitempty"`
}

type usageOption struct {
	Include bool `json:"include"`
}

type providerOption struct {
	RequireParameters bool `json:"require_parameters"`
}

type responseFormat struct {
	Type       string `json:"type"`
	JSONSchema struct {
		Name   string          `json:"name"`
		Strict bool            `json:"strict"`
		Schema json.RawMessage `json:"schema"`
	} `json:"json_schema"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content any    `json:"content"`
}

type contentPart struct {
	Type     string    `json:"type"`
	Text     string    `json:"text,omitempty"`
	ImageURL *imageURL `json:"image_url,omitempty"`
}

type imageURL struct {
	URL string `json:"url"`
}

// Body is the JSON body of a request to the endpoint; tests read it from the fake provider.
func (ep Endpoint) body(r Request) ([]byte, error) {
	req := chatRequest{Model: r.Model, Temperature: r.Temperature}
	if ep.useCompletionTokens() {
		req.MaxCompletionTokens = r.MaxTokens
	} else {
		req.MaxTokens = r.MaxTokens
	}
	if r.System != "" {
		req.Messages = append(req.Messages, chatMessage{Role: "system", Content: r.System})
	}
	if len(r.Images) == 0 {
		req.Messages = append(req.Messages, chatMessage{Role: "user", Content: r.Prompt})
	} else {
		parts := []contentPart{{Type: "text", Text: r.Prompt}}
		for _, img := range r.Images {
			parts = append(parts, contentPart{Type: "image_url", ImageURL: &imageURL{
				URL: "data:" + img.MediaType + ";base64," + base64.StdEncoding.EncodeToString(img.Data),
			}})
		}
		req.Messages = append(req.Messages, chatMessage{Role: "user", Content: parts})
	}
	if r.JSON != nil {
		rf := &responseFormat{Type: "json_schema"}
		rf.JSONSchema.Name, rf.JSONSchema.Strict, rf.JSONSchema.Schema = r.JSON.Name, true, r.JSON.Schema
		req.ResponseFormat = rf
		if ep.OpenRouter {
			req.Provider = &providerOption{RequireParameters: true}
		}
	}
	if ep.OpenRouter {
		req.Usage = &usageOption{Include: true}
	}
	return json.Marshal(req)
}

// chatEnvelope is the answer read loosely: a field of an unexpected type is a malformed answer, told from a
// body that is not JSON at all, and the usage the answer reports is kept either way (the tokens were spent).
type chatEnvelope struct {
	Model   json.RawMessage `json:"model"`
	Choices json.RawMessage `json:"choices"`
	Usage   json.RawMessage `json:"usage"`
}

type chatChoice struct {
	FinishReason json.RawMessage `json:"finish_reason"`
	Message      json.RawMessage `json:"message"`
}

type chatMessageBody struct {
	Content json.RawMessage `json:"content"`
	Refusal json.RawMessage `json:"refusal"`
}

// stringValue reads a JSON string; anything else is "" (a null, a number, an object).
func stringValue(raw json.RawMessage) (string, bool) {
	var s string
	if len(raw) == 0 || json.Unmarshal(raw, &s) != nil {
		return "", false
	}
	return s, true
}

type apiError struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    any    `json:"code"`
	} `json:"error"`
}

// Complete sends one chat completion. On a refusal or an answer that cannot be used it returns the
// Response too, with the usage the provider reported: the tokens were spent and are accounted.
func (c *Client) Complete(ctx context.Context, ep Endpoint, r Request) (Response, *Failure) {
	body, err := ep.body(r)
	if err != nil {
		return Response{}, fail(catalog.CallInvalidRequest, "the request is not JSON", false)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, ep.url("/chat/completions"), bytes.NewReader(body))
	if err != nil {
		return Response{}, fail(catalog.CallInvalidRequest, "the endpoint URL is not valid", false)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+ep.APIKey)
	resp, err := c.httpClient(ep).Do(req)
	if err != nil {
		return Response{}, networkFailure(ctx, err, ep.APIKey)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxResponse+1))
	if err != nil {
		return Response{}, networkFailure(ctx, err, ep.APIKey)
	}
	if len(raw) > maxResponse {
		return Response{}, fail(catalog.CallUnavailable, "the provider's answer is too large", false)
	}
	if resp.StatusCode != http.StatusOK {
		return Response{}, statusFailure(resp, raw, ep.APIKey)
	}
	var env chatEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		// Not a JSON object at all (a proxy's error page): the provider may answer properly next time.
		return Response{}, fail(catalog.CallUnavailable, "the provider's answer is not JSON", true)
	}
	out := Response{}
	out.Model, _ = stringValue(env.Model)
	out.Usage = readUsage(env.Usage)
	malformed := func(msg string) (Response, *Failure) {
		return out, fail(catalog.CallInvalidRequest, msg, false)
	}
	var choices []json.RawMessage
	if json.Unmarshal(env.Choices, &choices) != nil {
		return malformed("the provider's answer has malformed choices")
	}
	if len(choices) == 0 {
		return malformed("the provider's answer has no choices")
	}
	var ch chatChoice
	if json.Unmarshal(choices[0], &ch) != nil {
		return malformed("the provider's answer has a malformed choice")
	}
	if len(ch.FinishReason) > 0 && string(ch.FinishReason) != "null" {
		reason, ok := stringValue(ch.FinishReason)
		if !ok {
			return malformed("the provider's finish reason is not a string")
		}
		out.FinishReason = reason
	}
	var msg chatMessageBody
	if json.Unmarshal(ch.Message, &msg) != nil {
		return malformed("the provider's answer has a malformed message")
	}
	var content any
	if len(msg.Content) > 0 && json.Unmarshal(msg.Content, &content) != nil {
		return malformed("the provider's message content is malformed")
	}
	out.Text = contentText(content)
	refusal, _ := stringValue(msg.Refusal)
	switch {
	case refusal != "":
		return out, fail(catalog.CallRejected, "the model refused: "+scrub(refusal, ep.APIKey), false)
	case out.FinishReason == "content_filter":
		return out, fail(catalog.CallRejected, "the answer was blocked by the provider's content filter", false)
	}
	return out, nil
}

// readUsage reads the usage an answer reports; one that is missing or malformed is none (an answer
// whose usage cannot be read cannot be accounted).
func readUsage(raw json.RawMessage) Usage {
	var u struct {
		PromptTokens     int      `json:"prompt_tokens"`
		CompletionTokens int      `json:"completion_tokens"`
		InputTokens      int      `json:"input_tokens"`
		OutputTokens     int      `json:"output_tokens"`
		Cost             *float64 `json:"cost"`
	}
	if len(raw) == 0 || json.Unmarshal(raw, &u) != nil {
		return Usage{}
	}
	in, outTokens := u.PromptTokens, u.CompletionTokens
	if in == 0 && outTokens == 0 {
		in, outTokens = u.InputTokens, u.OutputTokens
	}
	return Usage{InputTokens: in, OutputTokens: outTokens, CostUSD: u.Cost}
}

// contentText is the text of a message's content: a string, or the text parts of a list.
func contentText(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case []any:
		var b strings.Builder
		for _, p := range x {
			if m, ok := p.(map[string]any); ok {
				if t, ok := m["text"].(string); ok {
					b.WriteString(t)
				}
			}
		}
		return b.String()
	}
	return ""
}

func fail(code, msg string, retryable bool) *Failure {
	return &Failure{Err: &catalog.CallError{Code: code, Message: msg, Retryable: retryable}}
}

// networkFailure: no answer came. The engine repeats the idempotent call within its deadline.
func networkFailure(ctx context.Context, err error, secrets ...string) *Failure {
	if ctx.Err() != nil {
		return fail(catalog.CallUnavailable, "the provider did not answer by the deadline", true)
	}
	return fail(catalog.CallUnavailable, "the provider was not reached: "+scrub(rootError(err), secrets...), true)
}

func rootError(err error) string {
	var ue *url.Error
	if errors.As(err, &ue) {
		return ue.Err.Error()
	}
	return err.Error()
}

// statusFailure sorts a non-200 answer into a CallError code.
func statusFailure(resp *http.Response, raw []byte, key string) *Failure {
	var ae apiError
	_ = json.Unmarshal(raw, &ae)
	detail := scrub(ae.Error.Message, key)
	code, _ := ae.Error.Code.(string)
	kind := ae.Error.Type
	with := func(msg string) string {
		if detail != "" {
			return msg + ": " + detail
		}
		return msg
	}
	switch status := resp.StatusCode; {
	case status == http.StatusTooManyRequests:
		if code == "insufficient_quota" || kind == "insufficient_quota" {
			// Repeating cannot help: the account has no quota left.
			return fail(catalog.CallUnavailable, with("the provider's quota is exhausted"), false)
		}
		after := parseRetryAfter(resp.Header.Get("Retry-After"), time.Now())
		return &Failure{Retry: true, RetryAfter: after,
			Err: &catalog.CallError{Code: catalog.CallRateLimited, Message: with("the provider limits the rate of requests"), Retryable: true, RetryAfter: after}}
	case status == http.StatusUnauthorized || status == http.StatusForbidden:
		return fail(catalog.CallUnauthorized, "the provider refused the API key", false)
	case status == http.StatusRequestTimeout || status == http.StatusConflict || status >= 500:
		f := fail(catalog.CallUnavailable, with("the provider is unavailable"), true)
		// The engine repeats the call after the provider's Retry-After at least, and keeps unavailable if
		// the attempts run out.
		f.Err.RetryAfter = parseRetryAfter(resp.Header.Get("Retry-After"), time.Now())
		return f
	case code == "content_filter" || code == "content_policy_violation" || kind == "content_policy_violation" || code == "moderation_blocked":
		return fail(catalog.CallRejected, with("the provider's content policy refused the request"), false)
	case status == http.StatusPaymentRequired:
		return fail(catalog.CallUnavailable, with("the provider's account has no credit"), false)
	case code == "context_length_exceeded":
		return fail(catalog.CallInvalidRequest, with("the request exceeds the context of the model"), false)
	}
	return fail(catalog.CallInvalidRequest, with("the provider refused the request"), false)
}

// parseRetryAfter reads a Retry-After header: seconds or an HTTP date; 0 when absent or unusable.
func parseRetryAfter(v string, now time.Time) time.Duration {
	if v == "" {
		return 0
	}
	var d time.Duration
	if n, err := strconv.ParseInt(v, 10, 64); err == nil && n >= 0 {
		if n > int64(maxRetryAfter/time.Second) {
			return maxRetryAfter
		}
		d = time.Duration(n) * time.Second
	} else if at, err := http.ParseTime(v); err == nil {
		d = at.Sub(now)
	}
	return min(max(d, 0), maxRetryAfter)
}

// scrub makes provider text fit for a CallError message: control characters out, the secrets replaced,
// bounded.
func scrub(s string, secrets ...string) string {
	for _, sec := range secrets {
		if len(sec) >= 3 {
			s = strings.ReplaceAll(s, sec, "***")
		}
	}
	s = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, s)
	s = strings.TrimSpace(s)
	if r := []rune(s); len(r) > maxMessage {
		s = string(r[:maxMessage])
	}
	return s
}

// Scrub is scrub for the connectors' own messages.
func Scrub(s string, secrets ...string) string { return scrub(s, secrets...) }

// StatusError sorts the failed answer of a read (a connector's Test) as a call's is.
func StatusError(resp *http.Response, raw []byte, key string) *catalog.CallError {
	f := statusFailure(resp, raw, key)
	return f.Err
}

// String shows what a Failure is without its detail, for tests.
func (f *Failure) String() string {
	if f == nil || f.Err == nil {
		return "<nil>"
	}
	return fmt.Sprintf("%s: %s", f.Err.Code, f.Err.Message)
}

// Do sends a read request (a connector's Test) through the client's transport: a tenant's endpoint is
// guarded as in Complete.
func (c *Client) Do(ep Endpoint, req *http.Request) (*http.Response, error) {
	return c.httpClient(ep).Do(req)
}

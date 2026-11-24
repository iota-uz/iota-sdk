package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptrace"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
)

// Methods of the protocol the client sends and observes.
const (
	methodInitialize = "initialize"
	methodToolsList  = "tools/list"
	methodToolsCall  = "tools/call"
)

// Bounds of a tools/list.
const (
	maxTools     = 1000
	maxListPages = 50
)

type serverInfo struct {
	Name    string
	Version string
}

// observer is the HTTP of one session: it authorizes requests to the server's origin, caps responses
// and records what the classification of a failure needs (was the request of a method written, what
// status came back).
type observer struct {
	next   http.RoundTripper
	origin string // scheme://host of the server URL
	token  string

	mu         sync.Mutex
	written    map[string]bool
	status     map[string]int
	retryAfter time.Duration
	tooLarge   bool
	blocked    bool
	complete   map[string]bool // a response of the method was read to its end
}

func (o *observer) markBlocked() {
	o.mu.Lock()
	o.blocked = true
	o.mu.Unlock()
}

func (o *observer) wasBlocked() bool {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.blocked
}

func newObserver(origin, token string) *observer {
	return &observer{origin: origin, token: token, written: map[string]bool{}, status: map[string]int{}, complete: map[string]bool{}}
}

var errTooLarge = errors.New("mcp: the response is larger than 1 MiB")

// closeTimeout bounds the DELETE that ends a session: the SDK sends it without a deadline.
const closeTimeout = 5 * time.Second

func (o *observer) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Method == http.MethodDelete {
		return o.endSession(req)
	}
	method := ""
	if req.Body != nil && req.Method == http.MethodPost {
		body, err := io.ReadAll(io.LimitReader(req.Body, 8<<20))
		_ = req.Body.Close()
		if err != nil {
			return nil, err
		}
		var msg struct {
			Method string `json:"method"`
		}
		if json.Unmarshal(body, &msg) == nil {
			method = msg.Method
		}
		req = req.Clone(req.Context())
		req.Body = io.NopCloser(bytes.NewReader(body))
		req.GetBody = func() (io.ReadCloser, error) { return io.NopCloser(bytes.NewReader(body)), nil }
		req.ContentLength = int64(len(body))
	} else {
		req = req.Clone(req.Context())
	}
	req.Header.Del("Authorization")
	if o.token != "" && req.URL.Scheme+"://"+req.URL.Host == o.origin {
		req.Header.Set("Authorization", "Bearer "+o.token)
	}
	if method != "" {
		trace := &httptrace.ClientTrace{WroteRequest: func(info httptrace.WroteRequestInfo) {
			if info.Err == nil {
				o.mu.Lock()
				o.written[method] = true
				o.mu.Unlock()
			}
		}}
		req = req.WithContext(httptrace.WithClientTrace(req.Context(), trace))
	}
	res, err := o.next.RoundTrip(req)
	if err != nil {
		if errors.Is(err, ErrBlocked) {
			o.markBlocked()
		}
		return nil, err
	}
	o.mu.Lock()
	if method != "" {
		o.status[method] = res.StatusCode
	}
	if res.StatusCode == http.StatusTooManyRequests {
		o.retryAfter = retryAfter(res.Header.Get("Retry-After"))
	}
	o.mu.Unlock()
	// Only a served answer can hold the server's JSON-RPC response: a redirect or a refusal is not one.
	answered := method
	if res.StatusCode < 200 || res.StatusCode > 299 {
		answered = ""
	}
	res.Body = &cappedBody{r: res.Body, left: MaxResponseBytes, o: o, method: answered}
	return res, nil
}

// endSession sends the DELETE of a session within closeTimeout and reads and closes its answer itself
// (the SDK leaves the body open).
func (o *observer) endSession(req *http.Request) (*http.Response, error) {
	ctx, cancel := context.WithTimeout(req.Context(), closeTimeout)
	defer cancel()
	req = req.Clone(ctx)
	req.Header.Del("Authorization")
	if o.token != "" && req.URL.Scheme+"://"+req.URL.Host == o.origin {
		req.Header.Set("Authorization", "Bearer "+o.token)
	}
	res, err := o.next.RoundTrip(req)
	if err != nil {
		return nil, err
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(res.Body, 64<<10))
	_ = res.Body.Close()
	res.Body = http.NoBody
	return res, nil
}

func retryAfter(h string) time.Duration {
	h = strings.TrimSpace(h)
	if h == "" {
		return 0
	}
	if n, err := strconv.Atoi(h); err == nil && n >= 0 {
		return time.Duration(n) * time.Second
	}
	if t, err := http.ParseTime(h); err == nil {
		if d := time.Until(t); d > 0 {
			return d
		}
	}
	return 0
}

// cappedBody fails a response once it is larger than MaxResponseBytes.
type cappedBody struct {
	r      io.ReadCloser
	left   int64
	o      *observer
	method string
}

func (b *cappedBody) Read(p []byte) (int, error) {
	if b.left <= 0 {
		// The cap is reached: one more byte is an overflow, the end of the body is an exact fit.
		var one [1]byte
		m, err := b.r.Read(one[:])
		if m > 0 {
			b.o.mu.Lock()
			b.o.tooLarge = true
			b.o.mu.Unlock()
			return 0, errTooLarge
		}
		b.done(err)
		return 0, err
	}
	if int64(len(p)) > b.left {
		p = p[:b.left]
	}
	n, err := b.r.Read(p)
	b.left -= int64(n)
	b.done(err)
	return n, err
}

// done records a body of the method read to its end.
func (b *cappedBody) done(err error) {
	if errors.Is(err, io.EOF) && b.method != "" {
		b.o.mu.Lock()
		b.o.complete[b.method] = true
		b.o.mu.Unlock()
	}
}

func (b *cappedBody) Close() error { return b.r.Close() }

func (o *observer) wasWritten(method string) bool {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.written[method]
}

func (o *observer) statusOf(method string) int {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.status[method]
}

func (o *observer) overflowed() bool {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.tooLarge
}

// refusedMethods are the server's requests the client never serves: it offers no capabilities.
var refusedMethods = map[string]bool{
	"sampling/createMessage": true,
	"elicitation/create":     true,
	"roots/list":             true,
}

func refuseServerRequests(next mcpsdk.MethodHandler) mcpsdk.MethodHandler {
	return func(ctx context.Context, method string, req mcpsdk.Request) (mcpsdk.Result, error) {
		if refusedMethods[method] {
			return nil, &jsonrpc.Error{Code: jsonrpc.CodeMethodNotFound, Message: "the client does not serve " + method}
		}
		return next(ctx, method, req)
	}
}

// session is one open MCP session.
type session struct {
	cs   *mcpsdk.ClientSession
	obs  *observer
	info serverInfo
}

// close ends the session in the background: the answer is already in hand, and the DELETE (bounded by
// closeTimeout) must not hold it back from the engine.
func (s *session) close() {
	if s != nil && s.cs != nil {
		cs := s.cs
		go func() { _ = cs.Close() }()
	}
}

// open sends initialize (and notifications/initialized) to the server with a bearer token ("" none).
// A failure is a CallError: unauthorized (401), rate_limited (429), invalid_request (a protocol error, a
// response over the cap) or unavailable.
func (c *Connector) open(ctx context.Context, cf config, token string) (*session, error) {
	obs := newObserver(cf.server.Scheme+"://"+cf.server.Host, token)
	httpc := c.net.client(func(next http.RoundTripper) http.RoundTripper { obs.next = next; return obs }, obs.markBlocked)
	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: c.clientID, Version: "1"}, &mcpsdk.ClientOptions{Capabilities: &mcpsdk.ClientCapabilities{}})
	client.AddReceivingMiddleware(refuseServerRequests)
	tr := &mcpsdk.StreamableClientTransport{Endpoint: cf.server.String(), HTTPClient: httpc, MaxRetries: -1, DisableStandaloneSSE: true}
	cs, err := client.Connect(ctx, tr, nil)
	if err != nil {
		return nil, classify(obs, methodInitialize, err, false)
	}
	s := &session{cs: cs, obs: obs}
	if r := cs.InitializeResult(); r != nil && r.ServerInfo != nil {
		s.info = serverInfo{Name: clip(r.ServerInfo.Name, 200), Version: clip(r.ServerInfo.Version, 100)}
	}
	return s, nil
}

// list opens a session and lists the server's tools.
func (c *Connector) list(ctx context.Context, cf config, token string) (serverInfo, []Tool, error) {
	s, err := c.open(ctx, cf, token)
	if err != nil {
		return serverInfo{}, nil, err
	}
	defer s.close()
	var (
		out    []Tool
		cursor string
	)
	for page := 0; page < maxListPages; page++ {
		res, err := s.cs.ListTools(ctx, &mcpsdk.ListToolsParams{Cursor: cursor})
		if err != nil {
			return serverInfo{}, nil, classify(s.obs, methodToolsList, err, false)
		}
		for _, t := range res.Tools {
			if t == nil {
				continue
			}
			tool, ok := toolOf(t)
			if ok {
				out = append(out, tool)
			}
			if len(out) >= maxTools {
				return s.info, out, nil
			}
		}
		if res.NextCursor == "" {
			break
		}
		cursor = res.NextCursor
	}
	return s.info, out, nil
}

func toolOf(t *mcpsdk.Tool) (Tool, bool) {
	if t.Name == "" || len(t.Name) > 200 {
		return Tool{}, false
	}
	in, err := json.Marshal(t.InputSchema)
	if err != nil || string(in) == "null" {
		in = json.RawMessage(`{"type":"object"}`)
	}
	tool := Tool{Name: t.Name, Title: clip(t.Title, 200), Description: clip(t.Description, 4000), InputSchema: in}
	if t.OutputSchema != nil {
		if out, err := json.Marshal(t.OutputSchema); err == nil && string(out) != "null" {
			tool.OutputSchema = out
		}
	}
	if t.Annotations != nil {
		tool.ReadOnly = t.Annotations.ReadOnlyHint
		if tool.Title == "" {
			tool.Title = clip(t.Annotations.Title, 200)
		}
	}
	return tool, true
}

// callResult is the answer of a tools/call.
type callResult struct {
	res *mcpsdk.CallToolResult
}

// call opens a session and calls a tool once. Before the tools/call request is written a failure is
// what open reports; after it, a failure without an answer is outcome_unknown, never retried here.
func (c *Connector) call(ctx context.Context, cf config, token, tool string, args map[string]any) (callResult, error) {
	s, err := c.open(ctx, cf, token)
	if err != nil {
		return callResult{}, err
	}
	defer s.close()
	res, err := s.cs.CallTool(ctx, &mcpsdk.CallToolParams{Name: tool, Arguments: args})
	if err != nil {
		return callResult{}, classify(s.obs, methodToolsCall, err, true)
	}
	if res == nil {
		return callResult{}, &catalog.CallError{Code: catalog.CallInvalidRequest, Message: "the server answered tools/call without a result"}
	}
	return callResult{res: res}, nil
}

// classify turns a failure of a session's method into a CallError. effect: the method may change
// something (tools/call), so a request that was written and got no answer has an unknown outcome.
func classify(obs *observer, method string, err error, effect bool) error {
	status := obs.statusOf(method)
	switch {
	case obs.wasBlocked() || errors.Is(err, ErrBlocked):
		return &catalog.CallError{Code: catalog.CallUnavailable, Message: "the server's address is not allowed (private, loopback, link-local or metadata address, or not https on an allowed port)"}
	case status == http.StatusUnauthorized:
		// 401: the request was refused before it was served; a refreshed token may be sent again.
		return &tokenRefusedError{ce: &catalog.CallError{Code: catalog.CallUnauthorized, Message: "the server refused the credentials (HTTP 401)"}}
	case status == http.StatusForbidden && effect && obs.wasWritten(method):
		// 403 does not say the operation did not start: never sent again.
		return &catalog.CallError{Code: catalog.CallRejected, Message: "the server refused " + method + " with HTTP 403 after it was sent; the call is not repeated"}
	case status == http.StatusForbidden:
		return &catalog.CallError{Code: catalog.CallUnauthorized, Message: "the server refused the credentials (HTTP 403)"}
	case status == http.StatusTooManyRequests && effect && obs.wasWritten(method):
		// The call reached the server: a repeat may run the tool twice, so it is not asked for.
		return &catalog.CallError{Code: catalog.CallRejected, Message: "the server refused " + method + " with HTTP 429 after it was sent; the call is not repeated"}
	case status == http.StatusTooManyRequests:
		return &rateLimitedError{after: obs.retryAfter}
	case obs.overflowed():
		return &catalog.CallError{Code: catalog.CallInvalidRequest, Message: "the server's response is larger than 1 MiB"}
	}
	var we *jsonrpc.Error
	if errors.As(err, &we) && (!synthetic(we.Code) || obs.completed(method)) {
		return &catalog.CallError{Code: catalog.CallInvalidRequest, Message: "the server refused " + method + " (JSON-RPC error " + strconv.FormatInt(we.Code, 10) + ")"}
	}
	if effect && obs.wasWritten(method) {
		return &catalog.CallError{Code: catalog.CallOutcomeUnknown, Message: method + " was sent and no answer came: its outcome is unknown"}
	}
	if status >= 400 {
		return &catalog.CallError{Code: catalog.CallUnavailable, Message: "the server answered " + method + " with HTTP " + strconv.Itoa(status), Retryable: status >= 500}
	}
	return &catalog.CallError{Code: catalog.CallUnavailable, Message: "the server is unavailable", Retryable: true}
}

// synthetic are the JSON-RPC codes the SDK makes up for a transport failure (a closed connection, a
// rejected write): without a whole response from the server they are not the server's refusal.
func synthetic(code int64) bool {
	switch code {
	case -32000, -32001, -32003, -32004, -32005:
		return true
	}
	return false
}

func (o *observer) completed(method string) bool {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.complete[method]
}

// tokenRefusedError is HTTP 401: the only answer after which the call is sent again, once, with a refreshed
// token. It unwraps to the unauthorized CallError the engine sees.
type tokenRefusedError struct{ ce *catalog.CallError }

func (e *tokenRefusedError) Error() string { return e.ce.Error() }
func (e *tokenRefusedError) Unwrap() error { return e.ce }

// refused401 reports an HTTP 401 of the server.
func refused401(err error) bool {
	var t *tokenRefusedError
	return errors.As(err, &t)
}

// rateLimitedError is HTTP 429 before the request was served: after is the server's Retry-After (0: none).
type rateLimitedError struct{ after time.Duration }

func (e *rateLimitedError) Error() string {
	return catalog.CallRateLimited + ": the server asked to slow down (HTTP 429)"
}

func clip(s string, n int) string {
	if len(s) <= n {
		return s
	}
	cut := n
	for cut > 0 && !utf8RuneStart(s[cut]) {
		cut--
	}
	return s[:cut]
}

func utf8RuneStart(b byte) bool { return b&0xC0 != 0x80 }

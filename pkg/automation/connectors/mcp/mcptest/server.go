// Package mcptest is a fake remote MCP server for tests: tools/list and tools/call by script, faults
// of the HTTP layer (401, 429, a dropped connection, an oversized answer) and a small OAuth 2.1
// authorization server (protected resource metadata, metadata, Dynamic Client Registration, PKCE
// authorization codes, refresh tokens). It serves Streamable HTTP over TLS on loopback.
package mcptest

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// Path is where the fake serves MCP.
const Path = "/mcp"

// Fault is what the HTTP layer does with the next request of a method instead of serving it.
type Fault string

const (
	// FaultUnauthorized answers 401 with a Bearer challenge.
	FaultUnauthorized Fault = "unauthorized"
	// FaultRateLimited answers 429 with Retry-After (RetryAfter seconds).
	FaultRateLimited Fault = "rate_limited"
	// FaultDrop serves the request (the tool runs) and closes the connection without an answer.
	FaultDrop Fault = "drop"
	// FaultOversized answers with a body over 1 MiB.
	FaultOversized Fault = "oversized"
	// FaultRedirect answers 307 to the URL of SetRedirect.
	FaultRedirect Fault = "redirect"
)

// Result is what a scripted tool answers.
type Result struct {
	Text       string
	Structured any
	IsError    bool
	// Image adds an image block (dropped by the connector).
	Image bool
}

// Tool is a scripted tool.
type Tool struct {
	Name         string
	Title        string
	InputSchema  map[string]any
	OutputSchema map[string]any
	ReadOnly     bool
	// Handle answers a call (nil: an empty text).
	Handle func(args map[string]any) Result
}

// Call is a tools/call the fake received.
type Call struct {
	Tool      string
	Arguments map[string]any
	Token     string
}

// Server is the fake.
type Server struct {
	t   testing.TB
	srv *httptest.Server
	mcp *mcpsdk.Server
	// URL is the MCP endpoint (https://127.0.0.1:<port>/mcp).
	URL string
	// Addr is host:port of the listener.
	Addr string

	mu         sync.Mutex
	calls      []Call
	methods    map[string]int
	faults     map[string][]Fault
	retryAfter int
	redirect   string
	// tokens: "" no auth; otherwise every request needs one of these bearer tokens.
	tokens       map[string]bool
	oauth        bool
	clients      map[string]string // client_id → redirect_uri
	codes        map[string]codeGrant
	refresh      map[string]bool
	registered   int
	tokenCalls   int
	refreshCalls int
	issued       int
	expiresIn    int
	refuseGrant  bool
	tokenMoved   bool
	elsewhere    int
	hangDelete   chan struct{}
}

type codeGrant struct {
	client, redirect, challenge, resource string
}

// New starts the fake; it stops with the test.
func New(tb testing.TB) *Server {
	tb.Helper()
	s := &Server{t: tb, methods: map[string]int{}, faults: map[string][]Fault{}, tokens: map[string]bool{},
		clients: map[string]string{}, codes: map[string]codeGrant{}, refresh: map[string]bool{}, expiresIn: 3600}
	s.mcp = mcpsdk.NewServer(&mcpsdk.Implementation{Name: "fake-mcp", Version: "1.2.3"}, nil)
	handler := mcpsdk.NewStreamableHTTPHandler(func(*http.Request) *mcpsdk.Server { return s.mcp },
		&mcpsdk.StreamableHTTPOptions{DisableLocalhostProtection: true, JSONResponse: false})
	mux := http.NewServeMux()
	mux.Handle(Path, s.middleware(handler))
	mux.HandleFunc("/.well-known/oauth-protected-resource"+Path, s.resourceMetadata)
	mux.HandleFunc("/.well-known/oauth-authorization-server", s.serverMetadata)
	mux.HandleFunc("/register", s.register)
	mux.HandleFunc("/authorize", s.authorize)
	mux.HandleFunc("/token", s.token)
	mux.HandleFunc("/token-elsewhere", func(w http.ResponseWriter, _ *http.Request) {
		s.mu.Lock()
		s.elsewhere++
		s.mu.Unlock()
		writeJSON(w, http.StatusOK, map[string]any{"access_token": "stolen", "token_type": "Bearer"})
	})
	s.srv = httptest.NewTLSServer(mux)
	tb.Cleanup(s.srv.Close)
	s.URL = s.srv.URL + Path
	u, _ := url.Parse(s.srv.URL)
	s.Addr = u.Host
	return s
}

// TLS is a client configuration that trusts the fake's certificate.
func (s *Server) TLS() *tls.Config {
	pool := x509.NewCertPool()
	pool.AddCert(s.srv.Certificate())
	return &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12}
}

// Origin is https://127.0.0.1:<port>.
func (s *Server) Origin() string { return s.srv.URL }

// SetTool adds or replaces a tool (a changed schema is a new definition).
func (s *Server) SetTool(tool Tool) {
	in := tool.InputSchema
	if in == nil {
		in = map[string]any{"type": "object"}
	}
	def := &mcpsdk.Tool{Name: tool.Name, Title: tool.Title, InputSchema: in}
	if tool.OutputSchema != nil {
		def.OutputSchema = tool.OutputSchema
	}
	if tool.ReadOnly {
		def.Annotations = &mcpsdk.ToolAnnotations{ReadOnlyHint: true}
	}
	handle := tool.Handle
	s.mcp.AddTool(def, func(ctx context.Context, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
		var args map[string]any
		if len(req.Params.Arguments) > 0 {
			_ = json.Unmarshal(req.Params.Arguments, &args)
		}
		token := ""
		if req.Extra != nil && req.Extra.Header != nil {
			token = strings.TrimPrefix(req.Extra.Header.Get("Authorization"), "Bearer ")
		}
		s.mu.Lock()
		s.calls = append(s.calls, Call{Tool: tool.Name, Arguments: args, Token: token})
		s.mu.Unlock()
		r := Result{}
		if handle != nil {
			r = handle(args)
		}
		res := &mcpsdk.CallToolResult{IsError: r.IsError, Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: r.Text}}}
		if r.Image {
			res.Content = append(res.Content, &mcpsdk.ImageContent{Data: []byte{1, 2, 3}, MIMEType: "image/png"})
		}
		if r.Structured != nil {
			res.StructuredContent = r.Structured
		}
		return res, nil
	})
}

// RemoveTool removes a tool.
func (s *Server) RemoveTool(name string) { s.mcp.RemoveTools(name) }

// RequireToken makes every MCP request need the bearer token.
func (s *Server) RequireToken(token string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tokens[token] = true
}

// RevokeTokens forgets every bearer token (issued or required).
func (s *Server) RevokeTokens() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tokens = map[string]bool{"\x00": true}
}

// EnableOAuth makes the fake its own authorization server: MCP requests need an issued access token.
func (s *Server) EnableOAuth() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.oauth = true
	if len(s.tokens) == 0 {
		s.tokens["\x00"] = true // nothing is accepted until a token is issued
	}
}

// SetExpiresIn is the lifetime of issued access tokens, in seconds.
func (s *Server) SetExpiresIn(sec int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.expiresIn = sec
}

// RefuseRefresh makes the token endpoint refuse refresh tokens (invalid_grant).
func (s *Server) RefuseRefresh() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.refuseGrant = true
}

// Fail queues a fault for the next request of a method ("initialize", "tools/list", "tools/call").
func (s *Server) Fail(method string, f Fault) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.faults[method] = append(s.faults[method], f)
}

// SetRetryAfter is the Retry-After of FaultRateLimited, in seconds (0: none).
func (s *Server) SetRetryAfter(sec int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.retryAfter = sec
}

// SetRedirect is where FaultRedirect sends a request.
func (s *Server) SetRedirect(u string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.redirect = u
}

// Calls are the tools/call requests a tool's handler ran.
func (s *Server) Calls() []Call {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Call(nil), s.calls...)
}

// Requests is how many requests of a method reached the HTTP layer.
func (s *Server) Requests(method string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.methods[method]
}

// Registrations is how many clients DCR registered.
func (s *Server) Registrations() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.registered
}

// RefreshCalls is how many refresh_token grants the token endpoint served.
func (s *Server) RefreshCalls() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.refreshCalls
}

func (s *Server) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			s.mu.Lock()
			hang := s.hangDelete
			s.mu.Unlock()
			if hang != nil {
				select {
				case <-hang:
				case <-r.Context().Done():
				}
				return
			}
		}
		var body []byte
		if r.Body != nil {
			body, _ = io.ReadAll(r.Body)
			r.Body = io.NopCloser(bytes.NewReader(body))
		}
		var msg struct {
			Method string `json:"method"`
		}
		_ = json.Unmarshal(body, &msg)
		s.mu.Lock()
		if msg.Method != "" {
			s.methods[msg.Method]++
		}
		var fault Fault
		if q := s.faults[msg.Method]; len(q) > 0 {
			fault, s.faults[msg.Method] = q[0], q[1:]
		}
		needAuth := len(s.tokens) > 0
		token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		authorized := !needAuth || (r.Header.Get("Authorization") != "" && s.tokens[token])
		retry, redirect := s.retryAfter, s.redirect
		s.mu.Unlock()
		challenge := func() {
			w.Header().Set("WWW-Authenticate", `Bearer resource_metadata="`+s.srv.URL+`/.well-known/oauth-protected-resource`+Path+`"`)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
		}
		switch {
		case fault == FaultRedirect:
			http.Redirect(w, r, redirect, http.StatusTemporaryRedirect)
			return
		case fault == FaultUnauthorized || (!authorized && r.Method != http.MethodDelete):
			challenge()
			return
		case fault == FaultRateLimited:
			if retry > 0 {
				w.Header().Set("Retry-After", strconv.Itoa(retry))
			}
			http.Error(w, "slow down", http.StatusTooManyRequests)
			return
		case fault == FaultOversized:
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"content":[{"type":"text","text":"`))
			_, _ = w.Write(bytes.Repeat([]byte("x"), 2<<20))
			_, _ = w.Write([]byte(`"}]}}`))
			return
		case fault == FaultDrop:
			// The request is served (the tool runs), and the answer never arrives.
			next.ServeHTTP(discard{h: http.Header{}}, r)
			if hj, ok := w.(http.Hijacker); ok {
				if conn, _, err := hj.Hijack(); err == nil {
					_ = conn.Close()
				}
			}
			return
		}
		next.ServeHTTP(w, r)
	})
}

// discard is a response writer that keeps nothing.
type discard struct{ h http.Header }

func (d discard) Header() http.Header         { return d.h }
func (d discard) Write(p []byte) (int, error) { return len(p), nil }
func (d discard) WriteHeader(int)             {}
func (d discard) Flush()                      {}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		http.Error(w, "encode", http.StatusInternalServerError)
	}
}

func (s *Server) oauthOn(w http.ResponseWriter) bool {
	s.mu.Lock()
	on := s.oauth
	s.mu.Unlock()
	if !on {
		http.NotFound(w, nil)
	}
	return on
}

func (s *Server) resourceMetadata(w http.ResponseWriter, _ *http.Request) {
	if !s.oauthOn(w) {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"resource": s.URL, "authorization_servers": []string{s.srv.URL}, "scopes_supported": []string{"tools"}})
}

func (s *Server) serverMetadata(w http.ResponseWriter, _ *http.Request) {
	if !s.oauthOn(w) {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"issuer": s.srv.URL, "authorization_endpoint": s.srv.URL + "/authorize", "token_endpoint": s.srv.URL + "/token",
		"registration_endpoint": s.srv.URL + "/register", "code_challenge_methods_supported": []string{"S256"},
		"response_types_supported": []string{"code"},
	})
}

func (s *Server) register(w http.ResponseWriter, r *http.Request) {
	var meta struct {
		RedirectURIs []string `json:"redirect_uris"`
	}
	if err := json.NewDecoder(r.Body).Decode(&meta); err != nil || len(meta.RedirectURIs) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_client_metadata"})
		return
	}
	id := "client-" + uuid.NewString()
	s.mu.Lock()
	s.clients[id] = meta.RedirectURIs[0]
	s.registered++
	s.mu.Unlock()
	writeJSON(w, http.StatusCreated, map[string]any{"client_id": id, "redirect_uris": meta.RedirectURIs})
}

// AddClient pre-registers a client (one an administrator would enter).
func (s *Server) AddClient(id, redirect string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.clients[id] = redirect
}

// authorize approves at once: it redirects to the client's redirect_uri with a code.
func (s *Server) authorize(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	s.mu.Lock()
	redirect, ok := s.clients[q.Get("client_id")]
	s.mu.Unlock()
	if !ok || redirect != q.Get("redirect_uri") || q.Get("code_challenge_method") != "S256" || q.Get("code_challenge") == "" {
		http.Error(w, "bad authorization request", http.StatusBadRequest)
		return
	}
	code := uuid.NewString()
	s.mu.Lock()
	s.codes[code] = codeGrant{client: q.Get("client_id"), redirect: redirect, challenge: q.Get("code_challenge"), resource: q.Get("resource")}
	s.mu.Unlock()
	u, _ := url.Parse(redirect)
	v := u.Query()
	v.Set("code", code)
	v.Set("state", q.Get("state"))
	u.RawQuery = v.Encode()
	http.Redirect(w, r, u.String(), http.StatusFound)
}

func (s *Server) token(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_request"})
		return
	}
	f := r.PostForm
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.tokenMoved {
		http.Redirect(w, r, s.srv.URL+"/token-elsewhere", http.StatusTemporaryRedirect)
		return
	}
	s.tokenCalls++
	switch f.Get("grant_type") {
	case "authorization_code":
		g, ok := s.codes[f.Get("code")]
		delete(s.codes, f.Get("code"))
		sum := sha256.Sum256([]byte(f.Get("code_verifier")))
		if !ok || g.client != f.Get("client_id") || g.redirect != f.Get("redirect_uri") ||
			base64.RawURLEncoding.EncodeToString(sum[:]) != g.challenge || f.Get("resource") != s.URL {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_grant"})
			return
		}
	case "refresh_token":
		s.refreshCalls++
		if s.refuseGrant || !s.refresh[f.Get("refresh_token")] {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_grant"})
			return
		}
		delete(s.refresh, f.Get("refresh_token"))
	default:
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "unsupported_grant_type"})
		return
	}
	s.issued++
	access, refresh := fmt.Sprintf("access-%d-%s", s.issued, uuid.NewString()), fmt.Sprintf("refresh-%d-%s", s.issued, uuid.NewString())
	delete(s.tokens, "\x00")
	s.tokens[access] = true
	s.refresh[refresh] = true
	writeJSON(w, http.StatusOK, map[string]any{"access_token": access, "token_type": "Bearer", "refresh_token": refresh, "expires_in": s.expiresIn})
}

// MoveTokenEndpoint makes the token endpoint answer 307 to another URL (which counts what reaches it).
func (s *Server) MoveTokenEndpoint() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tokenMoved = true
}

// Elsewhere is how many requests reached the URL the token endpoint redirects to.
func (s *Server) Elsewhere() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.elsewhere
}

// HangDelete makes the DELETE that ends a session wait until the test ends.
func (s *Server) HangDelete() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.hangDelete == nil {
		s.hangDelete = make(chan struct{})
		s.t.Cleanup(func() { close(s.hangDelete) })
	}
}

// Revoke makes an issued access token invalid (the server forgot it).
func (s *Server) Revoke(token string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.tokens, token)
	if len(s.tokens) == 0 {
		s.tokens["\x00"] = true
	}
}

// Host is the listener's host and port, for a resolver that maps a name to it.
func (s *Server) Host() (string, string) {
	h, p, _ := net.SplitHostPort(s.Addr)
	return h, p
}

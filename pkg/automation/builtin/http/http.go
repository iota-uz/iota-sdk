// Package http is the built-in node type http: one HTTPS request through the network policy of
// pkg/automation/code (https only, every hop's DNS checked, credentials dropped once a redirect leaves the
// origin), with the authorization of a declared secret or of a connection, run in the call's child
// instance. Only GET and HEAD, and requests that send the call's idempotency key in a header, are
// repeated after an attempt whose outcome is unknown.
package http

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	nethttp "net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/iota-uz/iota-sdk/pkg/automation/builtin/access"
	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	sandbox "github.com/iota-uz/iota-sdk/pkg/automation/code"
	"github.com/iota-uz/iota-sdk/pkg/automation/engine"
	"github.com/iota-uz/iota-sdk/pkg/automation/expr"
	"github.com/iota-uz/iota-sdk/pkg/automation/graph"
)

// Type is the node type.
const Type = "http"

// Outcomes of the node.
const (
	OutcomeSuccess = "success"
	OutcomeFailed  = "failed"
)

// MaxResponseBytes is the largest response body the node reads.
const MaxResponseBytes = 10 << 20

// DefaultSuccess is the success expression when the node gives none.
const DefaultSuccess = "status < 400"

const headerName = `^[A-Za-z0-9!#$%&'*+^_|~-]{1,100}$`

const paramsSchema = `{"type": "object", "properties": {
	"method": {"enum": ["GET", "POST", "PUT", "PATCH", "DELETE", "HEAD"], "default": "GET"},
	"url": {"type": "string", "minLength": 1, "maxLength": 8000, "description": "https URL; {{ }} templates"},
	"headers": {"type": "object", "maxProperties": 50, "propertyNames": {"pattern": "` + headerName + `"},
		"additionalProperties": {"type": "string", "maxLength": 8000}, "description": "name → value; {{ }} templates"},
	"query": {"type": "object", "maxProperties": 100, "propertyNames": {"pattern": "^[^.]{1,200}$"},
		"additionalProperties": {"type": "string", "maxLength": 8000}, "description": "name → value; {{ }} templates"},
	"body": {"type": "object", "properties": {
		"kind": {"enum": ["none", "json", "form", "text"], "default": "none"},
		"value": {"type": "string", "minLength": 1, "maxLength": 100000, "description": "CEL: any JSON (json), a map of strings (form), a string (text)"}
	}, "required": ["kind"], "additionalProperties": false},
	"auth": {"type": "object", "properties": {
		"secret": {"type": "string", "minLength": 1, "maxLength": 200, "description": "a secret name: API_TOKEN, or <connection>.<field>"},
		"scheme": {"enum": ["bearer", "basic", "header"]},
		"header": {"type": "string", "pattern": "` + headerName + `"},
		"connection": {"type": "string", "pattern": "^[a-z][a-z0-9_]{0,39}$", "description": "a connection whose connector authorizes requests to its API"}
	}, "additionalProperties": false},
	"timeout": {"type": "string", "pattern": "^[1-9][0-9]{0,5}(s|m)$", "x-max-ms": 120000, "default": "30s", "description": "at most 120s"},
	"success": {"type": "string", "minLength": 1, "maxLength": 2000, "default": "` + DefaultSuccess + `", "description": "CEL over status, headers and body"},
	"idempotency_header": {"type": "string", "pattern": "` + headerName + `", "description": "the call's idempotency key goes in this header; the request may then be repeated"}
}, "required": ["url"], "additionalProperties": false}`

// Deps are what the node sends requests with.
type Deps struct {
	Fetcher     sandbox.Fetcher // default sandbox.NewGuardedFetcher(sandbox.WithMaxBody(MaxResponseBytes))
	Secrets     access.Secrets
	Connections access.Connections
}

// Node is the http node type.
type Node struct{ d Deps }

var (
	_ catalog.CallNode       = (*Node)(nil)
	_ catalog.IdempotentCall = (*Node)(nil)
	_ catalog.LocalizedNode  = (*Node)(nil)
	_ engine.NodeValidator   = (*Node)(nil)
)

// New is the http node type.
func New(d Deps) *Node {
	if d.Fetcher == nil {
		d.Fetcher = sandbox.NewGuardedFetcher(sandbox.WithMaxBody(MaxResponseBytes))
	}
	return &Node{d: d}
}

func (n *Node) Spec() graph.NodeSpec {
	return graph.NodeSpec{
		Type: Type, Title: "HTTP request", Group: "network", Behavior: graph.BehaviorCall,
		Description: "Sends an HTTPS request; success when the success expression over the response holds.",
		Params:      graph.MustSchemaParams(paramsSchema),
		Outcomes:    []string{OutcomeSuccess, OutcomeFailed}, FailedOutcome: OutcomeFailed,
		Templates: []string{"url", "headers.*", "query.*"},
		Values:    []string{"body.value"},
		Output: expr.Fields{
			{Name: "status", Field: expr.IntField},
			{Name: "headers", Field: expr.MapOf(expr.StringField)},
			{Name: "body", Field: expr.DynField},
			{Name: "url", Field: expr.StringField},
		},
		DryRun: graph.DryRunCapture,
	}
}

func (n *Node) Requires() []catalog.ConnectorKind { return nil }

// Idempotent is the default for a request whose params are unknown: not repeated.
func (n *Node) Idempotent() bool { return false }

// IdempotentWith: GET and HEAD, or a request that sends the call's idempotency key.
func (n *Node) IdempotentWith(params map[string]any) bool {
	method, _ := params["method"].(string)
	if method == "" {
		method = nethttp.MethodGet
	}
	if h, _ := params["idempotency_header"].(string); h != "" {
		return true
	}
	return method == nethttp.MethodGet || method == nethttp.MethodHead
}

func (n *Node) Localize(spec graph.NodeSpec, locale string) graph.NodeSpec {
	switch locale {
	case "ru":
		spec.Title, spec.Description = "HTTP-запрос", "Отправляет HTTPS-запрос; success, когда выражение успеха над ответом истинно."
	case "uz":
		spec.Title, spec.Description = "HTTP so‘rov", "HTTPS so‘rov yuboradi; javob bo‘yicha muvaffaqiyat ifodasi bajarilsa success."
	case "uz-Cyrl":
		spec.Title, spec.Description = "HTTP сўров", "HTTPS сўров юборади; жавоб бўйича муваффақият ифодаси бажарилса success."
	}
	return spec
}

// successEnv is the scope of the success expression: the response.
var successEnv = func() *expr.Env {
	e, err := expr.NewEnv(expr.Scope{Vars: map[string]expr.Field{
		"status":  expr.IntField,
		"headers": expr.MapOf(expr.StringField),
		"body":    expr.DynField,
		"url":     expr.StringField,
	}}, expr.Limits{})
	if err != nil {
		panic(err)
	}
	return e
}()

type auth struct {
	Secret     string `json:"secret"`
	Scheme     string `json:"scheme"`
	Header     string `json:"header"`
	Connection string `json:"connection"`
}

func authOf(params map[string]any) (*auth, error) {
	raw, ok := params["auth"].(map[string]any)
	if !ok || len(raw) == 0 {
		return nil, nil //nolint:nilnil // no auth
	}
	b, err := json.Marshal(raw)
	if err != nil {
		return nil, err
	}
	var a auth
	if err := json.Unmarshal(b, &a); err != nil {
		return nil, err
	}
	switch {
	case a.Connection != "" && (a.Secret != "" || a.Scheme != "" || a.Header != ""):
		return nil, errors.New("auth is a connection, or a secret with its scheme: not both")
	case a.Connection != "":
	case a.Secret == "" || a.Scheme == "":
		return nil, errors.New("auth with a secret names its scheme: bearer, basic or header")
	case a.Scheme == "header" && a.Header == "":
		return nil, errors.New("auth scheme header names the header")
	case a.Scheme != "header" && a.Header != "":
		return nil, errors.New("only auth scheme header names a header")
	}
	return &a, nil
}

// ValidateParams checks what the schema cannot: the success expression and the shape of auth.
func (n *Node) ValidateParams(_ context.Context, params map[string]any) []graph.GraphError {
	var out []graph.GraphError
	if src, ok := params["success"].(string); ok && src != "" {
		if _, issues := successEnv.Check(src, expr.TypeBool); len(issues) > 0 {
			out = append(out, graph.GraphError{Path: "params.success", Code: graph.CodeExpressionType, Message: issues[0].Message, Line: issues[0].Line, Column: issues[0].Column})
		}
	}
	if _, err := authOf(params); err != nil {
		out = append(out, graph.GraphError{Path: "params.auth", Code: graph.CodeParams, Message: err.Error()})
	}
	if b, ok := params["body"].(map[string]any); ok {
		kind, _ := b["kind"].(string)
		_, hasValue := b["value"]
		switch {
		case kind != "" && kind != "none" && !hasValue:
			out = append(out, graph.GraphError{Path: "params.body.value", Code: graph.CodeParams, Message: "a body of kind " + kind + " has a value"})
		case (kind == "" || kind == "none") && hasValue:
			out = append(out, graph.GraphError{Path: "params.body.kind", Code: graph.CodeParams, Message: "a body with a value has a kind: json, form or text"})
		}
	}
	method, _ := params["method"].(string)
	if (method == "" || method == nethttp.MethodGet || method == nethttp.MethodHead) && params["body"] != nil {
		if b, _ := params["body"].(map[string]any); b["kind"] != nil && b["kind"] != "none" {
			out = append(out, graph.GraphError{Path: "params.body", Code: graph.CodeParams, Message: "a GET or HEAD request has no body"})
		}
	}
	return out
}

// fail is a CallError with a code; the message is masked by the caller.
func fail(code, format string, args ...any) error {
	return &catalog.CallError{Code: code, Message: fmt.Sprintf(format, args...)}
}

// Execute sends the request once.
func (n *Node) Execute(ctx context.Context, in catalog.CallInput) (catalog.CallResult, error) {
	p := in.Params
	method, _ := p["method"].(string)
	if method == "" {
		method = nethttp.MethodGet
	}
	rawURL, _ := p["url"].(string)
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || u.Host == "" {
		return catalog.CallResult{}, fail(access.CodeInvalidParams, "the URL is not an absolute https URL")
	}
	if u.Scheme != "https" {
		return catalog.CallResult{}, fail(access.CodeNetworkBlocked, "network blocked: only https URLs are allowed: %s", u.Hostname())
	}
	if q, ok := p["query"].(map[string]any); ok && len(q) > 0 {
		values := u.Query()
		for _, k := range sortedKeys(q) {
			values.Set(k, fmt.Sprint(q[k]))
		}
		u.RawQuery = values.Encode()
	}
	header := nethttp.Header{}
	if h, ok := p["headers"].(map[string]any); ok {
		for _, k := range sortedKeys(h) {
			header.Set(k, fmt.Sprint(h[k]))
		}
	}
	body, err := encodeBody(p, header)
	if err != nil {
		return catalog.CallResult{}, err
	}
	var masked []string
	a, err := authOf(p)
	if err != nil {
		return catalog.CallResult{}, fail(access.CodeInvalidParams, "%v", err)
	}
	if a != nil {
		values, err := n.authorize(ctx, in, a, u, header)
		if err != nil {
			return catalog.CallResult{}, err
		}
		masked = values
	}
	masker := access.Masker(masked...)
	if h, _ := p["idempotency_header"].(string); h != "" {
		header.Set(h, in.IdempotencyKey)
	}
	timeout := 30 * time.Second
	if ms, ok := graph.DurationMs(p["timeout"]); ok {
		timeout = time.Duration(ms) * time.Millisecond
	}
	rctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	resp, err := n.d.Fetcher.Fetch(rctx, sandbox.FetchRequest{URL: u.String(), Method: method, Header: header, Body: body})
	if err != nil {
		var blocked *sandbox.NetworkBlockedError
		switch {
		case errors.As(err, &blocked):
			return catalog.CallResult{}, fail(access.CodeNetworkBlocked, "%s", access.BlockedMessage(blocked))
		case errors.Is(err, sandbox.ErrResponseTooLarge):
			return catalog.CallResult{}, fail(access.CodeResponseTooLarge, "the response is larger than %d MiB", MaxResponseBytes>>20)
		case ctx.Err() != nil:
			return catalog.CallResult{}, ctx.Err()
		case rctx.Err() != nil:
			if n.IdempotentWith(p) {
				return catalog.CallResult{}, &catalog.CallError{Code: access.CodeTimeout, Message: "no response within " + timeout.String(), Retryable: true}
			}
			return catalog.CallResult{}, &catalog.CallError{Code: catalog.CallOutcomeUnknown, Message: "no response within " + timeout.String()}
		}
		// The engine repeats an idempotent request and records the outcome of another as unknown.
		return catalog.CallResult{}, errors.New(masker.Mask(err.Error()))
	}
	if resp.Status == nethttp.StatusTooManyRequests {
		if d, ok := retryAfter(resp.Header.Get("Retry-After")); ok {
			return catalog.CallResult{RetryAfter: d}, nil
		}
	}
	out := map[string]any{"status": resp.Status, "headers": headersOf(resp.Header), "body": bodyOf(resp), "url": resp.URL}
	raw, err := json.Marshal(out)
	if err != nil {
		return catalog.CallResult{}, fail(graph.FailInvalidAnswer, "the response is not JSON")
	}
	if err := json.Unmarshal(masker.MaskJSON(raw), &out); err != nil {
		return catalog.CallResult{}, fail(graph.FailInvalidAnswer, "the response is not JSON")
	}
	ok, err := succeeded(p, out)
	if err != nil {
		return catalog.CallResult{}, fail(graph.FailExpression, "%s", masker.Mask(err.Error()))
	}
	outcome := OutcomeSuccess
	if !ok {
		outcome = OutcomeFailed
		out["error"] = graph.FailedOutput("http_status", "the response "+strconv.Itoa(resp.Status)+" did not pass the success expression", false)["error"]
	}
	return catalog.CallResult{Outcome: outcome, Output: out}, nil
}

// authorize adds the authorization of the secret or the connection; values are what to mask.
func (n *Node) authorize(ctx context.Context, in catalog.CallInput, a *auth, u *url.URL, header nethttp.Header) ([]string, error) {
	if a.Connection != "" {
		conns, err := access.OpenAll(ctx, n.d.Connections, in.TenantID, []string{a.Connection})
		if err != nil {
			return nil, err
		}
		c := conns[a.Connection]
		if _, err := access.Target(c, u.String()); err != nil {
			return nil, fail(access.CodeConnectionOriginMismatch, "connection %s does not authorize requests to %s", a.Connection, u.Hostname())
		}
		if err := c.Authorize(ctx, header); err != nil {
			return nil, fail(access.CodeConnectionMissing, "connection %s could not authorize the request", a.Connection)
		}
		return c.Values, nil
	}
	secrets, err := access.ReadSecrets(ctx, n.d.Secrets, in, []string{a.Secret})
	if err != nil {
		return nil, err
	}
	v := secrets[a.Secret]
	switch a.Scheme {
	case "bearer":
		header.Set("Authorization", "Bearer "+v)
		return []string{v}, nil
	case "basic":
		enc := base64.StdEncoding.EncodeToString([]byte(v))
		header.Set("Authorization", "Basic "+enc)
		return []string{v, enc}, nil
	default:
		header.Set(a.Header, v)
		return []string{v}, nil
	}
}

// encodeBody is the request body of the node's body param and its content type.
func encodeBody(p map[string]any, header nethttp.Header) ([]byte, error) {
	b, ok := p["body"].(map[string]any)
	if !ok {
		return nil, nil
	}
	kind, _ := b["kind"].(string)
	value := b["value"]
	setType := func(t string) {
		if header.Get("Content-Type") == "" {
			header.Set("Content-Type", t)
		}
	}
	switch kind {
	case "", "none":
		return nil, nil
	case "json":
		out, err := json.Marshal(value)
		if err != nil {
			return nil, fail(access.CodeInvalidParams, "the body is not JSON")
		}
		setType("application/json")
		return out, nil
	case "form":
		m, ok := value.(map[string]any)
		if !ok {
			return nil, fail(access.CodeInvalidParams, "a form body is a map of strings")
		}
		form := url.Values{}
		for _, k := range sortedKeys(m) {
			switch v := m[k].(type) {
			case string:
				form.Set(k, v)
			case nil:
			default:
				j, _ := json.Marshal(v)
				form.Set(k, strings.Trim(string(j), `"`))
			}
		}
		setType("application/x-www-form-urlencoded")
		return []byte(form.Encode()), nil
	case "text":
		s, ok := value.(string)
		if !ok {
			return nil, fail(access.CodeInvalidParams, "a text body is a string")
		}
		setType("text/plain; charset=utf-8")
		return []byte(s), nil
	}
	return nil, fail(access.CodeInvalidParams, "body kind %q is none, json, form or text", kind)
}

// succeeded evaluates the success expression over the response.
func succeeded(p map[string]any, out map[string]any) (bool, error) {
	src, _ := p["success"].(string)
	if src == "" {
		src = DefaultSuccess
	}
	prg, issues := successEnv.Check(src, expr.TypeBool)
	if len(issues) > 0 {
		return false, errors.New("the success expression: " + issues[0].Message)
	}
	v, err := prg.Eval(out)
	if err != nil {
		var missing *expr.MissingContextError
		if errors.As(err, &missing) {
			return false, nil // the response lacks what the expression reads: not a success
		}
		return false, err
	}
	b, _ := v.(bool)
	return b, nil
}

// bodyOf is the response body: parsed JSON for a JSON content type, else the text.
func bodyOf(resp *sandbox.FetchResponse) any {
	ct := strings.ToLower(resp.Header.Get("Content-Type"))
	if strings.Contains(ct, "json") && len(resp.Body) > 0 {
		var v any
		if json.Unmarshal(resp.Body, &v) == nil {
			return v
		}
	}
	if utf8.Valid(resp.Body) {
		return string(resp.Body)
	}
	return strings.ToValidUTF8(string(resp.Body), "�")
}

func headersOf(h nethttp.Header) map[string]any {
	out := make(map[string]any, len(h))
	for k, vs := range h {
		out[strings.ToLower(k)] = strings.Join(vs, ", ")
	}
	return out
}

// retryAfter reads a Retry-After header: seconds or an HTTP date.
func retryAfter(v string) (time.Duration, bool) {
	v = strings.TrimSpace(v)
	if v == "" {
		return 0, false
	}
	if s, err := strconv.Atoi(v); err == nil && s >= 0 {
		return max(time.Duration(s)*time.Second, time.Second), true
	}
	if t, err := nethttp.ParseTime(v); err == nil {
		return max(time.Until(t), time.Second), true
	}
	return 0, false
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

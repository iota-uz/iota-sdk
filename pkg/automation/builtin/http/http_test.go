package http_test

import (
	"context"
	"errors"
	"net"
	nethttp "net/http"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/iota-uz/iota-sdk/pkg/automation/builtin/access"
	httpnode "github.com/iota-uz/iota-sdk/pkg/automation/builtin/http"
	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	sandbox "github.com/iota-uz/iota-sdk/pkg/automation/code"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
)

// fakeFetcher answers every request with resp and records it.
type fakeFetcher struct {
	got  []sandbox.FetchRequest
	resp *sandbox.FetchResponse
	err  error
}

func (f *fakeFetcher) Fetch(_ context.Context, req sandbox.FetchRequest) (*sandbox.FetchResponse, error) {
	f.got = append(f.got, req)
	if f.err != nil {
		return nil, f.err
	}
	return f.resp, nil
}

func jsonResp(status int, body string) *sandbox.FetchResponse {
	return &sandbox.FetchResponse{Status: status, Header: nethttp.Header{"Content-Type": {"application/json"}}, Body: []byte(body), URL: "https://api.example.com/x"}
}

type secrets map[string]string

func (s secrets) Secret(_ context.Context, _ uuid.UUID, _ spi.Subject, name string) (string, error) {
	if v, ok := s[name]; ok {
		return v, nil
	}
	return "", access.ErrSecretMissing
}

type conns map[string]access.Connection

func (c conns) Open(_ context.Context, _ uuid.UUID, name string) (access.Connection, error) {
	if v, ok := c[name]; ok {
		return v, nil
	}
	return access.Connection{}, access.ErrConnectionMissing
}

func call(params map[string]any) catalog.CallInput {
	return catalog.CallInput{TenantID: uuid.New(), IdempotencyKey: "run:node:tok:1", Params: params, Deadline: time.Now().Add(time.Minute)}
}

func codeOf(t *testing.T, err error) string {
	t.Helper()
	var ce *catalog.CallError
	if !errors.As(err, &ce) {
		t.Fatalf("want a CallError, got %v", err)
	}
	return ce.Code
}

// privateResolver resolves internal.example.com to a private address and every other name publicly.
type privateResolver struct{}

func (privateResolver) LookupNetIP(_ context.Context, _, host string) ([]netip.Addr, error) {
	if host == "internal.example.com" {
		return []netip.Addr{netip.MustParseAddr("10.0.0.5")}, nil
	}
	return []netip.Addr{netip.MustParseAddr("93.184.216.34")}, nil
}

func TestSSRFIsNetworkBlockedWithoutTheAddress(t *testing.T) {
	dialed := false
	n := httpnode.New(httpnode.Deps{Fetcher: sandbox.NewGuardedFetcher(sandbox.WithResolver(privateResolver{}),
		sandbox.WithDialer(func(context.Context, string, string) (net.Conn, error) {
			dialed = true
			return nil, errors.New("no network in tests")
		}))})
	for _, u := range []string{"https://169.254.169.254/latest/meta-data/", "https://internal.example.com/admin", "http://example.com/"} {
		_, err := n.Execute(context.Background(), call(map[string]any{"url": u}))
		if got := codeOf(t, err); got != access.CodeNetworkBlocked {
			t.Fatalf("%s: code %s (%v)", u, got, err)
		}
		if strings.Contains(err.Error(), "10.0.0.5") {
			t.Fatalf("%s: the message reveals the internal address: %v", u, err)
		}
	}
	if dialed {
		t.Fatal("a blocked request was dialed")
	}
}

func TestBearerSecretIsSentAndMasked(t *testing.T) {
	ff := &fakeFetcher{resp: jsonResp(200, `{"echo": "Bearer sk-9z8y7x", "ok": true}`)}
	n := httpnode.New(httpnode.Deps{Fetcher: ff, Secrets: secrets{"API_TOKEN": "sk-9z8y7x"}})
	res, err := n.Execute(context.Background(), call(map[string]any{
		"method": "POST", "url": "https://api.example.com/v1/items", "query": map[string]any{"q": "a b"},
		"headers": map[string]any{"X-Trace": "t1"}, "body": map[string]any{"kind": "json", "value": map[string]any{"name": "x"}},
		"auth": map[string]any{"secret": "API_TOKEN", "scheme": "bearer"}, "idempotency_header": "Idempotency-Key",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if res.Outcome != httpnode.OutcomeSuccess {
		t.Fatalf("outcome %s", res.Outcome)
	}
	req := ff.got[0]
	if req.Method != nethttp.MethodPost || req.URL != "https://api.example.com/v1/items?q=a+b" || string(req.Body) != `{"name":"x"}` {
		t.Fatalf("request %+v", req)
	}
	if req.Header.Get("Authorization") != "Bearer sk-9z8y7x" || req.Header.Get("Idempotency-Key") != "run:node:tok:1" || req.Header.Get("Content-Type") != "application/json" {
		t.Fatalf("headers %v", req.Header)
	}
	body, _ := res.Output["body"].(map[string]any)
	if body["echo"] != "***" {
		t.Fatalf("the secret is not masked in the output: %v", res.Output)
	}
}

func TestMissingSecretIsSecretMissing(t *testing.T) {
	n := httpnode.New(httpnode.Deps{Fetcher: &fakeFetcher{resp: jsonResp(200, `{}`)}, Secrets: secrets{}})
	_, err := n.Execute(context.Background(), call(map[string]any{"url": "https://api.example.com/", "auth": map[string]any{"secret": "NOPE", "scheme": "bearer"}}))
	if got := codeOf(t, err); got != access.CodeSecretMissing {
		t.Fatalf("code %s", got)
	}
}

func TestConnectionAuthorizesOnlyItsOrigin(t *testing.T) {
	ca := connectors.CodeAccess{BaseURL: "https://api.example.com/v1", Methods: []connectors.CodeMethod{{Name: "get", HTTPMethod: "GET"}}}
	c := access.NewConnection("crm", "acme", ca, []string{"tok-abc"}, func(_ context.Context, h nethttp.Header) error {
		h.Set("Authorization", "Bearer tok-abc")
		return nil
	})
	ff := &fakeFetcher{resp: jsonResp(200, `{"token": "tok-abc"}`)}
	n := httpnode.New(httpnode.Deps{Fetcher: ff, Connections: conns{"crm": c}})
	res, err := n.Execute(context.Background(), call(map[string]any{"url": "https://api.example.com/v1/deals", "auth": map[string]any{"connection": "crm"}}))
	if err != nil {
		t.Fatal(err)
	}
	if ff.got[0].Header.Get("Authorization") != "Bearer tok-abc" {
		t.Fatalf("headers %v", ff.got[0].Header)
	}
	if strings.Contains(stringOf(res.Output), "tok-abc") {
		t.Fatalf("the token leaked: %v", res.Output)
	}
	_, err = n.Execute(context.Background(), call(map[string]any{"url": "https://evil.example.org/", "auth": map[string]any{"connection": "crm"}}))
	if got := codeOf(t, err); got != access.CodeConnectionOriginMismatch {
		t.Fatalf("code %s", got)
	}
	_, err = n.Execute(context.Background(), call(map[string]any{"url": "https://api.example.com/", "auth": map[string]any{"connection": "other"}}))
	if got := codeOf(t, err); got != access.CodeConnectionMissing {
		t.Fatalf("code %s", got)
	}
}

func stringOf(m map[string]any) string {
	var b strings.Builder
	for k, v := range m {
		b.WriteString(k)
		b.WriteString("=")
		if mm, ok := v.(map[string]any); ok {
			b.WriteString(stringOf(mm))
		} else if s, ok := v.(string); ok {
			b.WriteString(s)
		}
		b.WriteString(";")
	}
	return b.String()
}

func TestSuccessExpressionAndRetryAfter(t *testing.T) {
	n := httpnode.New(httpnode.Deps{Fetcher: &fakeFetcher{resp: jsonResp(404, `{"error": "nope"}`)}})
	res, err := n.Execute(context.Background(), call(map[string]any{"url": "https://api.example.com/"}))
	if err != nil || res.Outcome != httpnode.OutcomeFailed || res.Output["status"] != float64(404) {
		t.Fatalf("404: %v %+v", err, res)
	}
	res, err = n.Execute(context.Background(), call(map[string]any{"url": "https://api.example.com/", "success": "status == 404 && body.error == 'nope'"}))
	if err != nil || res.Outcome != httpnode.OutcomeSuccess {
		t.Fatalf("custom success: %v %+v", err, res)
	}
	limited := &sandbox.FetchResponse{Status: 429, Header: nethttp.Header{"Retry-After": {"7"}}, URL: "https://api.example.com/"}
	n = httpnode.New(httpnode.Deps{Fetcher: &fakeFetcher{resp: limited}})
	res, err = n.Execute(context.Background(), call(map[string]any{"url": "https://api.example.com/"}))
	if err != nil || res.RetryAfter != 7*time.Second {
		t.Fatalf("429: %v %+v", err, res)
	}
}

func TestResponseTooLarge(t *testing.T) {
	n := httpnode.New(httpnode.Deps{Fetcher: &fakeFetcher{err: sandbox.ErrResponseTooLarge}})
	_, err := n.Execute(context.Background(), call(map[string]any{"url": "https://api.example.com/"}))
	if got := codeOf(t, err); got != access.CodeResponseTooLarge {
		t.Fatalf("code %s", got)
	}
}

func TestOnlyGetHeadAndIdempotencyHeaderAreIdempotent(t *testing.T) {
	n := httpnode.New(httpnode.Deps{})
	cases := []struct {
		params map[string]any
		want   bool
	}{
		{map[string]any{"url": "https://x"}, true},
		{map[string]any{"method": "HEAD"}, true},
		{map[string]any{"method": "POST"}, false},
		{map[string]any{"method": "DELETE"}, false},
		{map[string]any{"method": "POST", "idempotency_header": "Idempotency-Key"}, true},
	}
	for _, c := range cases {
		if got := n.IdempotentWith(c.params); got != c.want {
			t.Errorf("%v: %v", c.params, got)
		}
	}
	if n.Idempotent() {
		t.Error("the node is idempotent without params")
	}
}

func TestValidateParams(t *testing.T) {
	n := httpnode.New(httpnode.Deps{})
	if errs := n.ValidateParams(context.Background(), map[string]any{"url": "https://x", "success": "status <"}); len(errs) != 1 || errs[0].Path != "params.success" {
		t.Fatalf("success: %+v", errs)
	}
	if errs := n.ValidateParams(context.Background(), map[string]any{"url": "https://x", "auth": map[string]any{"secret": "A", "scheme": "header"}}); len(errs) != 1 || errs[0].Path != "params.auth" {
		t.Fatalf("auth: %+v", errs)
	}
	if errs := n.ValidateParams(context.Background(), map[string]any{"url": "https://x", "auth": map[string]any{"connection": "crm", "secret": "A"}}); len(errs) != 1 {
		t.Fatalf("auth both: %+v", errs)
	}
	if errs := n.ValidateParams(context.Background(), map[string]any{"url": "https://x", "method": "GET", "body": map[string]any{"kind": "json", "value": "1"}}); len(errs) != 1 {
		t.Fatalf("GET body: %+v", errs)
	}
}

func TestAuthNeedsAStaticHost(t *testing.T) {
	n := httpnode.New(httpnode.Deps{})
	auth := map[string]any{"secret": "API_TOKEN", "scheme": "bearer"}
	for url, ok := range map[string]bool{
		"https://api.example.com/v1/{{ event.path }}": true,
		"https://api.example.com?q={{ event.q }}":     true,
		"https://api.example.com":                     true,
		"https://api.example.com{{ event.path }}":     false,
		"https://{{ event.host }}/x":                  false,
		"{{ event.url }}":                             false,
	} {
		errs := n.ValidateParams(context.Background(), map[string]any{"url": url, "auth": auth})
		if (len(errs) == 0) != ok {
			t.Errorf("%s: %+v", url, errs)
		}
	}
	// Without auth a templated host is the author's choice.
	if errs := n.ValidateParams(context.Background(), map[string]any{"url": "{{ event.url }}"}); len(errs) != 0 {
		t.Fatalf("%+v", errs)
	}
}

func TestABodyOverTheOutputLimitKeepsTheOutcome(t *testing.T) {
	big := `{"ok": true, "x": "` + strings.Repeat("a", 2<<20) + `"}`
	n := httpnode.New(httpnode.Deps{Fetcher: &fakeFetcher{resp: jsonResp(200, big)}, MaxOutputBytes: 1 << 20})
	res, err := n.Execute(context.Background(), call(map[string]any{"method": "POST", "url": "https://api.example.com/", "success": "status == 200 && body.ok"}))
	if err != nil {
		t.Fatal(err)
	}
	if res.Outcome != httpnode.OutcomeSuccess || res.Output["body"] != nil || res.Output["body_omitted"] != true || res.Output["status"] != float64(200) {
		t.Fatalf("%v %v %v", res.Outcome, res.Output["body_omitted"], res.Output["status"])
	}
}

// A text body of 10 MiB of control characters grows sixfold as JSON and still comes back whole.
func TestATextBodyOfTheWholeRangeComesBack(t *testing.T) {
	body := strings.Repeat("\x01", httpnode.MaxResponseBytes)
	n := httpnode.New(httpnode.Deps{Fetcher: &fakeFetcher{resp: &sandbox.FetchResponse{Status: 200, Header: nethttp.Header{"Content-Type": {"text/plain"}}, Body: []byte(body), URL: "https://api.example.com/"}}})
	res, err := n.Execute(context.Background(), call(map[string]any{"url": "https://api.example.com/"}))
	if err != nil {
		t.Fatal(err)
	}
	if res.Outcome != httpnode.OutcomeSuccess || res.Output["body"] != body || res.Output["body_omitted"] != nil {
		t.Fatalf("the body was dropped: %v %v", res.Outcome, res.Output["body_omitted"])
	}
}

func TestBodyTemplate(t *testing.T) {
	ff := &fakeFetcher{resp: jsonResp(200, `{}`)}
	n := httpnode.New(httpnode.Deps{Fetcher: ff})
	if _, err := n.Execute(context.Background(), call(map[string]any{"method": "POST", "url": "https://api.example.com/",
		"body": map[string]any{"kind": "text", "template": "Hello, Aziz"}})); err != nil {
		t.Fatal(err)
	}
	if string(ff.got[0].Body) != "Hello, Aziz" || ff.got[0].Header.Get("Content-Type") != "text/plain; charset=utf-8" {
		t.Fatalf("%+v", ff.got[0])
	}
	if _, err := n.Execute(context.Background(), call(map[string]any{"method": "POST", "url": "https://api.example.com/",
		"body": map[string]any{"kind": "json", "template": `{"name": "Aziz"}`}})); err != nil {
		t.Fatal(err)
	}
	if string(ff.got[1].Body) != `{"name":"Aziz"}` {
		t.Fatalf("%s", ff.got[1].Body)
	}
	// A rendered template in value: the JSON text of an object is the body, a form's text is the form.
	if _, err := n.Execute(context.Background(), call(map[string]any{"method": "POST", "url": "https://api.example.com/",
		"body": map[string]any{"kind": "json", "value": `{"name": "Aziz"}`}})); err != nil {
		t.Fatal(err)
	}
	if string(ff.got[2].Body) != `{"name": "Aziz"}` || ff.got[2].Header.Get("Content-Type") != "application/json" {
		t.Fatalf("%s", ff.got[2].Body)
	}
	if _, err := n.Execute(context.Background(), call(map[string]any{"method": "POST", "url": "https://api.example.com/",
		"body": map[string]any{"kind": "form", "value": "a=1&b=Aziz"}})); err != nil {
		t.Fatal(err)
	}
	if string(ff.got[3].Body) != "a=1&b=Aziz" {
		t.Fatalf("%s", ff.got[3].Body)
	}
	if errs := n.ValidateParams(context.Background(), map[string]any{"url": "https://x", "method": "POST", "body": map[string]any{"kind": "text", "template": "a", "value": "'a'"}}); len(errs) != 1 {
		t.Fatalf("%+v", errs)
	}
	if errs := n.ValidateParams(context.Background(), map[string]any{"url": "https://x", "method": "POST", "body": map[string]any{"kind": "json", "value": `{"a": "{{ event.x }}"}`}}); len(errs) != 0 {
		t.Fatalf("a JSON template in value: %+v", errs)
	}
}

func TestRepeatableChangeStaysOnItsOrigin(t *testing.T) {
	ff := &fakeFetcher{err: &sandbox.OriginError{From: "https://api.example.com:443", To: "https://other.example.com:443"}}
	n := httpnode.New(httpnode.Deps{Fetcher: ff})
	_, err := n.Execute(context.Background(), call(map[string]any{"method": "POST", "url": "https://api.example.com/pay", "idempotency_header": "Idempotency-Key"}))
	if got := codeOf(t, err); got != httpnode.CodeRedirectRefused {
		t.Fatalf("code %s", got)
	}
	if !ff.got[0].SameOrigin || len(ff.got[0].Sensitive) != 1 {
		t.Fatalf("request %+v", ff.got[0])
	}
	// A GET with the key may follow: it changes nothing.
	ff = &fakeFetcher{resp: jsonResp(200, `{}`)}
	n = httpnode.New(httpnode.Deps{Fetcher: ff})
	if _, err := n.Execute(context.Background(), call(map[string]any{"url": "https://api.example.com/", "idempotency_header": "Idempotency-Key"})); err != nil {
		t.Fatal(err)
	}
	if ff.got[0].SameOrigin {
		t.Fatal("a GET was kept on its origin")
	}
}

func TestIdempotencyHeaderTheTransportSetsIsRefused(t *testing.T) {
	n := httpnode.New(httpnode.Deps{Fetcher: &fakeFetcher{resp: jsonResp(200, `{}`)}})
	for _, h := range []string{"Host", "content-length", "Transfer-Encoding", "Connection", "Authorization"} {
		if n.IdempotentWith(map[string]any{"method": "POST", "idempotency_header": h}) {
			t.Errorf("%s makes a POST repeatable", h)
		}
		if errs := n.ValidateParams(context.Background(), map[string]any{"url": "https://x", "method": "POST", "idempotency_header": h}); len(errs) != 1 {
			t.Errorf("%s: %+v", h, errs)
		}
		_, err := n.Execute(context.Background(), call(map[string]any{"method": "POST", "url": "https://api.example.com/", "idempotency_header": h}))
		if codeOf(t, err) != access.CodeInvalidParams {
			t.Errorf("%s was sent", h)
		}
	}
}

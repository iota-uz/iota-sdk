package code

import (
	"compress/gzip"
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func wantBlocked(t *testing.T, err error, reason string) {
	t.Helper()
	var blocked *NetworkBlockedError
	if !errors.As(err, &blocked) {
		t.Fatalf("want NetworkBlocked %s, got %v", reason, err)
	}
	if blocked.Reason != reason {
		t.Fatalf("want reason %s, got %s (%v)", reason, blocked.Reason, blocked)
	}
}

// refusingDialer fails the test if the policy lets a request reach the network.
func refusingDialer(t *testing.T) FetchOption {
	t.Helper()
	return WithDialer(func(_ context.Context, _, address string) (net.Conn, error) {
		t.Errorf("dialed %s", address)
		return nil, errors.New("dial refused by the test")
	})
}

func TestGuardedFetcherRefusesBeforeTheNetwork(t *testing.T) {
	f := NewGuardedFetcher(refusingDialer(t), WithResolver(staticResolver{
		"mixed.example.com":  {"93.184.215.14", "10.0.0.7", "93.184.215.15"},
		"mapped.example.com": {"::ffff:127.0.0.1"},
	}))
	for url, reason := range map[string]string{
		"http://example.com/":                      "scheme",
		"ftp://example.com/":                       "scheme",
		"https://example.com:8443/":                "port",
		"https://example.com:80/":                  "port",
		"https://localhost/":                       "name",
		"https://api.localhost/":                   "name",
		"https://metadata/":                        "name",
		"https://metadata.google.internal/":        "name",
		"https://postgres.railway.internal/":       "name",
		"https://printer.local/":                   "name",
		"https://intranet/":                        "name",
		"https://127.0.0.1/":                       "private_address",
		"https://169.254.169.254/latest/meta-data": "private_address",
		"https://[::ffff:127.0.0.1]/":              "private_address",
		"https://[::1]/":                           "private_address",
		"https://100.100.100.200/":                 "private_address",
		"https://10.1.2.3/":                        "private_address",
		"https://192.168.0.1/":                     "private_address",
		"https://0.0.0.0/":                         "private_address",
		"https://mixed.example.com/":               "private_address",
		"https://mapped.example.com/":              "private_address",
	} {
		t.Run(url, func(t *testing.T) {
			_, err := f.Fetch(context.Background(), FetchRequest{URL: url, Method: "GET"})
			wantBlocked(t, err, reason)
		})
	}
}

func TestGuardedFetcherDialTimeCheck(t *testing.T) {
	// The second line: the default dialer checks the address it connects to, whatever the resolver said.
	f := NewGuardedFetcher().(*guardedFetcher)
	for _, addr := range []string{"127.0.0.1:443", "[::1]:443", "169.254.169.254:443", "93.184.215.14:8080"} {
		_, err := f.dial(context.Background(), "tcp", addr)
		wantBlocked(t, err, "private_address")
	}
}

func TestGuardedFetcherFetches(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/gzip":
			w.Header().Set("Content-Encoding", "gzip")
			gz := gzip.NewWriter(w)
			_, _ = gz.Write([]byte(strings.Repeat("a", 4096)))
			_ = gz.Close()
		default:
			w.Header().Set("X-Seen-Auth", r.Header.Get("Authorization"))
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte("hello " + r.Method))
		}
	}))
	defer srv.Close()
	f := testFetcher(srv)
	resp, err := f.Fetch(context.Background(), FetchRequest{URL: "https://example.com/x", Method: "POST",
		Header: http.Header{"Authorization": {"Bearer t"}}, Body: []byte("{}")})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Status != 201 || resp.StatusText != "Created" || string(resp.Body) != "hello POST" || resp.Header.Get("X-Seen-Auth") != "Bearer t" {
		t.Fatalf("got %+v %s", resp, resp.Body)
	}
	// The limit applies to the decoded body.
	_, err = testFetcher(srv, WithMaxBody(1000)).Fetch(context.Background(), FetchRequest{URL: "https://example.com/gzip"})
	if err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("decoded body over the limit: %v", err)
	}
}

func TestGuardedFetcherRedirects(t *testing.T) {
	var authSeen atomic.Value
	authSeen.Store("")
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/to-http":
			http.Redirect(w, r, "http://example.com/next", http.StatusFound)
		case "/to-port":
			http.Redirect(w, r, "https://example.com:8443/next", http.StatusFound)
		case "/to-local":
			http.Redirect(w, r, "https://localhost/next", http.StatusFound)
		case "/to-other":
			http.Redirect(w, r, "https://api.example.com/echo", http.StatusFound)
		case "/loop":
			http.Redirect(w, r, "/loop", http.StatusFound)
		case "/echo":
			authSeen.Store(r.Host + " " + r.Header.Get("Authorization") + " " + r.Header.Get("Cookie"))
			_, _ = w.Write([]byte("ok"))
		}
	}))
	defer srv.Close()
	f := testFetcher(srv, WithResolver(staticResolver{"example.com": {"93.184.215.14"}, "api.example.com": {"93.184.215.20"}}))
	ctx := context.Background()

	for path, reason := range map[string]string{"/to-http": "scheme", "/to-port": "port", "/to-local": "name", "/loop": "redirect_limit"} {
		_, err := f.Fetch(ctx, FetchRequest{URL: "https://example.com" + path})
		wantBlocked(t, err, reason)
	}

	resp, err := f.Fetch(ctx, FetchRequest{URL: "https://example.com/to-other", Header: http.Header{
		"Authorization": {"Bearer s3cr3t"}, "Cookie": {"sid=1"}, "X-Keep": {"1"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if !resp.Redirected || resp.URL != "https://api.example.com/echo" {
		t.Fatalf("got %+v", resp)
	}
	if got := authSeen.Load().(string); got != "api.example.com  " {
		t.Fatalf("credentials reached another origin: %q", got)
	}

	resp, err = f.Fetch(ctx, FetchRequest{URL: "https://example.com/to-other", Redirect: "manual"})
	if err != nil || resp.Status != http.StatusFound || resp.Redirected {
		t.Fatalf("manual: %v %+v", err, resp)
	}
	if _, err := f.Fetch(ctx, FetchRequest{URL: "https://example.com/to-other", Redirect: "error"}); err == nil {
		t.Fatal("redirect: error followed a redirect")
	}
}

func TestEnvFetchFromTheSandbox(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"method":"` + r.Method + `","key":"` + r.Header.Get("X-Key") + `"}`))
	}))
	defer srv.Close()
	res := runCode(t, `export default async (_: any, env: any) => {
  const r = await env.fetch("https://example.com/api", { method: "POST", headers: { "X-Key": "k" }, body: "{}" });
  const data = await r.json();
  return { outcome: "done", output: { status: r.status, ok: r.ok, type: r.headers["content-type"], data } };
}`, RunInput{Fetcher: testFetcher(srv)})
	if res.Err != nil {
		t.Fatal(res.Err)
	}
	if string(res.Output) != `{"status":200,"ok":true,"type":"application/json","data":{"method":"POST","key":"k"}}` {
		t.Fatalf("output %s", res.Output)
	}
}

func TestEnvFetchBlockedInTheSandbox(t *testing.T) {
	f := NewGuardedFetcher(refusingDialer(t))
	for _, url := range []string{"https://127.0.0.1/", "https://169.254.169.254/", "https://[::ffff:127.0.0.1]/", "https://100.100.100.200/", "http://example.com/"} {
		res := runCode(t, `export default async (_: any, env: any) => { await env.fetch("`+url+`"); return { outcome: "done" } }`,
			RunInput{Fetcher: f})
		wantReason(t, res, ReasonNetworkBlocked)
		if !strings.HasPrefix(res.Err.Message, "NetworkBlocked: ") || res.Err.Line != 1 {
			t.Fatalf("%s: %+v", url, res.Err)
		}
	}
	// Caught, it is an ordinary error the code handles.
	res := runCode(t, `export default async (_: any, env: any) => {
  try { await env.fetch("https://127.0.0.1/") } catch (e) { return { outcome: "done", output: (e as Error).name } }
  return { outcome: "done" };
}`, RunInput{Fetcher: f})
	if res.Err != nil || string(res.Output) != `"NetworkBlocked"` {
		t.Fatalf("got %+v %s", res.Err, res.Output)
	}
	// Without a Fetcher every fetch is blocked.
	res = runCode(t, `export default async (_: any, env: any) => { await env.fetch("https://example.com/"); return { outcome: "done" } }`, RunInput{})
	wantReason(t, res, ReasonNetworkBlocked)
}

func TestEnvFetchLimits(t *testing.T) {
	var inflight, peak atomic.Int32
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := inflight.Add(1)
		for {
			p := peak.Load()
			if n <= p || peak.CompareAndSwap(p, n) {
				break
			}
		}
		time.Sleep(20 * time.Millisecond)
		inflight.Add(-1)
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()
	res := runCode(t, `export default async (_: any, env: any) => {
  const results = await Promise.allSettled(Array.from({ length: 12 }, () => env.fetch("https://example.com/")));
  return { outcome: "done", output: results.map((r) => r.status) };
}`, RunInput{Fetcher: testFetcher(srv), Limits: Limits{MaxFetches: 10, MaxConcurrentFetches: 3}})
	if res.Err != nil {
		t.Fatal(res.Err)
	}
	if got := strings.Count(string(res.Output), "fulfilled"); got != 10 {
		t.Fatalf("%d fetches succeeded, want 10: %s", got, res.Output)
	}
	if p := peak.Load(); p > 3 {
		t.Fatalf("%d fetches ran at once, want at most 3", p)
	}
}

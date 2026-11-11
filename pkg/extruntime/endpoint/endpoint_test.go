package endpoint_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/endpoint"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/host"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/host/hosttest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/pool"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
)

const prelude = `const item = { id: "acme.test.item" };
const domain = (code, params) => Object.assign(new Error(code), { kind: "domain", code, params });
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));
`

const source = prelude + `
export const echo = async (ctx) => {
  const r = ctx.request;
  return {
    status: 201,
    headers: { "X-Reply": "yes", "Set-Cookie": "sid=stolen", "Content-Length": "999" },
    body: {
      user: ctx.user.userId, kind: ctx.user.kind, perms: ctx.user.has("p.read"), trigger: ctx.invocation.trigger,
      method: r.method, path: r.path, query: r.query, headers: r.headers, body: r.body ?? null,
      raw: Buffer.from(r.rawBody).toString("base64"), text: r.text(),
    },
  };
};
export const save = async (ctx) => {
  const row = await ctx.transaction(async (tx) => tx.insert(item, { label: ctx.request.body.label }));
  return { status: 200, body: { key: row.key } };
};
export const slow = async () => { await sleep(5000); return { status: 200 }; };
export const noContent = async () => ({ status: 204, body: { ignored: true } });
export const domainFail = async () => { throw domain("acme.a.rejected", { why: "no" }); };
export const crash = async () => { throw new Error("secret internal detail"); };
export const badStatus = async () => ({ status: 99 });
export const notAResponse = async () => "ok";
export const sync = (ctx) => ({ status: 200, headers: { "Content-Type": "text/plain" }, body: "plain" });
`

type fakeSource struct {
	mu  sync.Mutex
	eps []endpoint.Endpoint
}

func (f *fakeSource) ActiveEndpoints(context.Context) ([]endpoint.Endpoint, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]endpoint.Endpoint(nil), f.eps...), nil
}

type fixture struct {
	t   *testing.T
	env *hosttest.Env
	src *fakeSource
	h   *endpoint.Handler
	art pool.Artifact
	cfg endpoint.Config
}

type option func(*endpoint.Config)

func newFixture(t *testing.T, opts ...option) *fixture {
	t.Helper()
	env := hosttest.New(t)
	h, err := host.New(host.Config{
		Pool: env.NewPool(1), DB: env.DB, Bindings: env.Bindings, Catalog: env.Catalog, Data: env.Store, Artifacts: env.Artifacts, Logger: env.Logger,
	})
	require.NoError(t, err)
	f := &fixture{t: t, env: env, src: &fakeSource{}}
	f.art = env.Artifacts.Add(t, "acme.a", "1.0.0", source)
	f.cfg = endpoint.Config{
		Source: f.src, Runner: h, Owners: &endpoint.PgOwners{DB: env.DB}, Subjects: env.Subjects,
		// The test session: a user is "signed in" when the request carries X-Test-User.
		SessionSubject: func(ctx context.Context) (spi.Subject, bool) { return spi.Subject{}, false },
		PublicTenant:   func(*http.Request) (uuid.UUID, bool) { return env.Tenant, true },
		Timeout:        20 * time.Second,
		Logger:         nil,
	}
	for _, o := range opts {
		o(&f.cfg)
	}
	f.add("acme.a.echo", "echo", http.MethodPost, "/hook", false)
	return f
}

func (f *fixture) add(id, export, method, path string, public bool) {
	f.src.mu.Lock()
	f.src.eps = append(f.src.eps, endpoint.Endpoint{ID: id, Package: "acme.a", Version: "1.0.0", Export: export, Method: method, Path: path, Public: public, Artifact: f.art})
	f.src.mu.Unlock()
}

// handler builds the handler lazily so options see the final config.
func (f *fixture) handler() *endpoint.Handler {
	if f.h == nil {
		cfg := f.cfg
		cfg.SessionSubject = func(ctx context.Context) (spi.Subject, bool) {
			r, _ := ctx.Value(reqKey{}).(*http.Request)
			if r == nil || r.Header.Get("X-Test-User") == "" {
				return spi.Subject{}, false
			}
			id, _ := strconv.ParseInt(r.Header.Get("X-Test-User"), 10, 64)
			var perms []string
			if p := r.Header.Get("X-Test-Perms"); p != "" {
				perms = strings.Split(p, ",")
			}
			return f.env.Subject(id, perms...), true
		}
		h, err := endpoint.NewHandler(cfg)
		require.NoError(f.t, err)
		f.h = h
	}
	return f.h
}

type reqKey struct{}

// reply is the part of a response the tests inspect besides its body.
type reply struct {
	StatusCode int
	Header     http.Header
}

func (f *fixture) do(method, target string, body []byte, headers map[string]string) (reply, []byte) {
	f.t.Helper()
	req := httptest.NewRequest(method, target, bytes.NewReader(body))
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	req = req.WithContext(context.WithValue(req.Context(), reqKey{}, req))
	rec := httptest.NewRecorder()
	f.handler().ServeHTTP(rec, req)
	res := rec.Result()
	out, err := io.ReadAll(res.Body)
	require.NoError(f.t, err)
	require.NoError(f.t, res.Body.Close())
	return reply{StatusCode: res.StatusCode, Header: res.Header}, out
}

func jsonMap(t *testing.T, b []byte) map[string]any {
	t.Helper()
	var m map[string]any
	require.NoError(t, json.Unmarshal(b, &m), string(b))
	return m
}

func errCode(t *testing.T, b []byte) string {
	t.Helper()
	e, _ := jsonMap(t, b)["error"].(map[string]any)
	require.NotNil(t, e, string(b))
	return e["code"].(string)
}

const base = "/_ext/endpoints/acme.a"

func TestAuthenticatedRequestRunsAsTheSessionUserWithTheFullRequest(t *testing.T) {
	f := newFixture(t)
	raw := []byte("{\"policy\":\"x\"}\n")
	res, body := f.do(http.MethodPost, base+"/hook?a=1&b=two&a=ignored", raw, map[string]string{
		"Content-Type": "application/json", "X-Test-User": "7", "X-Test-Perms": "p.read",
		"Cookie": "sid=secret", "Authorization": "Bearer secret", "X-Custom": "v",
	})
	require.Equal(t, http.StatusCreated, res.StatusCode, string(body))
	require.Equal(t, "yes", res.Header.Get("X-Reply"))
	require.Empty(t, res.Header.Get("Set-Cookie"), "a package cannot set cookies on the application's origin")
	require.NotEqual(t, "999", res.Header.Get("Content-Length"))
	require.Equal(t, "application/json; charset=utf-8", res.Header.Get("Content-Type"))

	got := jsonMap(t, body)
	require.Equal(t, "7", got["user"], "the session user is the subject")
	require.Equal(t, "initiator", got["kind"])
	require.Equal(t, true, got["perms"])
	require.Equal(t, "endpoint", got["trigger"])
	require.Equal(t, "POST", got["method"])
	require.Equal(t, "/hook", got["path"])
	require.Equal(t, map[string]any{"a": "1", "b": "two"}, got["query"])
	require.Equal(t, map[string]any{"policy": "x"}, got["body"])
	require.Equal(t, base64.StdEncoding.EncodeToString(raw), got["raw"], "exact bytes")
	require.Equal(t, string(raw), got["text"])
	headers := got["headers"].(map[string]any)
	require.Equal(t, "v", headers["x-custom"])
	require.Equal(t, "application/json", headers["content-type"])
	for _, secret := range []string{"cookie", "authorization"} {
		require.NotContains(t, headers, secret)
	}

	var trigger, subjectKind, binding string
	var user int64
	require.NoError(t, f.env.DB.QueryRow(context.Background(),
		`SELECT trigger_kind, trigger_ref, subject_kind, subject_user_id, binding_package FROM extruntime.invocations WHERE contract = 'acme.a.echo'`).
		Scan(&trigger, new(string), &subjectKind, &user, &binding))
	require.Equal(t, []any{"endpoint", "initiator", int64(7), "acme.a"}, []any{trigger, subjectKind, user, binding})
	var instances int
	require.NoError(t, f.env.DB.QueryRow(context.Background(), `SELECT count(*) FROM extruntime.workflow_instances`).Scan(&instances))
	require.Zero(t, instances, "an endpoint call leaves no durable instance")
}

func TestFormAndOpaqueBodies(t *testing.T) {
	f := newFixture(t)
	auth := map[string]string{"X-Test-User": "7"}

	form := []byte("name=Ali&tag=a&tag=b&city=%D0%A2")
	res, body := f.do(http.MethodPost, base+"/hook", form, merge(auth, "Content-Type", "application/x-www-form-urlencoded; charset=utf-8"))
	require.Equal(t, http.StatusCreated, res.StatusCode, string(body))
	got := jsonMap(t, body)
	require.Equal(t, map[string]any{"name": "Ali", "tag": []any{"a", "b"}, "city": "Т"}, got["body"])
	require.Equal(t, base64.StdEncoding.EncodeToString(form), got["raw"])

	binary := []byte{0xff, 0xfe, 0x00, 0x01, 0x80}
	_, body = f.do(http.MethodPost, base+"/hook", binary, merge(auth, "Content-Type", "application/octet-stream"))
	got = jsonMap(t, body)
	require.Nil(t, got["body"], "no parsed form for opaque payloads")
	require.Equal(t, base64.StdEncoding.EncodeToString(binary), got["raw"], "the exact bytes still arrive")

	_, body = f.do(http.MethodPost, base+"/hook", []byte("{not json"), merge(auth, "Content-Type", "application/json"))
	got = jsonMap(t, body)
	require.Nil(t, got["body"], "a body that does not parse has no parsed form")
	require.Equal(t, "{not json", got["text"])

	_, body = f.do(http.MethodPost, base+"/hook", nil, auth)
	require.Empty(t, jsonMap(t, body)["raw"])
}

func merge(m map[string]string, kv ...string) map[string]string {
	out := map[string]string{}
	for k, v := range m {
		out[k] = v
	}
	for i := 0; i < len(kv); i += 2 {
		out[kv[i]] = kv[i+1]
	}
	return out
}

func TestPublicEndpointRunsAsTheTriggerOwner(t *testing.T) {
	f := newFixture(t)
	f.add("acme.a.hook", "echo", http.MethodPost, "/public", true)
	f.env.Subjects.Grant(42, "p.read")
	_, err := f.env.DB.Exec(context.Background(), `INSERT INTO extruntime.trigger_owners (trigger_id, tenant_id, user_id) VALUES ('acme.a.hook', $1, 42)`, f.env.Tenant)
	require.NoError(t, err)

	res, body := f.do(http.MethodPost, base+"/public", []byte(`{}`), map[string]string{"Content-Type": "application/json"})
	require.Equal(t, http.StatusCreated, res.StatusCode, string(body))
	got := jsonMap(t, body)
	require.Equal(t, "42", got["user"], "no session: the owner is the subject")
	require.Equal(t, "owner", got["kind"])
	require.Equal(t, true, got["perms"], "with the rights the owner holds now")

	var kind string
	require.NoError(t, f.env.DB.QueryRow(context.Background(), `SELECT subject_kind FROM extruntime.invocations WHERE contract = 'acme.a.hook'`).Scan(&kind))
	require.Equal(t, "owner", kind)

	t.Run("a session wins over the owner", func(t *testing.T) {
		_, body := f.do(http.MethodPost, base+"/public", []byte(`{}`), map[string]string{"X-Test-User": "7"})
		got := jsonMap(t, body)
		require.Equal(t, "7", got["user"])
		require.Equal(t, "initiator", got["kind"])
	})

	t.Run("an owner whose rights were revoked still runs, with the current rights", func(t *testing.T) {
		f.env.Subjects.Grant(42)
		_, body := f.do(http.MethodPost, base+"/public", []byte(`{}`), nil)
		require.Equal(t, false, jsonMap(t, body)["perms"])
		f.env.Subjects.Grant(42, "p.read")
	})

	t.Run("a disabled owner makes the endpoint unavailable", func(t *testing.T) {
		f.env.Subjects.Block(42, "disabled")
		res, body := f.do(http.MethodPost, base+"/public", []byte(`{}`), nil)
		require.Equal(t, http.StatusServiceUnavailable, res.StatusCode)
		require.Equal(t, endpoint.CodeOwnerBlocked, errCode(t, body))
		f.env.Subjects.Grant(42, "p.read")
	})

	t.Run("a tenant without an owner gets 503 naming the endpoint", func(t *testing.T) {
		f2 := newFixture(t)
		f2.add("acme.a.hook", "echo", http.MethodPost, "/public", true)
		res, body := f2.do(http.MethodPost, base+"/public", []byte(`{}`), nil)
		require.Equal(t, http.StatusServiceUnavailable, res.StatusCode)
		require.Equal(t, endpoint.CodeOwnerMissing, errCode(t, body))
		require.Contains(t, string(body), "acme.a.hook")
		var runs int
		require.NoError(t, f2.env.DB.QueryRow(context.Background(), `SELECT count(*) FROM extruntime.invocations`).Scan(&runs))
		require.Zero(t, runs, "the handler never ran")
	})

	t.Run("an unknown tenant gets 503", func(t *testing.T) {
		f3 := newFixture(t, func(c *endpoint.Config) {
			c.PublicTenant = func(*http.Request) (uuid.UUID, bool) { return uuid.Nil, false }
		})
		f3.add("acme.a.hook", "echo", http.MethodPost, "/public", true)
		res, body := f3.do(http.MethodPost, base+"/public", []byte(`{}`), nil)
		require.Equal(t, http.StatusServiceUnavailable, res.StatusCode)
		require.Equal(t, endpoint.CodeNoTenant, errCode(t, body))
	})
}

func TestRoutingAndAuthentication(t *testing.T) {
	f := newFixture(t)
	f.add("acme.a.get", "echo", http.MethodGet, "/hook", false)

	res, body := f.do(http.MethodPost, base+"/hook", nil, nil)
	require.Equal(t, http.StatusUnauthorized, res.StatusCode, "an endpoint that is not public needs a session")
	require.Equal(t, endpoint.CodeUnauthenticated, errCode(t, body))

	res, body = f.do(http.MethodPut, base+"/hook", nil, map[string]string{"X-Test-User": "7"})
	require.Equal(t, http.StatusMethodNotAllowed, res.StatusCode)
	require.Equal(t, "GET, POST", res.Header.Get("Allow"))
	require.Equal(t, endpoint.CodeMethodNotAllow, errCode(t, body))

	for _, path := range []string{base + "/nope", "/_ext/endpoints/acme.other/hook", "/_ext/endpoints/acme.a", "/_ext/endpoints/", "/_ext/endpoints/acme.a/hook/extra"} {
		res, body = f.do(http.MethodPost, path, nil, map[string]string{"X-Test-User": "7"})
		require.Equal(t, http.StatusNotFound, res.StatusCode, path)
		require.Equal(t, endpoint.CodeNotFound, errCode(t, body))
	}

	res, _ = f.do(http.MethodGet, base+"/hook", nil, map[string]string{"X-Test-User": "7"})
	require.Equal(t, http.StatusCreated, res.StatusCode, "GET reaches the GET endpoint")
	var n int
	require.NoError(t, f.env.DB.QueryRow(context.Background(), `SELECT count(*) FROM extruntime.invocations WHERE contract = 'acme.a.get'`).Scan(&n))
	require.Equal(t, 1, n)
}

func TestBodyLimitAndDeadline(t *testing.T) {
	f := newFixture(t, func(c *endpoint.Config) { c.MaxBodyBytes = 64; c.Timeout = 700 * time.Millisecond })
	f.add("acme.a.slow", "slow", http.MethodPost, "/slow", false)
	auth := map[string]string{"X-Test-User": "7"}

	res, body := f.do(http.MethodPost, base+"/hook", bytes.Repeat([]byte("x"), 65), auth)
	require.Equal(t, http.StatusRequestEntityTooLarge, res.StatusCode)
	require.Equal(t, endpoint.CodeBodyTooLarge, errCode(t, body))
	res, _ = f.do(http.MethodPost, base+"/hook", bytes.Repeat([]byte("x"), 64), auth)
	require.Equal(t, http.StatusCreated, res.StatusCode, "the limit is inclusive")

	start := time.Now()
	res, body = f.do(http.MethodPost, base+"/slow", nil, auth)
	require.Equal(t, http.StatusGatewayTimeout, res.StatusCode, string(body))
	require.Equal(t, endpoint.CodeTimeout, errCode(t, body))
	require.Less(t, time.Since(start), 4*time.Second, "the handler was cut off at the deadline, not awaited")

	f.env.Eventually(10*time.Second, func() bool {
		res, _ := f.do(http.MethodPost, base+"/hook", nil, auth)
		return res.StatusCode == http.StatusCreated
	}, "the pool serves requests again after the cancelled one")
}

func TestHandlerOutcomesBecomeHTTPResponses(t *testing.T) {
	f := newFixture(t)
	auth := map[string]string{"X-Test-User": "7"}
	f.add("acme.a.nc", "noContent", http.MethodPost, "/nc", false)
	f.add("acme.a.dom", "domainFail", http.MethodPost, "/domain", false)
	f.add("acme.a.crash", "crash", http.MethodPost, "/crash", false)
	f.add("acme.a.badstatus", "badStatus", http.MethodPost, "/badstatus", false)
	f.add("acme.a.notresp", "notAResponse", http.MethodPost, "/notresp", false)
	f.add("acme.a.sync", "sync", http.MethodPost, "/sync", false)
	f.add("acme.a.save", "save", http.MethodPost, "/save", false)

	res, body := f.do(http.MethodPost, base+"/nc", nil, auth)
	require.Equal(t, http.StatusNoContent, res.StatusCode)
	require.Empty(t, body)

	res, _ = f.do(http.MethodPost, base+"/sync", nil, auth)
	require.Equal(t, http.StatusOK, res.StatusCode, "a synchronous handler works")
	require.Equal(t, "text/plain", res.Header.Get("Content-Type"), "the handler chose the content type")

	res, body = f.do(http.MethodPost, base+"/domain", nil, auth)
	require.Equal(t, http.StatusUnprocessableEntity, res.StatusCode)
	require.Equal(t, "acme.a.rejected", errCode(t, body), "a domain error keeps its code and params")
	require.JSONEq(t, `{"why":"no"}`, string(mustMarshal(t, jsonMap(t, body)["error"].(map[string]any)["params"])))

	res, body = f.do(http.MethodPost, base+"/crash", nil, auth)
	require.Equal(t, http.StatusInternalServerError, res.StatusCode)
	require.NotContains(t, string(body), "secret internal detail", "internals are not leaked")
	require.Equal(t, endpoint.CodeInternal, errCode(t, body))
	require.NotEmpty(t, jsonMap(t, body)["invocation"], "support can find the recorded invocation")

	for _, path := range []string{"/badstatus", "/notresp"} {
		res, body = f.do(http.MethodPost, base+path, nil, auth)
		require.Equal(t, http.StatusBadGateway, res.StatusCode, path)
		require.Equal(t, endpoint.CodeBadResponse, errCode(t, body))
	}

	// the endpoint context is a command context: ctx.transaction commits data
	res, body = f.do(http.MethodPost, base+"/save", []byte(`{"label":"from-endpoint"}`), merge(auth, "Content-Type", "application/json"))
	require.Equal(t, http.StatusOK, res.StatusCode, string(body))
	require.Equal(t, []string{"from-endpoint"}, f.env.Items())
	require.Zero(t, f.idle())
}

func mustMarshal(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	require.NoError(t, err)
	return b
}

func (f *fixture) idle() int {
	var n int
	require.NoError(f.t, f.env.DB.QueryRow(context.Background(),
		`SELECT count(*) FROM pg_stat_activity WHERE datname = current_database() AND state LIKE 'idle in transaction%'`).Scan(&n))
	return n
}

func TestConfigRequiresSourceAndRunner(t *testing.T) {
	_, err := endpoint.NewHandler(endpoint.Config{})
	require.Error(t, err)
	_, err = endpoint.NewHandler(endpoint.Config{Source: &fakeSource{}})
	require.Error(t, err)
}

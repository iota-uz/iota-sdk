package engine_test

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors/jev"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors/openai"
	"github.com/iota-uz/iota-sdk/pkg/automation/engine"
)

// provider is a fake chat completions API: reply gets the request body and answers (status, body).
type provider struct {
	srv   *httptest.Server
	mu    sync.Mutex
	reply func(body map[string]any) (int, string)
	seen  []map[string]any
}

func newProvider(t *testing.T) *provider {
	t.Helper()
	p := &provider{}
	p.srv = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var body map[string]any
		_ = json.Unmarshal(raw, &body)
		p.mu.Lock()
		p.seen = append(p.seen, body)
		reply := p.reply
		p.mu.Unlock()
		status, out := http.StatusOK, `{}`
		if reply != nil {
			status, out = reply(body)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(out))
	}))
	t.Cleanup(p.srv.Close)
	return p
}

// transport sends every request to the fake, whatever host it names.
func (p *provider) transport() http.RoundTripper {
	u, _ := url.Parse(p.srv.URL)
	base := p.srv.Client().Transport.(*http.Transport).Clone()
	base.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec // the test's fake
	return rtFunc(func(r *http.Request) (*http.Response, error) {
		r = r.Clone(r.Context())
		r.URL.Scheme, r.URL.Host = u.Scheme, u.Host
		return base.RoundTrip(r)
	})
}

type rtFunc func(*http.Request) (*http.Response, error)

func (f rtFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func (p *provider) requests() []map[string]any {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]map[string]any(nil), p.seen...)
}

func completion(content string, in, out int) (int, string) {
	b, _ := json.Marshal(map[string]any{
		"model":   "served-model",
		"choices": []any{map[string]any{"finish_reason": "stop", "message": map[string]any{"content": content}}},
		"usage":   map[string]any{"prompt_tokens": in, "completion_tokens": out},
	})
	return http.StatusOK, string(b)
}

// platformOnly is a platform credential of the openai kind, offered by opt-in.
type platformOnly struct{ creds connectors.Credentials }

func (p platformOnly) Platform(kind catalog.ConnectorKind) (connectors.Credentials, connectors.Identity, bool) {
	if kind != catalog.KindOpenAI {
		return nil, connectors.Identity{}, false
	}
	return p.creds, connectors.Identity{}, true
}
func (platformOnly) Kinds() []catalog.ConnectorKind {
	return []catalog.ConnectorKind{catalog.KindOpenAI}
}
func (platformOnly) Policy(catalog.ConnectorKind) connectors.PlatformPolicy {
	return connectors.PlatformOptIn
}

type aiEnv struct {
	*env
	store    connectors.Store
	grants   connectors.PlatformGrants
	dir      connectors.Directory
	provider *provider
}

const (
	defaultLLMModel = "gpt-4o-mini-2024-07-18"
	llmKey          = "sk-platform-key-98765"
)

// newAIEnv is an engine whose catalog has the openai and jev connectors over a fake provider and the
// platform key of openai offered by opt-in.
func newAIEnv(t *testing.T) *aiEnv {
	t.Helper()
	keys, err := connectors.NewKeyring("k1", map[string][]byte{"k1": []byte("0123456789abcdef0123456789abcdef")})
	require.NoError(t, err)
	ae := &aiEnv{provider: newProvider(t)}
	ae.env = newEnvWith(t, func(db *pgxpool.Pool) catalog.Availability {
		ae.grants = connectors.NewPlatformGrants(db)
		ae.store = connectors.NewStore(db, keys, nil, platformOnly{creds: connectors.Credentials{"api_key": llmKey, "base_url": ae.provider.srv.URL}}, ae.grants)
		return ae.store
	})
	ae.dir = connectors.NewDirectory(ae.reg)
	tr := ae.provider.transport()
	require.NoError(t, ae.dir.Register(openai.New(openai.WithTransport(tr), openai.WithDefaultModel(defaultLLMModel))))
	require.NoError(t, ae.dir.Register(jev.New(jev.WithTransport(tr), jev.WithDefaultModel("openai/gpt-4o-mini-2024-07-18"))))
	return ae
}

func (ae *aiEnv) proc() *proc {
	return ae.env.proc(func(c *engine.Config) {
		c.Credentials = ae.store
		c.Connectors = ae.dir
		c.Usage = connectors.NewUsageRecorder(ae.db)
	})
}

func (ae *aiEnv) connect(kind catalog.ConnectorKind, creds connectors.Credentials) {
	ae.t.Helper()
	_, err := ae.store.Save(context.Background(), connectors.Connection{TenantID: ae.tenant, Kind: kind, Name: "main", DisplayName: "Main"}, creds)
	require.NoError(ae.t, err)
}

const llmGraph = `{"name": "Summarize", "triggers": [{"id": "created", "event": "acme.order.created.v1"}],
 "nodes": [
  {"id": "llm", "type": "ai.llm", "params": {"prompt": "Summarize: {{ event.note }}", "format": "json",
    "schema": {"type": "object", "properties": {"label": {"type": "string"}, "score": {"type": "integer"}}, "required": ["label", "score"]}}},
  {"id": "ok", "type": "acme.note", "params": {"text": "ok {{ nodes.llm.output.json.label }} {{ nodes.llm.output.json.score }}"}},
  {"id": "bad", "type": "acme.note", "params": {"text": "failed {{ nodes.llm.output.error.code }}: {{ nodes.llm.output.error.message }}"}},
  {"id": "fin", "type": "end", "params": {}}
 ],
 "edges": [
  {"from": {"node": "created", "outcome": "fired"}, "to": "llm"},
  {"from": {"node": "llm", "outcome": "done"}, "to": "ok"},
  {"from": {"node": "llm", "outcome": "failed"}, "to": "bad"},
  {"from": {"node": "ok", "outcome": "done"}, "to": "fin"}, {"from": {"node": "ok", "outcome": "failed"}, "to": "fin"},
  {"from": {"node": "bad", "outcome": "done"}, "to": "fin"}, {"from": {"node": "bad", "outcome": "failed"}, "to": "fin"}
 ]}`

// runUntilDone publishes an event and drives the process until the run it starts is done.
func (ae *aiEnv) runUntilDone(p *proc, key string, note string) engine.RunSummary {
	ae.t.Helper()
	known := map[string]bool{}
	for _, r := range ae.runs(p, ae.human(), engine.RunFilter{}) {
		known[r.RunID.String()] = true
	}
	ae.publish(ae.tenant, orderCreated, key, order{Amount: 1, Note: note}, 0)
	var run engine.RunSummary
	p.until(func() bool {
		for _, r := range ae.runs(p, ae.human(), engine.RunFilter{}) {
			if !known[r.RunID.String()] && ae.status(p, r.RunID) == engine.RunDone {
				run = r
				return true
			}
		}
		return false
	}, "the new run is done")
	return run
}

// The model is written into the version when the graph is saved; the result of the call is typed by the
// schema and the tokens are accounted to the tenant's own connection.
func TestLLMNodeSavesItsModelAndRuns(t *testing.T) {
	ae := newAIEnv(t)
	ae.connect(catalog.KindOpenAI, connectors.Credentials{"api_key": "sk-tenant-key-1234", "base_url": ae.provider.srv.URL})
	ae.provider.reply = func(map[string]any) (int, string) { return completion(`{"label":"vip","score":4}`, 50, 7) }
	p := ae.proc()

	am := ae.create(p, ae.human(), llmGraph)
	var model string
	require.NoError(t, ae.db.QueryRow(context.Background(),
		`SELECT graph->'nodes'->0->'params'->>'model' FROM automation.versions WHERE tenant_id = $1 AND automation_id = $2 AND version = 1`, ae.tenant, am.ID).Scan(&model))
	assert.Equal(t, defaultLLMModel, model, "the empty model is saved as the default model")

	run := ae.runUntilDone(p, "o-1", "big order")
	assert.Equal(t, []string{"ok:ok vip 4"}, ae.effects(run.RunID))
	reqs := ae.provider.requests()
	require.Len(t, reqs, 1)
	assert.Equal(t, defaultLLMModel, reqs[0]["model"])
	assert.Contains(t, reqs[0]["messages"].([]any)[0].(map[string]any)["content"], "Summarize: big order")

	var in, out int64
	require.NoError(t, ae.db.QueryRow(context.Background(), `SELECT
		COALESCE(sum(quantity) FILTER (WHERE unit = 'token_in'), 0), COALESCE(sum(quantity) FILTER (WHERE unit = 'token_out'), 0)
		FROM automation.connector_usage WHERE tenant_id = $1 AND kind = 'openai' AND connection_id NOT LIKE 'platform:%' AND run_id = $2`, ae.tenant, run.RunID).Scan(&in, &out))
	assert.EqualValues(t, 50, in)
	assert.EqualValues(t, 7, out)
}

// An answer that does not fit the schema is one request, and the run takes the failed outcome with
// invalid_request.
func TestLLMAnswerOutsideTheSchemaFailsAfterOneRequest(t *testing.T) {
	ae := newAIEnv(t)
	ae.connect(catalog.KindOpenAI, connectors.Credentials{"api_key": "sk-tenant-key-1234", "base_url": ae.provider.srv.URL})
	ae.provider.reply = func(map[string]any) (int, string) { return completion(`{"label":"vip","score":"high"}`, 50, 7) }
	p := ae.proc()
	ae.create(p, ae.human(), llmGraph)

	run := ae.runUntilDone(p, "o-1", "x")
	effects := ae.effects(run.RunID)
	require.Len(t, effects, 1)
	assert.True(t, strings.HasPrefix(effects[0], "bad:failed invalid_request: "), effects[0])
	assert.Len(t, ae.provider.requests(), 1, "exactly one request")
}

// A provider that answers 429 is repeated by the engine, and every attempt that was answered is accounted.
func TestLLMRateLimitIsRepeated(t *testing.T) {
	ae := newAIEnv(t)
	ae.connect(catalog.KindOpenAI, connectors.Credentials{"api_key": "sk-tenant-key-1234", "base_url": ae.provider.srv.URL})
	var n int
	ae.provider.reply = func(map[string]any) (int, string) {
		n++
		if n < 3 {
			return http.StatusTooManyRequests, `{"error":{"message":"slow down","type":"rate_limit_exceeded"}}`
		}
		return completion(`{"label":"ok","score":1}`, 10, 2)
	}
	p := ae.env.proc(func(c *engine.Config) {
		c.Credentials = ae.store
		c.Connectors = ae.dir
		c.Usage = connectors.NewUsageRecorder(ae.db)
		c.CallLimits.BackoffBase = 5e6
	})
	ae.create(p, ae.human(), llmGraph)
	run := ae.runUntilDone(p, "o-1", "x")
	assert.Equal(t, []string{"ok:ok ok 1"}, ae.effects(run.RunID))
	assert.Len(t, ae.provider.requests(), 3)
}

const jevGraph = `{"name": "Route", "triggers": [{"id": "created", "event": "acme.order.created.v1"}],
 "nodes": [
  {"id": "cls", "type": "ai.jev", "params": {"input": "{'note': event.note, 'amount': event.amount, 'tags': ['a', 'b']}", "min_confidence": 0.9,
    "choices": [{"value": "refund", "description": "money back"}, {"value": "exchange", "description": "another item"}]}},
  {"id": "pick", "type": "acme.note", "params": {"text": "choice {{ nodes.cls.output.choice }}"}},
  {"id": "doubt", "type": "acme.note", "params": {"text": "low {{ nodes.cls.output.choice }} {{ nodes.cls.output.confidence }}"}},
  {"id": "bad", "type": "acme.note", "params": {"text": "failed {{ nodes.cls.output.error.code }}"}},
  {"id": "fin", "type": "end", "params": {}}
 ],
 "edges": [
  {"from": {"node": "created", "outcome": "fired"}, "to": "cls"},
  {"from": {"node": "cls", "outcome": "refund"}, "to": "pick"}, {"from": {"node": "cls", "outcome": "exchange"}, "to": "pick"},
  {"from": {"node": "cls", "outcome": "low_confidence"}, "to": "doubt"},
  {"from": {"node": "cls", "outcome": "failed"}, "to": "bad"},
  {"from": {"node": "pick", "outcome": "done"}, "to": "fin"}, {"from": {"node": "pick", "outcome": "failed"}, "to": "fin"},
  {"from": {"node": "doubt", "outcome": "done"}, "to": "fin"}, {"from": {"node": "doubt", "outcome": "failed"}, "to": "fin"},
  {"from": {"node": "bad", "outcome": "done"}, "to": "fin"}, {"from": {"node": "bad", "outcome": "failed"}, "to": "fin"}
 ]}`

func decisionBody(choice string, refund, exchange float64) func(map[string]any) (int, string) {
	return func(map[string]any) (int, string) {
		b, _ := json.Marshal(map[string]any{"choice": choice, "probabilities": map[string]float64{"refund": refund, "exchange": exchange}})
		return completion(string(b), 80, 12)
	}
}

// With min_confidence 0.9 an answer at 0.7 goes to low_confidence, and the chosen value stays in the
// output; one at 0.95 takes the choice; a refusal takes failed.
func TestJevTakesTheOutcomeOfTheChoiceAndOfTheConfidence(t *testing.T) {
	ae := newAIEnv(t)
	ae.connect(catalog.KindJEV, connectors.Credentials{"provider": "openrouter", "api_key": "sk-or-key-1234"})
	p := ae.proc()
	am := ae.create(p, ae.human(), jevGraph)
	var model, prompt string
	require.NoError(t, ae.db.QueryRow(context.Background(),
		`SELECT graph->'nodes'->0->'params'->>'model', graph->'nodes'->0->'params'->>'prompt' FROM automation.versions WHERE tenant_id = $1 AND automation_id = $2 AND version = 1`,
		ae.tenant, am.ID).Scan(&model, &prompt))
	assert.Equal(t, "openai/gpt-4o-mini-2024-07-18", model)
	assert.Contains(t, prompt, "When in doubt, choose the safest option", "the default instruction is saved with the node")

	ae.provider.reply = decisionBody("refund", 0.7, 0.3)
	run := ae.runUntilDone(p, "o-1", "wrong size")
	assert.Equal(t, []string{"doubt:low refund 0.7"}, ae.effects(run.RunID))

	ae.provider.reply = decisionBody("exchange", 0.05, 0.95)
	run = ae.runUntilDone(p, "o-2", "wrong size")
	assert.Equal(t, []string{"pick:choice exchange"}, ae.effects(run.RunID))

	ae.provider.reply = func(map[string]any) (int, string) {
		return http.StatusOK, `{"choices":[{"finish_reason":"stop","message":{"content":null,"refusal":"no"}}],"usage":{"prompt_tokens":5,"completion_tokens":1}}`
	}
	run = ae.runUntilDone(p, "o-3", "wrong size")
	assert.Equal(t, []string{"bad:failed rejected"}, ae.effects(run.RunID))

	// The input has values of different types, the list beside the strings; the request carries it
	// serialized with sorted keys.
	req := ae.provider.requests()[0]
	user := req["messages"].([]any)[1].(map[string]any)["content"].(string)
	assert.Contains(t, user, "{\n  \"amount\": 1,\n  \"note\": \"wrong size\",\n  \"tags\": [\n    \"a\",\n    \"b\"\n  ]\n}")
	assert.Equal(t, 3, ae.count(`SELECT count(*) FROM automation.connector_usage WHERE kind = 'jev' AND unit = 'token_in'`))
}

// A repeated dry run in cached mode does not call the provider, and the decision is the one of the model.
func TestAIDryRunCachedDoesNotCallTheProviderAgain(t *testing.T) {
	ae := newAIEnv(t)
	ae.connect(catalog.KindJEV, connectors.Credentials{"provider": "openrouter", "api_key": "sk-or-key-1234"})
	ae.connect(catalog.KindOpenAI, connectors.Credentials{"api_key": "sk-tenant-key-1234", "base_url": ae.provider.srv.URL})
	ae.provider.reply = func(body map[string]any) (int, string) {
		if strings.Contains(string(mustJSON(body)), `"probabilities"`) {
			return decisionBody("exchange", 0.05, 0.95)(body)
		}
		return completion(`{"label":"vip","score":4}`, 10, 2)
	}
	p := ae.proc()
	au := p.rt.Authoring()
	event := json.RawMessage(`{"kind": "acme.order.created.v1", "amount": 1, "note": "wrong size", "secret": ""}`)
	ctx := context.Background()

	for _, tc := range []struct {
		name, graph, node, outcome string
	}{
		{"jev", jevGraph, "cls", "exchange"},
		{"llm", llmGraph, "llm", "done"},
	} {
		before := len(ae.provider.requests())
		in := engine.DryRunInput{Graph: json.RawMessage(tc.graph), Decisions: engine.DecisionsCached,
			Subject: engine.SubjectRef{Entity: "acme.order", Key: "o-1"}, Events: []json.RawMessage{event}}
		for i := 0; i < 2; i++ {
			res, err := au.DryRun(ctx, ae.human(), in)
			require.NoError(t, err, tc.name)
			var step engine.TraceStep
			for _, s := range res.Trace {
				if s.Node == tc.node {
					step = s
				}
			}
			assert.Equal(t, tc.outcome, step.Outcome, "%s run %d", tc.name, i)
			assert.Equal(t, "executed", step.Status, "%s run %d", tc.name, i)
			assert.NotContains(t, string(step.Output), "sk-", "no key in the output")
			if tc.name == "jev" {
				// Only the secret fields of the credentials are masked: the provider is a public field.
				assert.Contains(t, string(step.Output), `"provider":"openrouter"`, "run %d", i)
				assert.Contains(t, string(step.Output), `"choice":"exchange"`, "run %d", i)
			}
		}
		assert.Len(t, ae.provider.requests(), before+1, "%s: the second dry run took the decision from the cache", tc.name)

		// Another input is another decision.
		in.Events = []json.RawMessage{json.RawMessage(`{"kind": "acme.order.created.v1", "amount": 1, "note": "another note", "secret": ""}`)}
		_, err := au.DryRun(ctx, ae.human(), in)
		require.NoError(t, err)
		assert.Len(t, ae.provider.requests(), before+2, "%s: a different input is a cache miss", tc.name)
	}
}

func mustJSON(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}

// The platform key is invisible to a tenant until its administrator enables it; the calls are then
// accounted to the tenant's usage on the platform connection; a run that started after the key was
// switched off ends with unavailable.
func TestPlatformKeyIsOptIn(t *testing.T) {
	ae := newAIEnv(t)
	ae.provider.reply = func(map[string]any) (int, string) { return completion(`{"label":"vip","score":4}`, 30, 5) }
	p := ae.proc()
	ctx := context.Background()

	view, err := ae.reg.ForTenant(ctx, ae.tenant)
	require.NoError(t, err)
	assert.NotContains(t, view.Catalog.Nodes, "ai.llm", "no node without a connection or an enabled platform key")
	assert.Equal(t, catalog.ReasonConnectionUnavailable, view.Unavailable["ai.llm"])
	_, _, err = p.rt.Service().Create(ctx, ae.human(), engine.CreateInput{Graph: json.RawMessage(llmGraph)})
	var verr *engine.ValidationError
	require.ErrorAs(t, err, &verr)
	assert.Equal(t, engine.CodeConnectionUnavailable, verr.Errors[0].Code)

	require.NoError(t, ae.grants.SetEnabled(ctx, ae.tenant, catalog.KindOpenAI, true, 1))
	view, err = ae.reg.ForTenant(ctx, ae.tenant)
	require.NoError(t, err)
	assert.Contains(t, view.Catalog.Nodes, "ai.llm")
	assert.Equal(t, []catalog.ConnectionRef{{ID: "platform:openai", Name: "platform"}}, view.Connections[catalog.KindOpenAI])

	// The graph names the platform key.
	named := strings.Replace(llmGraph, `"type": "ai.llm", "params": {`, `"type": "ai.llm", "params": {"connection": "platform:openai", `, 1)
	ae.create(p, ae.human(), named)
	run := ae.runUntilDone(p, "o-1", "x")
	assert.Equal(t, []string{"ok:ok vip 4"}, ae.effects(run.RunID))
	assert.Equal(t, int64(30), ae.sum(`SELECT COALESCE(sum(quantity), 0) FROM automation.connector_usage WHERE tenant_id = $1 AND connection_id = 'platform:openai' AND unit = 'token_in'`, ae.tenant))
	assert.Equal(t, int64(5), ae.sum(`SELECT COALESCE(sum(quantity), 0) FROM automation.connector_usage WHERE tenant_id = $1 AND connection_id = 'platform:openai' AND unit = 'token_out'`, ae.tenant))
	assert.Equal(t, defaultLLMModel, ae.provider.requests()[0]["model"])

	// The administrator switches the key off; the enabled graph is still enabled, and a run that starts
	// now cannot use the key.
	require.NoError(t, ae.grants.SetEnabled(ctx, ae.tenant, catalog.KindOpenAI, false, 1))
	before := len(ae.provider.requests())
	run = ae.runUntilDone(p, "o-2", "x")
	effects := ae.effects(run.RunID)
	require.Len(t, effects, 1)
	assert.True(t, strings.HasPrefix(effects[0], "bad:failed unavailable: "), effects[0])
	assert.Contains(t, effects[0], "platform key is not enabled")
	assert.Len(t, ae.provider.requests(), before, "the provider was not called")
}

func (e *env) sum(sql string, args ...any) int64 {
	e.t.Helper()
	var n int64
	require.NoError(e.t, e.db.QueryRow(context.Background(), sql, args...).Scan(&n))
	return n
}

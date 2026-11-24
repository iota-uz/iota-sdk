package automations_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/modules"
	"github.com/iota-uz/iota-sdk/modules/automations"
	"github.com/iota-uz/iota-sdk/modules/automations/permissions"
	"github.com/iota-uz/iota-sdk/modules/automations/presentation/graphs"
	"github.com/iota-uz/iota-sdk/modules/automations/presentation/integrations"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/role"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	corePerms "github.com/iota-uz/iota-sdk/modules/core/permissions"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors"
	"github.com/iota-uz/iota-sdk/pkg/automation/engine"
	"github.com/iota-uz/iota-sdk/pkg/automation/graph"
	"github.com/iota-uz/iota-sdk/pkg/composition"
	"github.com/iota-uz/iota-sdk/pkg/config"
	"github.com/iota-uz/iota-sdk/pkg/config/providers/static"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/wiring"
	"github.com/iota-uz/iota-sdk/pkg/itf"
)

// codeNode is a Code node type: its source is reviewed before a version is enabled.
type codeNode struct{}

func (codeNode) Spec() graph.NodeSpec {
	return graph.NodeSpec{Type: "acme.code", Title: "Code", Behavior: graph.BehaviorEffect, DryRun: graph.DryRunCapture, Outcomes: []string{"done"},
		Params: graph.MustSchemaParams(`{"type":"object","properties":{"source":{"type":"string"}},"required":["source"],"additionalProperties":false}`)}
}
func (codeNode) Requires() []catalog.ConnectorKind { return nil }
func (codeNode) Review(p map[string]any) engine.CodeNodeReview {
	src, _ := p["source"].(string)
	return engine.CodeNodeReview{Lang: "ts", Source: src, Secrets: []string{"crm.token"}}
}

// hooky is a connector with a webhook and a node that needs its connection.
type hooky struct{}

const hookyKind catalog.ConnectorKind = "hooky"

func (hooky) Kind() catalog.ConnectorKind       { return hookyKind }
func (hooky) Nodes() []catalog.NodeType         { return []catalog.NodeType{hookyNode{}} }
func (hooky) Triggers() []catalog.TriggerSource { return nil }
func (hooky) Descriptor() connectors.Descriptor {
	return connectors.Descriptor{Kind: hookyKind, Title: map[string]string{"en": "Hooky"}, Auth: connectors.AuthAPIKey,
		Credentials: json.RawMessage(`{"type":"object","properties":{"secret":{"type":"string","x-secret":true}}}`)}
}
func (hooky) Test(context.Context, connectors.Credentials) (connectors.Identity, error) {
	return connectors.Identity{Display: "hooky"}, nil
}
func (hooky) Verify(r *http.Request, _ []byte, _ connectors.Connection, creds connectors.Credentials) error {
	if r.Header.Get("X-Hooky-Secret") != creds["secret"] {
		return errors("bad signature")
	}
	return nil
}
func (hooky) Handle(_ context.Context, w http.ResponseWriter, _ []byte, _ connectors.Connection, _ connectors.Credentials) ([]engine.InboundEvent, error) {
	w.Header().Set("X-Hooky", "ack")
	return nil, nil
}

type errors string

func (e errors) Error() string { return string(e) }

type hookyNode struct{}

func (hookyNode) Spec() graph.NodeSpec {
	return graph.NodeSpec{Type: "hooky.note", Title: "Hooky note", Behavior: graph.BehaviorEffect, DryRun: graph.DryRunCapture, Outcomes: []string{"done"},
		Params: graph.MustSchemaParams(`{"type":"object","properties":{},"additionalProperties":false}`)}
}
func (hookyNode) Requires() []catalog.ConnectorKind { return []catalog.ConnectorKind{hookyKind} }

var allAutomations = []permission.Permission{
	permissions.AutomationsRead, permissions.AutomationsAuthor, permissions.AutomationsEnable, permissions.AutomationsOperate, permissions.ConnectionsManage,
}

// app is the SDK composition with the extension core: the automations module runs the engine and
// mounts its pages.
func app(t *testing.T, perms ...permission.Permission) (*itf.Suite, *composition.Container) {
	t.Helper()
	src, err := config.Build(static.New(map[string]any{"automation.connections.keys": "k1:" + strings.Repeat("ab", 32)}))
	require.NoError(t, err)
	components := []composition.Component{}
	for _, c := range modules.Components() {
		if c.Descriptor().Name != "automations" {
			components = append(components, c)
		}
	}
	components = append(components,
		automations.NewComponent(automations.WithConnectors(hooky{}), automations.WithoutBackground()),
		wiring.NewComponent(wiring.Options{Natives: modules.Natives()}),
	)
	s := itf.NewSuiteBuilder(t).WithComponents(components...).WithSource(src).AsUser(perms...).Build()
	// The persisted user, in the test tenant.
	s = s.AsUser(sameTenant(s, perms...))
	container := s.Env().Container
	reg, err := composition.Resolve[catalog.Registry](container)
	require.NoError(t, err)
	if _, ok := reg.Node("acme.code"); !ok {
		require.NoError(t, reg.RegisterNode(codeNode{}))
	}
	return mountAll(t, s), container
}

func mountAll(t *testing.T, s *itf.Suite) *itf.Suite {
	t.Helper()
	found := map[string]bool{}
	for _, c := range s.Env().App.Controllers() {
		switch c.Descriptor().ID {
		case graphs.ControllerID, graphs.HooksControllerID, integrations.ControllerID:
			s.Register(c)
			found[c.Descriptor().ID] = true
		}
	}
	require.True(t, found[graphs.ControllerID], "the automations pages are composed")
	require.True(t, found[graphs.HooksControllerID], "the webhook endpoint is composed")
	return s
}

type rpcReply struct {
	Result json.RawMessage `json:"result"`
	Error  *struct {
		Code string          `json:"code"`
		Data json.RawMessage `json:"data"`
	} `json:"error"`
}

func call(t *testing.T, s *itf.Suite, path, method string, params any, out any) *rpcReply {
	t.Helper()
	body := s.POST(path).JSON(map[string]any{"jsonrpc": "2.0", "id": 1, "method": method, "params": params}).Expect(t).Body()
	var r rpcReply
	require.NoError(t, json.Unmarshal([]byte(body), &r), body)
	if r.Error != nil {
		t.Logf("%s: %s", method, body)
	}
	if out != nil && r.Error == nil {
		require.NoError(t, json.Unmarshal(r.Result, out), string(r.Result))
	}
	return &r
}

func rpc(t *testing.T, s *itf.Suite, method string, params any, out any) *rpcReply {
	t.Helper()
	return call(t, s, "/automations/rpc", method, params, out)
}

func codeGraph(name, src string) map[string]any {
	return map[string]any{
		"name": name, "run_as": "owner",
		"triggers": []any{map[string]any{"id": "start", "event": engine.EventManual}},
		"nodes":    []any{map[string]any{"id": "code", "type": "acme.code", "params": map[string]any{"source": src}}, map[string]any{"id": "fin", "type": "end", "params": map[string]any{}}},
		"edges": []any{
			map[string]any{"from": map[string]any{"node": "start", "outcome": "fired"}, "to": "code"},
			map[string]any{"from": map[string]any{"node": "code", "outcome": "done"}, "to": "fin"},
		},
	}
}

// The pages render through the client host with the module's own bundle; without Automations.Read
// the section is a 403 and has no navigation entry.
func TestAutomationsPages(t *testing.T) {
	s, _ := app(t, allAutomations...)
	body := s.GET("/automations").Expect(t).Status(http.StatusOK).
		Contains(`"id":"automations.list"`).Contains(`"rpc":"/automations/rpc"`).Body()
	assert.Contains(t, body, `/assets/automation-graphs/`)
	s.GET("/automations/new").Expect(t).Status(http.StatusOK).Contains(`"view":"new"`)
	id := uuid.NewString()
	s.GET("/automations/" + id).Expect(t).Status(http.StatusOK).Contains(`"view":"editor"`).Contains(id)

	reader, _ := app(t, permissions.AutomationsRead)
	reader.GET("/automations/new").Expect(t).Status(http.StatusForbidden)
	other, _ := app(t, corePerms.UserRead)
	other.GET("/automations").Expect(t).Status(http.StatusForbidden).NotContains(`iota-client-route-mount`)
}

// Create (v1), enable without Code nodes, start manually: the run is the list's last run. A save that
// changes the code of an enabled graph is pending review and the enabled version keeps running; a review
// taken before another save no longer enables.
func TestCodeReviewBeforeEnabling(t *testing.T) {
	s, _ := app(t, allAutomations...)

	var created graphs.SaveResult
	require.Nil(t, rpc(t, s, graphs.MethodCreate, map[string]any{"graph": codeGraph("Daily digest", "return 1"), "layout": map[string]any{}}, &created).Error)
	require.True(t, created.OK, "%+v", created)
	require.Equal(t, 1, created.Version)
	id := created.ID

	var review graphs.ReviewResponse
	require.Nil(t, rpc(t, s, graphs.MethodEnableReview, map[string]any{"id": id, "version": 1}, &review).Error)
	require.Len(t, review.CodeNodes, 1)
	assert.Equal(t, "return 1", review.CodeNodes[0].Source)
	assert.Equal(t, []string{"crm.token"}, review.CodeNodes[0].Secrets, "secrets by name only")
	assert.Equal(t, graphs.RunAsOwner, review.RunAs)
	require.Len(t, review.Triggers, 1)
	assert.Equal(t, graphs.TriggerManual, review.Triggers[0].Kind)
	require.NotEmpty(t, review.CodeDigest)

	var enabled graphs.EnableResponse
	require.Nil(t, rpc(t, s, graphs.MethodEnable, map[string]any{"id": id, "version": 1, "code_digest": "stale"}, &enabled).Error)
	assert.Equal(t, graphs.ErrReviewRequired, enabled.Error, "a digest of code not shown is refused")
	require.Nil(t, rpc(t, s, graphs.MethodEnable, map[string]any{"id": id, "version": 1, "code_digest": review.CodeDigest}, &enabled).Error)
	require.True(t, enabled.OK, "%+v", enabled)
	assert.Equal(t, 1, enabled.EnabledVersion)

	var started graphs.RunManualResponse
	require.Nil(t, rpc(t, s, graphs.MethodRunManual, map[string]any{"id": id, "trigger_id": "start", "input": map[string]any{}}, &started).Error)
	require.True(t, started.OK, "%+v", started)
	var list graphs.ListResponse
	require.Nil(t, rpc(t, s, graphs.MethodList, map[string]any{}, &list).Error)
	require.Len(t, list.Items, 1)
	require.NotNil(t, list.Items[0].LastRun)
	assert.Equal(t, started.RunID, list.Items[0].LastRun.RunID)
	assert.Equal(t, []graphs.TriggerView{{Kind: graphs.TriggerManual, Title: "Manual start"}}, list.Items[0].Triggers)

	// v2 changes the code: saved, pending review; v1 keeps running.
	var saved graphs.SaveResult
	require.Nil(t, rpc(t, s, graphs.MethodSave, map[string]any{"id": id, "base": 1, "graph": codeGraph("Daily digest", "return 2"), "layout": map[string]any{}}, &saved).Error)
	require.True(t, saved.OK)
	assert.Equal(t, 2, saved.Version)
	assert.True(t, saved.PendingReview)
	var got graphs.GetResponse
	require.Nil(t, rpc(t, s, graphs.MethodGet, map[string]any{"id": id}, &got).Error)
	require.NotNil(t, got.Automation.EnabledVersion)
	assert.Equal(t, 1, *got.Automation.EnabledVersion)

	// A retry of the same save (its answer lost) is a conflict naming the user, not a duplicate.
	require.Nil(t, rpc(t, s, graphs.MethodSave, map[string]any{"id": id, "base": 1, "graph": codeGraph("Daily digest", "return 2"), "layout": map[string]any{}}, &saved).Error)
	require.NotNil(t, saved.Conflict)
	assert.Equal(t, 2, saved.Conflict.Version)
	assert.NotEmpty(t, saved.Conflict.By, "the author by name (#<id> when the name is unknown)")

	// The review of v2, then v3 is saved: enabling v2 with that review is refused.
	var review2 graphs.ReviewResponse
	require.Nil(t, rpc(t, s, graphs.MethodEnableReview, map[string]any{"id": id, "version": 2}, &review2).Error)
	require.Nil(t, rpc(t, s, graphs.MethodSave, map[string]any{"id": id, "base": 2, "graph": codeGraph("Daily digest", "return 3"), "layout": map[string]any{}}, &saved).Error)
	require.True(t, saved.OK)
	require.Nil(t, rpc(t, s, graphs.MethodEnable, map[string]any{"id": id, "version": 2, "code_digest": review2.CodeDigest}, &enabled).Error)
	assert.False(t, enabled.OK)
	assert.Equal(t, graphs.ErrReviewRequired, enabled.Error)
	var latest graphs.ReviewResponse
	require.Nil(t, rpc(t, s, graphs.MethodEnableReview, map[string]any{"id": id, "version": 0}, &latest).Error)
	assert.Equal(t, 3, latest.Version, "version 0 reviews the latest")
	require.Nil(t, rpc(t, s, graphs.MethodEnable, map[string]any{"id": id, "version": 3, "code_digest": latest.CodeDigest}, &enabled).Error)
	require.True(t, enabled.OK)

	var disabled graphs.DisableResponse
	require.Nil(t, rpc(t, s, graphs.MethodDisable, map[string]any{"id": id}, &disabled).Error)
	assert.True(t, disabled.OK)
	require.Nil(t, rpc(t, s, graphs.MethodList, map[string]any{"status": "disabled"}, &list).Error)
	require.Len(t, list.Items, 1)
	require.Nil(t, rpc(t, s, graphs.MethodList, map[string]any{"status": "enabled"}, &list).Error)
	assert.Empty(t, list.Items)
}

// A reader may not save; another tenant's graph is not found; a stale layout save is refused with the
// positions of the other tab.
func TestPermissionsAndLayoutConflict(t *testing.T) {
	s, container := app(t, allAutomations...)
	var created graphs.SaveResult
	require.Nil(t, rpc(t, s, graphs.MethodCreate, map[string]any{"graph": codeGraph("Notes", "return 1"), "layout": map[string]any{}}, &created).Error)
	require.True(t, created.OK)

	var lay graphs.SaveLayoutResponse
	require.Nil(t, rpc(t, s, graphs.MethodSaveLayout, map[string]any{"id": created.ID, "base_revision": created.LayoutRevision, "positions": map[string]any{"code": map[string]any{"x": 10, "y": 20}}}, &lay).Error)
	require.True(t, lay.OK)
	require.Nil(t, rpc(t, s, graphs.MethodSaveLayout, map[string]any{"id": created.ID, "base_revision": created.LayoutRevision, "positions": map[string]any{"code": map[string]any{"x": 99, "y": 99}}}, &lay).Error)
	assert.False(t, lay.OK)
	assert.Equal(t, graphs.ErrVersionConflict, lay.Error)
	assert.Equal(t, graphs.XY{X: 10, Y: 20}, lay.Positions["code"], "the positions of the newer tab stay")

	reader, _ := app(t, permissions.AutomationsRead)
	_ = container // another composition: another tenant
	r := rpc(t, reader, graphs.MethodSave, map[string]any{"id": created.ID, "base": 1, "graph": codeGraph("Notes", "x"), "layout": map[string]any{}}, nil)
	require.NotNil(t, r.Error)
	assert.Equal(t, "forbidden", r.Error.Code)
	r = rpc(t, reader, graphs.MethodGet, map[string]any{"id": created.ID}, nil)
	require.NotNil(t, r.Error, "the reader's tenant is another one")
	assert.Equal(t, "not_found", r.Error.Code)

	// Without Automations.Author the editor is read-only with the reason.
	var get graphs.GetResponse
	readerSameTenant := s.AsUser(sameTenant(s, permissions.AutomationsRead))
	require.Nil(t, rpc(t, readerSameTenant, graphs.MethodGet, map[string]any{"id": created.ID}, &get).Error)
	require.NotNil(t, get.ReadOnly)
	assert.Equal(t, "no_author", get.ReadOnly.Code)
	assert.False(t, get.Can.Author)
	assert.False(t, get.Can.Enable)
}

// An agent's session (or a request authenticated by a token) never enables.
func TestSessionOrigin(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/automations/rpc", nil)
	assert.Equal(t, engine.OriginAgent, graphs.SessionOrigin(r), "no session")
	r.Header.Set("Authorization", "Bearer x")
	assert.Equal(t, engine.OriginAgent, graphs.SessionOrigin(r))
}

// The webhook of a connection is served without a session or a CSRF token; deleting a connection an
// enabled graph uses is refused with that graph.
func TestWebhookAndConnectionInUse(t *testing.T) {
	s, container := app(t, allAutomations...)
	store, err := composition.Resolve[connectors.ConnectionStore](container)
	require.NoError(t, err)
	tenant := s.Env().Tenant.ID
	conn, err := store.Save(context.Background(), connectors.Connection{TenantID: tenant, Kind: hookyKind, Name: "main", DisplayName: "Main", Status: connectors.StatusConnected},
		connectors.Credentials{"secret": "s3cret"})
	require.NoError(t, err)

	h := hookHandler(t, s)
	post := func(path, secret string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString(`{}`))
		if secret != "" {
			req.Header.Set("X-Hooky-Secret", secret)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}
	rec := post(graphs.HooksPath+"/"+conn.ID.String(), "s3cret")
	assert.Equal(t, http.StatusOK, rec.Code, "no cookie, no CSRF token")
	assert.Equal(t, "ack", rec.Header().Get("X-Hooky"))
	assert.Equal(t, http.StatusNotFound, post(graphs.HooksPath+"/"+conn.ID.String(), "wrong").Code)
	assert.Equal(t, http.StatusNotFound, post(graphs.HooksPath+"/"+uuid.NewString(), "s3cret").Code)

	g := map[string]any{
		"name": "Hooky notes", "run_as": "owner",
		"triggers": []any{map[string]any{"id": "start", "event": engine.EventManual}},
		"nodes":    []any{map[string]any{"id": "note", "type": "hooky.note", "params": map[string]any{}}, map[string]any{"id": "fin", "type": "end", "params": map[string]any{}}},
		"edges": []any{
			map[string]any{"from": map[string]any{"node": "start", "outcome": "fired"}, "to": "note"},
			map[string]any{"from": map[string]any{"node": "note", "outcome": "done"}, "to": "fin"},
		},
	}
	var created graphs.SaveResult
	require.Nil(t, rpc(t, s, graphs.MethodCreate, map[string]any{"graph": g, "layout": map[string]any{}}, &created).Error)
	require.True(t, created.OK, "%+v", created)
	var review graphs.ReviewResponse
	require.Nil(t, rpc(t, s, graphs.MethodEnableReview, map[string]any{"id": created.ID, "version": 1}, &review).Error)
	var enabled graphs.EnableResponse
	require.Nil(t, rpc(t, s, graphs.MethodEnable, map[string]any{"id": created.ID, "version": 1, "code_digest": review.CodeDigest}, &enabled).Error)
	require.True(t, enabled.OK, "%+v", enabled)

	var del integrations.DeleteResponse
	require.Nil(t, call(t, s, integrations.DefaultPath+"/rpc", integrations.MethodDelete, map[string]any{"id": conn.ID.String()}, &del).Error)
	assert.False(t, del.OK)
	assert.Equal(t, integrations.ErrConnectionInUse, del.Error)
	require.Len(t, del.Usages, 1)
	assert.Equal(t, created.ID, del.Usages[0].AutomationID, "the use links the graph")
}

// hookHandler is the composed webhook controller's router: a request with no session middleware.
func hookHandler(t *testing.T, s *itf.Suite) http.Handler {
	t.Helper()
	for _, c := range s.Env().App.Controllers() {
		if c.Descriptor().ID == graphs.HooksControllerID {
			return routerOf(c)
		}
	}
	t.Fatal("no webhook controller")
	return nil
}

func routerOf(c application.Controller) http.Handler {
	r := mux.NewRouter()
	c.Register(r)
	return r
}

// sameTenant is the suite's user in its tenant with perms only.
func sameTenant(s *itf.Suite, perms ...permission.Permission) user.User {
	u := s.Env().User
	r := role.New("reader", role.WithID(2), role.WithPermissions(perms), role.WithTenantID(s.Env().Tenant.ID))
	return user.New(u.FirstName(), u.LastName(), u.Email(), u.UILanguage(), user.WithID(u.ID()), user.WithTenantID(s.Env().Tenant.ID), user.WithRoles([]role.Role{r}))
}

// Every key the automations screens read is in the module's locale (the host passes GraphsMessageKeys).
func TestGraphsScreenKeysAreInTheLocale(t *testing.T) {
	t.Parallel()
	keys, err := automations.GraphsMessageKeys()
	require.NoError(t, err)
	have := map[string]bool{}
	for _, k := range keys {
		have[k] = true
	}
	dir := filepath.Join("modules", "automations", "presentation", "graphs", "web")
	var src []byte
	for _, f := range []string{"Automations.tsx", "List.tsx", "EditorPage.tsx", "Dialogs.tsx", "RunDialog.tsx", "model.ts"} {
		b, err := os.ReadFile(filepath.Join(dir, f))
		require.NoError(t, err)
		src = append(src, b...)
	}
	found := 0
	for _, m := range regexp.MustCompile(`\bt\('([A-Za-z_.]+)'`).FindAllSubmatch(src, -1) {
		found++
		assert.True(t, have["Automations.Graphs."+string(m[1])], "missing locale key %s", m[1])
	}
	assert.Greater(t, found, 80)
	// Keys built from values: statuses, filters, counted phrases, field errors.
	for _, k := range []string{
		"Run.Queued", "Run.Running", "Run.Waiting", "Run.Blocked", "Run.Done", "Run.Failed", "Run.Cancelled", "Run.FilterFailed",
		"Filter.All", "Filter.Enabled", "Filter.Disabled", "Filter.Attention", "Run.Field.Required", "Run.Field.Number", "Run.Field.Json",
	} {
		assert.True(t, have["Automations.Graphs."+k], "missing locale key %s", k)
	}
	for _, base := range []string{"Disable.Pending", "Disable.Cancelled", "Delete.Runs", "Delete.Waits"} {
		for _, form := range []string{"Singular", "Paucal", "Multiple", "Plural"} {
			assert.True(t, have["Automations.Graphs."+base+"."+form], "missing locale key %s.%s", base, form)
		}
	}
}

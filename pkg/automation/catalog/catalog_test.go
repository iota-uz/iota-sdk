package catalog_test

import (
	"context"
	"encoding/json"
	"errors"
	"os/exec"
	"sort"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/automation/expr"
	"github.com/iota-uz/iota-sdk/pkg/automation/graph"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/native"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
)

// The catalog does not depend on SDK modules, the engine, connectors or a database driver.
func TestCatalogDependencies(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go is not on PATH")
	}
	out, err := exec.Command("go", "list", "-f", `{{join .Imports " "}}`, ".").CombinedOutput()
	require.NoError(t, err, string(out))
	for _, imp := range strings.Fields(string(out)) {
		for _, banned := range []string{"github.com/iota-uz/iota-sdk/modules/", "github.com/iota-uz/iota-sdk/pkg/automation/connectors", "github.com/iota-uz/iota-sdk/pkg/automation/engine", "github.com/jackc/pgx"} {
			assert.False(t, strings.HasPrefix(imp, banned), "catalog imports %s", imp)
		}
	}
}

type fakeAvailability map[catalog.ConnectorKind][]catalog.ConnectionRef

func (f fakeAvailability) Connected(context.Context, uuid.UUID) (map[catalog.ConnectorKind][]catalog.ConnectionRef, error) {
	return f, nil
}

type node struct {
	spec     graph.NodeSpec
	requires []catalog.ConnectorKind
}

func (n node) Spec() graph.NodeSpec              { return n.spec }
func (n node) Requires() []catalog.ConnectorKind { return n.requires }

type permissionedNode struct {
	node
	perm string
}

func (n permissionedNode) Permission() string { return n.perm }

type localizedNode struct{ node }

func (n localizedNode) Localize(s graph.NodeSpec, locale string) graph.NodeSpec {
	s.Title = s.Title + "@" + locale
	return s
}

type source struct {
	events   []graph.EventSpec
	requires []catalog.ConnectorKind
}

func (s source) Events() []graph.EventSpec         { return s.events }
func (s source) Requires() []catalog.ConnectorKind { return s.requires }

type connector struct {
	kind     catalog.ConnectorKind
	nodes    []catalog.NodeType
	triggers []catalog.TriggerSource
}

func (c connector) Kind() catalog.ConnectorKind       { return c.kind }
func (c connector) Nodes() []catalog.NodeType         { return c.nodes }
func (c connector) Triggers() []catalog.TriggerSource { return c.triggers }

func callNode(t *testing.T, typ string) node {
	t.Helper()
	p, err := graph.FieldsParams(expr.Fields{{Name: "text", Field: expr.StringField}})
	require.NoError(t, err)
	return node{spec: graph.NodeSpec{
		Type: typ, Title: typ, Behavior: graph.BehaviorCall, DryRun: graph.DryRunCapture, Params: p, Templates: []string{"text"},
		Outcomes: []string{"sent", "failed"},
	}}
}

func telegram(t *testing.T) connector {
	t.Helper()
	return connector{
		kind:  catalog.KindTelegramBot,
		nodes: []catalog.NodeType{callNode(t, "telegram.send")},
		triggers: []catalog.TriggerSource{source{events: []graph.EventSpec{{
			Key: "telegram.message", Title: "Message", Subject: graph.SubjectConversation,
			Payload: expr.Fields{{Name: "text", Field: expr.StringField}},
		}}}},
	}
}

func TestTelegramAppearsOnlyWithAConnection(t *testing.T) {
	ctx := context.Background()
	avail := fakeAvailability{}
	reg := catalog.NewRegistry(avail)
	require.NoError(t, reg.RegisterConnector(telegram(t)))
	require.NoError(t, reg.RegisterNode(callNode(t, "log.write")))

	v, err := reg.ForTenant(ctx, uuid.New())
	require.NoError(t, err)
	assert.NotContains(t, v.Catalog.Nodes, "telegram.send")
	assert.NotContains(t, v.Catalog.Events, "telegram.message")
	assert.Equal(t, catalog.ReasonConnectionUnavailable, v.Unavailable["telegram.send"])
	assert.Equal(t, catalog.ReasonConnectionUnavailable, v.Unavailable["telegram.message"])
	assert.Contains(t, v.Catalog.Nodes, "log.write")
	before := v.Catalog.Revision

	avail[catalog.KindTelegramBot] = []catalog.ConnectionRef{{ID: uuid.NewString(), Name: "support_bot"}}
	v, err = reg.ForTenant(ctx, uuid.New())
	require.NoError(t, err)
	assert.Contains(t, v.Catalog.Nodes, "telegram.send")
	assert.Contains(t, v.Catalog.Events, "telegram.message")
	assert.Empty(t, v.Unavailable)
	assert.NotEqual(t, before, v.Catalog.Revision)
	assert.Equal(t, []catalog.ConnectorKind{catalog.KindTelegramBot}, v.NodeRequires["telegram.send"])

	cv := v.JSON(nil)
	for _, n := range cv.Nodes {
		if n.Type == "telegram.send" {
			assert.Equal(t, []string{"telegram_bot"}, n.Requires)
			assert.Contains(t, string(n.Params), `"connection"`)
		}
		if n.Type == "log.write" {
			assert.Empty(t, n.Requires)
			assert.NotContains(t, string(n.Params), `"connection"`)
		}
	}
	// Node returns the executable node whatever the availability.
	n, ok := reg.Node("telegram.send")
	require.True(t, ok)
	assert.Equal(t, "telegram.send", n.Spec().Type)
}

const sendGraph = `{
  "name": "Reply",
  "triggers": [{"id": "msg", "event": "telegram.message", "params": %s}],
  "nodes": [{"id": "send", "type": "telegram.send", "params": %s}],
  "edges": [
    {"from": {"node": "msg", "outcome": "fired"}, "to": "send"},
    {"from": {"node": "send", "outcome": "sent"}, "to": "ignore"},
    {"from": {"node": "send", "outcome": "failed"}, "to": "ignore"}
  ]
}`

func validate(t *testing.T, v catalog.View, triggerParams, nodeParams string) []graph.GraphError {
	t.Helper()
	raw := strings.Replace(strings.Replace(sendGraph, "%s", triggerParams, 1), "%s", nodeParams, 1)
	g, errs := graph.Decode([]byte(raw))
	require.Empty(t, errs)
	return graph.Validate(v.Catalog, g, graph.Options{Rules: catalog.ConnectionRule(v)})
}

func codes(errs []graph.GraphError) []string {
	out := make([]string, len(errs))
	for i, e := range errs {
		out[i] = e.Path + ":" + e.Code
	}
	return out
}

func TestConnectionParamAndRule(t *testing.T) {
	ctx := context.Background()
	bot := uuid.NewString()
	avail := fakeAvailability{catalog.KindTelegramBot: {{ID: bot, Name: "support_bot"}}}
	reg := catalog.NewRegistry(avail)
	require.NoError(t, reg.RegisterConnector(telegram(t)))
	v, err := reg.ForTenant(ctx, uuid.New())
	require.NoError(t, err)

	// One connection: the param may be left out.
	assert.Empty(t, validate(t, v, `{}`, `{"text": "hi"}`))
	assert.Empty(t, validate(t, v, `{"connection": "`+bot+`"}`, `{"text": "hi", "connection": "`+bot+`"}`))
	// A connection that is not available.
	other := uuid.NewString()
	assert.Equal(t, []string{"triggers[0].params.connection:connection_unavailable", "nodes[0].params.connection:connection_unavailable"},
		codes(validate(t, v, `{"connection": "`+other+`"}`, `{"text": "hi", "connection": "`+other+`"}`)))
	// The param is checked as params: a number is refused.
	errs := validate(t, v, `{}`, `{"text": "hi", "connection": 5}`)
	require.NotEmpty(t, errs)
	assert.Equal(t, graph.CodeParams, errs[0].Code)

	// A tenant connection and the platform credential: the tenant's overrides, the param may stay out.
	avail[catalog.KindTelegramBot] = append(avail[catalog.KindTelegramBot], catalog.ConnectionRef{ID: "platform:telegram_bot", Name: "platform"})
	v, err = reg.ForTenant(ctx, uuid.New())
	require.NoError(t, err)
	assert.Empty(t, validate(t, v, `{}`, `{"text": "hi"}`))
	assert.Empty(t, validate(t, v, `{"connection": "platform:telegram_bot"}`, `{"text": "hi", "connection": "platform:telegram_bot"}`))

	// Two tenant connections: the param must name one.
	avail[catalog.KindTelegramBot] = append(avail[catalog.KindTelegramBot], catalog.ConnectionRef{ID: uuid.NewString(), Name: "sales_bot"})
	v, err = reg.ForTenant(ctx, uuid.New())
	require.NoError(t, err)
	assert.Equal(t, []string{"triggers[0].params.connection:connection_required", "nodes[0].params.connection:connection_required"},
		codes(validate(t, v, `{}`, `{"text": "hi"}`)))

	// A view where the connector is unavailable: the rule names the node and the trigger.
	delete(avail, catalog.KindTelegramBot)
	v, err = reg.ForTenant(ctx, uuid.New())
	require.NoError(t, err)
	got := codes(validate(t, v, `{}`, `{"text": "hi"}`))
	assert.Contains(t, got, "triggers[0].event:connection_unavailable")
	assert.Contains(t, got, "nodes[0].type:connection_unavailable")
}

func TestResolveConnectionPrefersTheFirstAndPicksTheKind(t *testing.T) {
	refs := map[catalog.ConnectorKind][]catalog.ConnectionRef{
		catalog.KindEskiz:  {{ID: "platform:eskiz"}},
		catalog.KindTwilio: {{ID: "t1", Name: "main"}},
	}
	r, k, err := catalog.ResolveConnection(refs, []catalog.ConnectorKind{catalog.KindEskiz, catalog.KindTwilio}, "t1")
	require.NoError(t, err)
	assert.Equal(t, "t1", r.ID)
	assert.Equal(t, catalog.KindTwilio, k)
	// A tenant connection does not override a platform credential of another kind: the graph must choose.
	_, _, err = catalog.ResolveConnection(refs, []catalog.ConnectorKind{catalog.KindEskiz, catalog.KindTwilio}, nil)
	require.ErrorIs(t, err, catalog.ErrConnectionRequired)
	// It overrides the platform credential of its own kind.
	sameKind := map[catalog.ConnectorKind][]catalog.ConnectionRef{
		catalog.KindEskiz:  {{ID: "e1", Name: "own"}, {ID: "platform:eskiz"}},
		catalog.KindTwilio: {},
	}
	r, k, err = catalog.ResolveConnection(sameKind, []catalog.ConnectorKind{catalog.KindEskiz, catalog.KindTwilio}, nil)
	require.NoError(t, err)
	assert.Equal(t, "e1", r.ID)
	assert.Equal(t, catalog.KindEskiz, k)
	// And then only one candidate may remain.
	sameKind[catalog.KindTwilio] = []catalog.ConnectionRef{{ID: "platform:twilio"}}
	_, _, err = catalog.ResolveConnection(sameKind, []catalog.ConnectorKind{catalog.KindEskiz, catalog.KindTwilio}, nil)
	require.ErrorIs(t, err, catalog.ErrConnectionRequired)
	_, _, err = catalog.ResolveConnection(map[catalog.ConnectorKind][]catalog.ConnectionRef{
		catalog.KindEskiz: {{ID: "platform:eskiz"}}, catalog.KindTwilio: {{ID: "platform:twilio"}},
	}, []catalog.ConnectorKind{catalog.KindEskiz, catalog.KindTwilio}, nil)
	require.ErrorIs(t, err, catalog.ErrConnectionRequired)
	r, k, err = catalog.ResolveConnection(refs, []catalog.ConnectorKind{catalog.KindEskiz}, nil)
	require.NoError(t, err)
	assert.Equal(t, "platform:eskiz", r.ID)
	assert.Equal(t, catalog.KindEskiz, k)
	_, _, err = catalog.ResolveConnection(refs, []catalog.ConnectorKind{catalog.KindSMTP}, nil)
	require.ErrorIs(t, err, catalog.ErrConnectionUnavailable)
	_, _, err = catalog.ResolveConnection(refs, []catalog.ConnectorKind{catalog.KindEskiz}, 5)
	require.ErrorIs(t, err, catalog.ErrConnectionUnavailable)
}

func TestDuplicatesFailAtComposition(t *testing.T) {
	reg := catalog.NewRegistry(nil)
	require.NoError(t, reg.RegisterConnector(telegram(t)))
	require.Error(t, reg.RegisterConnector(telegram(t)), "a kind twice")
	require.Error(t, reg.RegisterNode(callNode(t, "telegram.send")), "a node type twice")
	require.Error(t, reg.RegisterTriggers(source{events: []graph.EventSpec{{Key: "telegram.message", Subject: graph.SubjectNone}}}), "an event twice")

	// A connector that fails adds nothing.
	bad := connector{kind: catalog.KindSMTP, nodes: []catalog.NodeType{callNode(t, "smtp.send"), callNode(t, "telegram.send")}}
	require.Error(t, reg.RegisterConnector(bad))
	_, ok := reg.Node("smtp.send")
	assert.False(t, ok)
	require.NoError(t, reg.RegisterConnector(connector{kind: catalog.KindSMTP, nodes: []catalog.NodeType{callNode(t, "smtp.send")}}))

	// The reserved param.
	reserved, err := graph.FieldsParams(expr.Fields{{Name: "connection", Field: expr.StringField}})
	require.NoError(t, err)
	n := callNode(t, "x.send")
	n.spec.Params = reserved
	require.Error(t, reg.RegisterNode(n))
	// A payload with kind.
	require.Error(t, reg.RegisterTriggers(source{events: []graph.EventSpec{{Key: "x.y", Subject: graph.SubjectNone, Payload: expr.Fields{{Name: "kind", Field: expr.StringField}}}}}))
}

func TestPermissionedAndLocalizedNodes(t *testing.T) {
	ctx := context.Background()
	reg := catalog.NewRegistry(nil)
	require.NoError(t, reg.RegisterNode(permissionedNode{node: callNode(t, "pay.refund"), perm: "Payments.Refund"}))
	require.NoError(t, reg.RegisterNode(localizedNode{node: callNode(t, "note.add")}))

	author := spi.Subject{Permissions: []string{"Automations.Author"}}
	v, err := reg.ForTenant(ctx, uuid.New(), catalog.AsAuthor(author), catalog.WithLocale("ru"))
	require.NoError(t, err)
	assert.Equal(t, catalog.ReasonPermission, v.Unavailable["pay.refund"])
	assert.Equal(t, "note.add@ru", v.Catalog.Nodes["note.add"].Title)

	author.Permissions = append(author.Permissions, "Payments.Refund")
	v, err = reg.ForTenant(ctx, uuid.New(), catalog.AsAuthor(author))
	require.NoError(t, err)
	assert.Contains(t, v.Catalog.Nodes, "pay.refund")
	assert.Equal(t, "note.add", v.Catalog.Nodes["note.add"].Title)

	// The system view hides nothing by permission.
	v, err = reg.ForTenant(ctx, uuid.New())
	require.NoError(t, err)
	assert.Contains(t, v.Catalog.Nodes, "pay.refund")
}

// ---- native events

type clientCreated struct {
	ClientID  int64   `json:"clientId,string" wire:"int64"`
	FirstName string  `json:"firstName"`
	Phone     *string `json:"phone,omitempty"`
	Email     *string `json:"email,omitempty"`
}

var clientCreatedEvent = native.Event[clientCreated]("crm.client.created.v1", native.EventSpec{
	Subject: "crm.client",
	Label:   manifest.Label{"en": "Client created", "ru": "Клиент создан"},
})

// metaEvent is a native event with access metadata, as *native.EventDef gets it in #1325.
type metaEvent struct {
	native.EventDefinition
	read      string
	sensitive map[string][]string
}

func (m metaEvent) Descriptions() manifest.Label {
	return manifest.Label{"en": "A client was created", "ru": "Создан клиент"}
}
func (m metaEvent) ReadPermission() string         { return m.read }
func (m metaEvent) Sensitive() map[string][]string { return m.sensitive }
func (m metaEvent) Category() string               { return "clients" }

type fakeNative []native.EventDefinition

func (f fakeNative) Events() []native.EventDefinition { return f }

func TestAuthorWithoutClientReadDoesNotSeeContacts(t *testing.T) {
	ctx := context.Background()
	ev := metaEvent{
		EventDefinition: clientCreatedEvent,
		read:            "Client.List",
		sensitive:       map[string][]string{"phone": {"Client.Read"}, "email": {"Client.Read"}},
	}
	reg := catalog.NewRegistry(nil)
	require.NoError(t, reg.RegisterTriggers(catalog.FromNativeEvents(fakeNative{ev})))
	require.NoError(t, reg.RegisterNode(callNode(t, "note.add")))

	author := spi.Subject{Permissions: []string{"Client.List"}}
	v, err := reg.ForTenant(ctx, uuid.New(), catalog.AsAuthor(author), catalog.WithLocale("ru"))
	require.NoError(t, err)
	spec, ok := v.Catalog.Events["crm.client.created.v1"]
	require.True(t, ok)
	assert.Equal(t, "Клиент создан", spec.Title)
	assert.Equal(t, "Создан клиент", spec.Description)
	assert.Equal(t, "Создан клиент", v.JSON(nil).Events[0].Description)
	assert.Equal(t, "clients", spec.Group)
	assert.Equal(t, "crm.client", spec.Subject)
	_, hasPhone := spec.Payload.Get("phone")
	_, hasEmail := spec.Payload.Get("email")
	_, hasName := spec.Payload.Get("firstName")
	assert.False(t, hasPhone)
	assert.False(t, hasEmail)
	assert.True(t, hasName)
	var schema map[string]any
	require.NoError(t, json.Unmarshal(v.JSON(nil).Events[0].Payload, &schema))
	assert.NotContains(t, schema["properties"], "phone")

	// A CEL reference to a hidden field does not type-check.
	g := graph.Graph{
		Name:     "Clients",
		Triggers: []graph.Trigger{{ID: "created", Event: "crm.client.created.v1", Filter: `event.phone != ""`}},
		Nodes:    []graph.Node{{ID: "note", Type: "note.add", Params: map[string]any{"text": "x"}}},
		Edges: []graph.Edge{
			{From: graph.EdgeFrom{Node: "created", Outcome: "fired"}, To: "note"},
			{From: graph.EdgeFrom{Node: "note", Outcome: "sent"}, To: "ignore"},
			{From: graph.EdgeFrom{Node: "note", Outcome: "failed"}, To: "ignore"},
		},
	}
	errs := graph.Validate(v.Catalog, g, graph.Options{})
	require.NotEmpty(t, errs)
	assert.Equal(t, "triggers[0].filter", errs[0].Path)

	// With Client.Read the fields are there and the filter checks.
	author.Permissions = append(author.Permissions, "Client.Read")
	v, err = reg.ForTenant(ctx, uuid.New(), catalog.AsAuthor(author))
	require.NoError(t, err)
	_, hasPhone = v.Catalog.Events["crm.client.created.v1"].Payload.Get("phone")
	assert.True(t, hasPhone)
	assert.Empty(t, graph.Validate(v.Catalog, g, graph.Options{}))

	// Without the read permission the event is not visible.
	v, err = reg.ForTenant(ctx, uuid.New(), catalog.AsAuthor(spi.Subject{}))
	require.NoError(t, err)
	assert.Equal(t, catalog.ReasonPermission, v.Unavailable["crm.client.created.v1"])
}

func TestNativeSetIsASource(t *testing.T) {
	set, err := native.Compose(func(s *native.Set) error { return s.Add(clientCreatedEvent) })
	require.NoError(t, err)
	src := catalog.FromNativeEvents(set)
	evs := src.Events()
	require.Len(t, evs, 1)
	assert.Equal(t, "crm.client.created.v1", evs[0].Key)
	assert.Equal(t, "crm", evs[0].Group)
	assert.Equal(t, "Client created", evs[0].Title)
	id, ok := evs[0].Payload.Get("clientId")
	require.True(t, ok)
	assert.Equal(t, expr.String, id.Kind)
	phone, _ := evs[0].Payload.Get("phone")
	assert.Equal(t, expr.Optional, phone.Kind)
	reg := catalog.NewRegistry(nil)
	require.NoError(t, reg.RegisterTriggers(src))
	v, err := reg.ForTenant(context.Background(), uuid.New())
	require.NoError(t, err)
	assert.Contains(t, v.Catalog.Subjects, "crm.client")
}

func TestFieldFromManifest(t *testing.T) {
	f, err := catalog.FieldFromManifest(manifest.Type{Kind: manifest.KindObject, Properties: []manifest.Property{
		{Name: "at", Type: manifest.Type{Kind: manifest.KindDateTime}},
		{Name: "n", Type: manifest.Type{Kind: manifest.KindInt, Nullable: true}},
		{Name: "tags", Type: manifest.Type{Kind: manifest.KindArray, Items: &manifest.Type{Kind: manifest.KindEnum, Values: []string{"a", "b"}}}},
		{Name: "owner", Type: manifest.Type{Kind: manifest.KindRef, Entity: "core.user"}},
		{Name: "extra", Type: manifest.Type{Kind: manifest.KindJSON, Optional: true}},
		{Name: "amount", Type: manifest.Type{Kind: manifest.KindDecimal}},
	}})
	require.NoError(t, err)
	want := expr.ObjectOf(expr.Fields{
		{Name: "at", Field: expr.TimestampField},
		{Name: "n", Field: expr.OptionalOf(expr.IntField)},
		{Name: "tags", Field: expr.ListOf(expr.EnumOf("a", "b"))},
		{Name: "owner", Field: expr.ObjectOf(expr.Fields{{Name: "entity", Field: expr.StringField}, {Name: "key", Field: expr.StringField}})},
		{Name: "extra", Field: expr.OptionalOf(expr.DynField)},
		{Name: "amount", Field: expr.StringField},
	})
	assert.Equal(t, want, f)

	_, err = catalog.FieldFromManifest(manifest.Type{Kind: "money"})
	require.Error(t, err)
	_, err = catalog.FieldFromManifest(manifest.Type{Kind: manifest.KindObject, Properties: []manifest.Property{{Name: "a-b", Type: manifest.Type{Kind: manifest.KindString}}}})
	require.Error(t, err)
}

type badDef struct{ native.EventDefinition }

func (badDef) ID() string { return "acme.bad.v1" }
func (badDef) Resource() manifest.Resource {
	return manifest.Resource{ID: "acme.bad.v1", Payload: &manifest.Type{Kind: manifest.KindObject, Properties: []manifest.Property{{Name: "x", Type: manifest.Type{Kind: "money"}}}}}
}

func TestUnsupportedNativeTypeFailsRegistration(t *testing.T) {
	reg := catalog.NewRegistry(nil)
	err := reg.RegisterTriggers(catalog.FromNativeEvents(fakeNative{badDef{}}))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "acme.bad.v1")
}

type failingAvailability struct{}

func (failingAvailability) Connected(context.Context, uuid.UUID) (map[catalog.ConnectorKind][]catalog.ConnectionRef, error) {
	return nil, errors.New("db down")
}

func TestAvailabilityErrorFailsTheView(t *testing.T) {
	_, err := catalog.NewRegistry(failingAvailability{}).ForTenant(context.Background(), uuid.New())
	require.Error(t, err)
}

func TestWaitOnAnUnavailableEventHidesTheNode(t *testing.T) {
	reg := catalog.NewRegistry(fakeAvailability{})
	require.NoError(t, reg.RegisterConnector(telegram(t)))
	p, err := graph.FieldsParams(expr.Fields{})
	require.NoError(t, err)
	require.NoError(t, reg.RegisterNode(node{spec: graph.NodeSpec{
		Type: "telegram.await", Behavior: graph.BehaviorWait, DryRun: graph.DryRunExecute, Params: p,
		Wait: &graph.WaitSpec{Event: graph.ValueOrParam{Value: "telegram.message"}, Matched: "matched"},
	}}))
	v, err := reg.ForTenant(context.Background(), uuid.New())
	require.NoError(t, err)
	assert.Equal(t, catalog.ReasonConnectionUnavailable, v.Unavailable["telegram.await"])
}

func TestCredentialsDoNotPrint(t *testing.T) {
	c := catalog.Credentials{"token": "s3cr3t"}
	for _, s := range []string{c.String(), c.GoString()} {
		assert.NotContains(t, s, "s3cr3t")
	}
}

// sensitiveSource is a trigger source with access rules.
type sensitiveSource struct {
	source
	access map[string]catalog.EventAccess
}

func (s sensitiveSource) Access(key string) (catalog.EventAccess, bool) {
	a, ok := s.access[key]
	return a, ok
}

func TestSensitiveMatchFieldsAndPaths(t *testing.T) {
	payload := expr.Fields{
		{Name: "phone", Field: expr.StringField},
		{Name: "meta", Field: expr.DynField},
		{Name: "contact", Field: expr.OptionalOf(expr.ObjectOf(expr.Fields{{Name: "email", Field: expr.StringField}, {Name: "city", Field: expr.StringField}}))},
	}
	ev := graph.EventSpec{Key: "crm.lead.v1", Subject: graph.SubjectNone, Payload: payload,
		Match: map[string]graph.MatchField{
			"phones": {Payload: "phone", Type: graph.MatchStrings},
			"emails": {Payload: "contact.email", Type: graph.MatchStrings},
			"cities": {Payload: "contact.city", Type: graph.MatchStrings},
		}}
	reg := catalog.NewRegistry(nil)
	require.NoError(t, reg.RegisterTriggers(sensitiveSource{source: source{events: []graph.EventSpec{ev}}, access: map[string]catalog.EventAccess{
		"crm.lead.v1": {Sensitive: map[string][]string{"phone": {"Lead.Read"}, "contact.email": {"Lead.Read"}}},
	}}))
	v, err := reg.ForTenant(context.Background(), uuid.New(), catalog.AsAuthor(spi.Subject{}))
	require.NoError(t, err)
	got := v.Catalog.Events["crm.lead.v1"]
	assert.Equal(t, []string{"cities"}, sortedMatch(got.Match), "a match on a hidden field would reveal it")
	contact, _ := got.Payload.Get("contact")
	_, hasEmail := contact.Prop("email")
	_, hasCity := contact.Prop("city")
	assert.False(t, hasEmail)
	assert.True(t, hasCity)

	// A sensitive path that names nothing, or a field inside dyn, fails registration.
	for _, path := range []string{"fone", "meta.phone", "contact.zip"} {
		ev.Key = "crm.lead." + strings.ReplaceAll(path, ".", "_") + ".v1"
		err := reg.RegisterTriggers(sensitiveSource{source: source{events: []graph.EventSpec{ev}}, access: map[string]catalog.EventAccess{
			ev.Key: {Sensitive: map[string][]string{path: {"Lead.Read"}}},
		}})
		require.Error(t, err, path)
	}
}

func sortedMatch(m map[string]graph.MatchField) []string {
	out := []string{}
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

type tenantOnlyEvents struct{ key string }

func (s tenantOnlyEvents) Requires() []catalog.ConnectorKind { return nil }
func (s tenantOnlyEvents) Events() []graph.EventSpec {
	return []graph.EventSpec{{Key: s.key, Title: "E", Subject: graph.SubjectNone, Payload: expr.Fields{}}}
}
func (s tenantOnlyEvents) TenantConnectionOnly(string) bool { return true }

// A tenant-only event and an ordinary node type that share a key do not share the restriction.
func TestTenantOnlyEventsAndNodesAreSeparate(t *testing.T) {
	reg := catalog.NewRegistry(fakeAvailability{"conn": {{ID: "platform:conn", Name: "platform"}}})
	node := node{spec: graph.NodeSpec{Type: "shared.key", Title: "N", Behavior: graph.BehaviorEffect, DryRun: graph.DryRunCapture, Outcomes: []string{"done"}, DoneOutcome: "done",
		Params: graph.MustSchemaParams(`{"type":"object","additionalProperties":false,"properties":{}}`)}, requires: []catalog.ConnectorKind{"conn"}}
	require.NoError(t, reg.RegisterNode(node))
	require.NoError(t, reg.RegisterTriggers(tenantOnlyEvents{key: "shared.key"}))
	v, err := reg.ForTenant(context.Background(), uuid.New())
	require.NoError(t, err)
	assert.NotContains(t, v.TenantOnlyNodes, "shared.key")
	assert.Contains(t, v.Catalog.Nodes, "shared.key", "the node is available with the platform credential")
}

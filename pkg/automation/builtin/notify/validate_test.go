package notify_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/modules/core/notifications"
	"github.com/iota-uz/iota-sdk/pkg/automation/builtin/notify"
	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/automation/engine"
	"github.com/iota-uz/iota-sdk/pkg/automation/expr"
	"github.com/iota-uz/iota-sdk/pkg/automation/graph"
)

// orderEvents is a trigger source of two events: one with a notification definition that has recipients,
// one without.
type orderEvents struct{}

func (orderEvents) Requires() []catalog.ConnectorKind { return nil }
func (orderEvents) Events() []graph.EventSpec {
	payload := expr.Fields{{Name: "amount", Field: expr.IntField}}
	return []graph.EventSpec{
		{Key: "acme.order.created.v1", Title: "Created", Group: "acme", Subject: "crm.client", Payload: payload},
		{Key: "acme.order.deleted.v1", Title: "Deleted", Group: "acme", Subject: "crm.client", Payload: payload},
	}
}

type validation struct {
	reg  catalog.Registry
	cfg  notify.Config
	view catalog.View
}

func newValidation(t *testing.T) *validation {
	t.Helper()
	cat := notifications.NewCatalog()
	require.NoError(t, cat.Register(notifications.Definition{Module: "acme", Key: "acme.order.created.v1", Name: map[string]string{"en": "Created"},
		RecipientKeys: []notifications.RecipientDefinition{{Key: "owner", Name: map[string]string{"en": "Owner"},
			Resolve: func(context.Context, notifications.Event) ([]uint, error) { return nil, nil }}}}))
	require.NoError(t, cat.Register(notifications.Definition{Module: "acme", Key: "acme.order.deleted.v1", Name: map[string]string{"en": "Deleted"}}))
	v := &validation{reg: catalog.NewRegistry(nil, catalog.WithParticipants(func(event string) []string {
		def, ok := cat.Get(event)
		if !ok {
			return nil
		}
		var keys []string
		for _, r := range def.RecipientKeys {
			keys = append(keys, r.Key)
		}
		return keys
	})), cfg: notify.Config{Sender: &sender{}, Catalog: cat, Pool: new(pgxpool.Pool), BaseURL: "https://erp.example.com"}}
	require.NoError(t, v.reg.RegisterTriggers(engine.Triggers()))
	require.NoError(t, v.reg.RegisterTriggers(orderEvents{}))
	for _, spec := range graph.BuiltinNodes(nil, nil) {
		require.NoError(t, v.reg.RegisterNode(specNode{spec}))
	}
	require.NoError(t, notify.Register(v.reg, v.cfg))
	view, err := v.reg.ForTenant(context.Background(), tenant)
	require.NoError(t, err)
	v.view = view
	return v
}

type specNode struct{ spec graph.NodeSpec }

func (n specNode) Spec() graph.NodeSpec              { return n.spec }
func (n specNode) Requires() []catalog.ConnectorKind { return nil }

// errors validates a graph as the engine does: the format, then the rules of the node types.
func (v *validation) errors(t *testing.T, raw string) []graph.GraphError {
	t.Helper()
	g, errs := graph.Decode([]byte(raw))
	require.Empty(t, errs)
	return graph.Validate(v.view.Catalog, g, graph.Options{Rules: func(g graph.Graph) []graph.GraphError {
		var out []graph.GraphError
		for i, n := range g.Nodes {
			nt, _ := v.reg.Node(n.Type)
			if gc, ok := nt.(engine.GraphChecker); ok {
				out = append(out, gc.CheckNode(g, i, v.view)...)
			}
		}
		return out
	}})
}

func notifyGraph(trigger, params string) string {
	return fmt.Sprintf(`{"name": "Notify", "triggers": [{"id": "t", "event": %q}],
	 "nodes": [{"id": "ping", "type": "notify", "params": %s},
	  {"id": "none", "type": "end", "params": {}}, {"id": "fin", "type": "end", "params": {}}],
	 "edges": [{"from": {"node": "t", "outcome": "fired"}, "to": "ping"},
	  {"from": {"node": "ping", "outcome": "sent"}, "to": "fin"}, {"from": {"node": "ping", "outcome": "no_recipients"}, "to": "none"},
	  {"from": {"node": "ping", "outcome": "failed"}, "to": "fin"}]}`, trigger, params)
}

func TestTheNodesAreValidInAGraph(t *testing.T) {
	v := newValidation(t)
	for name, params := range map[string]string{
		"users by expression": `{"recipients": {"users": "[client.id, 7]"}, "title": "Hi {{ event.amount }}", "body": "Paid", "channels": ["email"], "level": "warning"}`,
		"groups and roles":    `{"recipients": {"groups": ["7a8c0e5e-0000-4000-8000-000000000001"], "roles": ["12"]}, "title": "Hi", "body": "Paid", "action_url": "/orders/{{ subject.key }}"}`,
		"participants":        `{"recipients": {"participants": ["owner"]}, "title": "Hi", "body": "Paid"}`,
		"a link by template":  `{"recipients": {"roles": ["1"]}, "title": "Hi", "body": "Paid", "action_url": "{{ subject.url }}"}`,
		"a link on our host":  `{"recipients": {"roles": ["1"]}, "title": "Hi", "body": "Paid", "action_url": "https://erp.example.com/orders/{{ subject.key }}"}`,
	} {
		t.Run(name, func(t *testing.T) {
			raw := strings.ReplaceAll(notifyGraph("acme.order.created.v1", params), "client.id", "event.amount")
			assert.Empty(t, v.errors(t, raw))
		})
	}
	contact := `{"name": "Contact", "triggers": [{"id": "t", "event": "acme.order.created.v1"}],
	 "nodes": [{"id": "sms", "type": "send_to_contact", "params": {"channel": "sms", "body": "Ready {{ event.amount }}"}},
	  {"id": "fin", "type": "end", "params": {}}],
	 "edges": [{"from": {"node": "t", "outcome": "fired"}, "to": "sms"},
	  {"from": {"node": "sms", "outcome": "sent"}, "to": "fin"}, {"from": {"node": "sms", "outcome": "no_contact"}, "to": "fin"},
	  {"from": {"node": "sms", "outcome": "failed"}, "to": "fin"}]}`
	assert.Empty(t, v.errors(t, contact))
	record := strings.Replace(contact, `"channel": "sms"`, `"record": "{'entity': 'crm.client', 'key': '7'}", "channel": "sms"`, 1)
	assert.Empty(t, v.errors(t, record))
}

func TestParticipantsNeedAnEventWithRecipients(t *testing.T) {
	v := newValidation(t)
	params := `{"recipients": {"participants": ["owner"]}, "title": "Hi", "body": "Paid"}`

	errs := v.errors(t, notifyGraph("acme.order.deleted.v1", params))
	require.Len(t, errs, 1, "an event whose definition has no recipients offers no participants")
	assert.Equal(t, "nodes[0].params.recipients.participants", errs[0].Path)
	assert.Equal(t, graph.CodeParams, errs[0].Code)

	errs = v.errors(t, notifyGraph(engine.EventManual, params))
	require.Len(t, errs, 1, "a manual run has no event")
	assert.Equal(t, "nodes[0].params.recipients.participants", errs[0].Path)

	errs = v.errors(t, notifyGraph("acme.order.created.v1", `{"recipients": {"participants": ["boss"]}, "title": "Hi", "body": "Paid"}`))
	require.Len(t, errs, 1, "a recipient the definition does not have")
	assert.Contains(t, errs[0].Message, `"boss"`)

	// Every trigger that reaches the node must offer them.
	two := `{"name": "Two", "triggers": [{"id": "a", "event": "acme.order.created.v1"}, {"id": "b", "event": "acme.order.deleted.v1"}],
	 "nodes": [{"id": "ping", "type": "notify", "params": ` + params + `}, {"id": "fin", "type": "end", "params": {}}],
	 "edges": [{"from": {"node": "a", "outcome": "fired"}, "to": "ping"}, {"from": {"node": "b", "outcome": "fired"}, "to": "ping"},
	  {"from": {"node": "ping", "outcome": "sent"}, "to": "fin"}, {"from": {"node": "ping", "outcome": "no_recipients"}, "to": "fin"},
	  {"from": {"node": "ping", "outcome": "failed"}, "to": "fin"}]}`
	errs = v.errors(t, two)
	require.Len(t, errs, 1)
	assert.Contains(t, errs[0].Message, "trigger b")

	// A trigger that cannot reach the node does not matter.
	apart := strings.Replace(two, `{"from": {"node": "b", "outcome": "fired"}, "to": "ping"}`, `{"from": {"node": "b", "outcome": "fired"}, "to": "fin"}`, 1)
	assert.Empty(t, v.errors(t, apart))

	// Without a notification catalog no event offers participants.
	bare := newValidation(t)
	bare.cfg.Catalog = nil
	reg := catalog.NewRegistry(nil)
	require.NoError(t, reg.RegisterTriggers(engine.Triggers()))
	require.NoError(t, reg.RegisterTriggers(orderEvents{}))
	for _, spec := range graph.BuiltinNodes(nil, nil) {
		require.NoError(t, reg.RegisterNode(specNode{spec}))
	}
	require.NoError(t, notify.Register(reg, bare.cfg))
	bare.reg = reg
	view, err := reg.ForTenant(context.Background(), tenant)
	require.NoError(t, err)
	bare.view = view
	assert.Len(t, bare.errors(t, notifyGraph("acme.order.created.v1", params)), 1)
}

func TestAnExternalLinkIsRefusedWhenTheGraphIsSaved(t *testing.T) {
	v := newValidation(t)
	for link, refused := range map[string]bool{
		"/orders/1":                              false,
		"/orders/{{ event.amount }}":             false,
		"{{ subject.url }}":                      false,
		"https://erp.example.com/orders/1":       false,
		"https://erp.example.com":                false,
		"https://evil.example.com/orders/1":      true,
		"https://evil.example.com/{{ event.x }}": true,
		"https://{{ event.amount }}/orders":      true,
		"https://erp.example.com{{ event.x }}":   true, // the template could extend the host
		"//evil.example.com":                     true,
		"orders/1":                               true,
		"javascript:alert(1)":                    true,
		"https://user@erp.example.com/x":         true,
		"https://erp.example.com//evil.com":      true, // the path is protocol-relative
		"/%2F%2Fevil.com":                        true, // and so is this one, once decoded
		"/orders/%2e%2e/%5cevil":                 true,
		"/orders/%41":                            false,
		// every literal part after a template is checked, not only the text before the first one
		"/orders/{{ event.amount }}/%5Cbad":                  true,
		"/orders/{{ event.amount }}/%5cbad":                  true,
		"/orders/{{ event.amount }}\\bad":                    true,
		"/orders/{{ event.amount }}/a/{{ event.amount }}%5C": true,
		"/orders/{{ event.amount }}/ok/{{ event.amount }}":   false,
	} {
		raw := notifyGraph("acme.order.created.v1", fmt.Sprintf(`{"recipients": {"roles": ["1"]}, "title": "Hi", "body": "Paid", "action_url": %q}`, link))
		errs := v.errors(t, strings.ReplaceAll(raw, "event.x", "event.amount"))
		if !refused {
			assert.Empty(t, errs, link)
			continue
		}
		var at []string
		for _, e := range errs {
			at = append(at, e.Path)
		}
		assert.Equal(t, []string{"nodes[0].params.action_url"}, at, link)
	}
}

func TestUnknownChannelsAreRefusedWhenTheGraphIsSaved(t *testing.T) {
	v := newValidation(t)
	errs := v.errors(t, notifyGraph("acme.order.created.v1", `{"recipients": {"roles": ["1"]}, "title": "Hi", "body": "Paid", "channels": ["email", "fax"]}`))
	require.Len(t, errs, 1)
	assert.Equal(t, "nodes[0].params.channels[1]", errs[0].Path)

	contact := `{"name": "Contact", "triggers": [{"id": "t", "event": "acme.order.created.v1"}],
	 "nodes": [{"id": "sms", "type": "send_to_contact", "params": {"channel": "in_app", "body": "Ready"}}, {"id": "fin", "type": "end", "params": {}}],
	 "edges": [{"from": {"node": "t", "outcome": "fired"}, "to": "sms"},
	  {"from": {"node": "sms", "outcome": "sent"}, "to": "fin"}, {"from": {"node": "sms", "outcome": "no_contact"}, "to": "fin"},
	  {"from": {"node": "sms", "outcome": "failed"}, "to": "fin"}]}`
	errs = v.errors(t, contact)
	require.Len(t, errs, 1, "a record is not reached in the inbox")
	assert.Equal(t, "nodes[0].params.channel", errs[0].Path)
}

func TestTheNodeParamsAreChecked(t *testing.T) {
	v := newValidation(t)
	for name, params := range map[string]string{
		"no audience":     `{"recipients": {}, "title": "Hi", "body": "Paid"}`,
		"no title":        `{"recipients": {"roles": ["1"]}, "body": "Paid"}`,
		"a level of ours": `{"recipients": {"roles": ["1"]}, "title": "Hi", "body": "Paid", "level": "error"}`,
		"a bad group":     `{"recipients": {"groups": ["nope"]}, "title": "Hi", "body": "Paid"}`,
		"a bad role":      `{"recipients": {"roles": ["0"]}, "title": "Hi", "body": "Paid"}`,
		"a stray param":   `{"recipients": {"roles": ["1"]}, "title": "Hi", "body": "Paid", "sound": "loud"}`,
	} {
		t.Run(name, func(t *testing.T) {
			assert.NotEmpty(t, v.errors(t, notifyGraph("acme.order.created.v1", params)))
		})
	}
}

func TestTheNodeOutcomesAndOutputAreDeclared(t *testing.T) {
	v := newValidation(t)
	n := v.view.Catalog.Nodes[notify.NodeNotify]
	assert.Equal(t, graph.BehaviorEffect, n.Behavior)
	assert.Equal(t, []string{"sent", "no_recipients", "failed"}, n.Outcomes)
	_, hasNotice := n.Output.Get("notice_id")
	_, hasRecipients := n.Output.Get("recipients")
	_, hasSkipped := n.Output.Get("skipped")
	assert.True(t, hasNotice && hasRecipients && hasSkipped)
	c := v.view.Catalog.Nodes[notify.NodeSendToContact]
	assert.Equal(t, graph.BehaviorEffect, c.Behavior)
	assert.Equal(t, []string{"sent", "no_contact", "failed"}, c.Outcomes)
	_, hasChannel := c.Output.Get("channel")
	assert.True(t, hasChannel)
	assert.Equal(t, graph.DryRunCapture, n.DryRun)

	// A later node reads the output.
	g := strings.Replace(notifyGraph("acme.order.created.v1", `{"recipients": {"roles": ["1"]}, "title": "Hi", "body": "Paid"}`),
		`{"id": "fin", "type": "end", "params": {}}`, `{"id": "fin", "type": "end", "params": {"reason": "sent to {{ nodes.ping.output.recipients }} as {{ nodes.ping.output.notice_id }}"}}`, 1)
	assert.Empty(t, v.errors(t, g))
}

func TestRegisterRefusesAConfigTheNodesCannotWorkWith(t *testing.T) {
	reg := func() catalog.Registry { return catalog.NewRegistry(nil) }
	pool := new(pgxpool.Pool)
	require.ErrorContains(t, notify.Register(reg(), notify.Config{Pool: pool}), "Sender")
	require.ErrorContains(t, notify.Register(reg(), notify.Config{Sender: &sender{}}), "Pool")
	require.ErrorContains(t, notify.Register(reg(), notify.Config{Sender: &sender{}, Pool: pool, BaseURL: "erp.example.com"}), "BaseURL")
	require.ErrorContains(t, notify.Register(reg(), notify.Config{Sender: &sender{}, Pool: pool, BaseURL: "ftp://erp.example.com"}), "BaseURL")
	r := reg()
	require.NoError(t, notify.Register(r, notify.Config{Sender: &sender{}, Pool: pool}))
	assert.Error(t, notify.Register(r, notify.Config{Sender: &sender{}, Pool: pool}), "the nodes are registered once")
}

// The editor learns which events offer participants from the catalog view.
func TestTheCatalogViewNamesTheParticipantsOfAnEvent(t *testing.T) {
	v := newValidation(t)
	byKey := map[string][]string{}
	for _, e := range v.view.JSON(graph.DefaultBehaviors()).Events {
		byKey[e.Key] = e.Participants
	}
	assert.Equal(t, []string{"owner"}, byKey["acme.order.created.v1"])
	assert.Empty(t, byKey["acme.order.deleted.v1"])
	assert.Empty(t, byKey[engine.EventManual])
	var schema map[string]any
	for _, n := range v.view.JSON(graph.DefaultBehaviors()).Nodes {
		if n.Type == notify.NodeNotify {
			require.NoError(t, json.Unmarshal(n.Params, &schema))
		}
	}
	participants := schema["properties"].(map[string]any)["recipients"].(map[string]any)["properties"].(map[string]any)["participants"].(map[string]any)
	assert.Equal(t, true, participants["x-event-participants"], "the form shows the field only where the trigger events offer participants")
}

package graph_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/automation/expr"
	"github.com/iota-uz/iota-sdk/pkg/automation/graph"
)

// recordMailCatalog has a record event (about a "record" with its own variables) and an incoming mail (about
// no subject), and a node that takes only records.
func recordMailCatalog(t *testing.T, mixed string) graph.Catalog {
	t.Helper()
	c := testCatalog(t)
	c.Subjects["mail"] = graph.SubjectSpec{}
	c.Events["mail.received"] = graph.EventSpec{Key: "mail.received", Subject: graph.SubjectNone, Payload: expr.Fields{
		{Name: "mail", Field: expr.ObjectOf(expr.Fields{{Name: "subject", Field: expr.StringField}, {Name: "labels", Field: expr.ListOf(expr.StringField)}})},
	}}
	c.Events["order.created"] = graph.EventSpec{Key: "order.created", Subject: "order", Payload: expr.Fields{{Name: "total", Field: expr.DoubleField}}}
	c.Nodes["review"] = graph.NodeSpec{
		Type: "review", Behavior: graph.BehaviorEffect, Subjects: []string{"order"},
		Params:      graph.MustSchemaParams(`{"type": "object", "properties": {}, "additionalProperties": false}`),
		Outcomes:    []string{"done"},
		DoneOutcome: "done",
	}
	c.MixedSubject = mixed
	if err := c.Check(nil); err != nil {
		t.Fatal(err)
	}
	return c
}

const recordMailGraph = `{"name": "Record and mail",
  "triggers": [{"id": "created", "event": "order.created"}, {"id": "incoming", "event": "mail.received"}],
  "nodes": [
    {"id": "check", "type": "branch", "params": {"expression": "order.status == \"new\" && event.total > 1.0"}},
    {"id": "review", "type": "review", "params": {}},
    {"id": "urgent", "type": "branch", "params": {"expression": "event.mail.subject.startsWith(\"URGENT\")"}},
    {"id": "tell", "type": "notify", "params": {"text": "{{ join(event.mail.labels, \", \") }}"}},
    {"id": "both", "type": "branch", "params": {"expression": "event.kind != \"\""}}
  ],
  "edges": [
    {"from": {"node": "created", "outcome": "fired"}, "to": "check"},
    {"from": {"node": "check", "outcome": "true"}, "to": "review"},
    {"from": {"node": "check", "outcome": "false"}, "to": "both"},
    {"from": {"node": "review", "outcome": "done"}, "to": "ignore"},
    {"from": {"node": "incoming", "outcome": "fired"}, "to": "urgent"},
    {"from": {"node": "urgent", "outcome": "true"}, "to": "tell"},
    {"from": {"node": "urgent", "outcome": "false"}, "to": "both"},
    {"from": {"node": "tell", "outcome": "done"}, "to": "both"},
    {"from": {"node": "tell", "outcome": "failed"}, "to": "ignore"},
    {"from": {"node": "both", "outcome": "true"}, "to": "ignore"},
    {"from": {"node": "both", "outcome": "false"}, "to": "ignore"}
  ]}`

// A graph with a record trigger and a mail trigger is valid, with a mixed subject or without one: the record
// branch reads the record's variables and takes the node for records, the mail branch reads event.mail.
func TestReachableTriggersValid(t *testing.T) {
	for _, mixed := range []string{"", "order"} {
		c := recordMailCatalog(t, mixed)
		g := mustDecode(t, recordMailGraph)
		if errs := graph.Validate(c, g, graph.Options{}); len(errs) > 0 {
			t.Errorf("mixed %q: %v", mixed, errs)
		}
	}
}

func TestReachableTriggersUnknownVariable(t *testing.T) {
	c := recordMailCatalog(t, "")
	bad := strings.NewReplacer(
		`event.mail.subject.startsWith(\"URGENT\")`, `order.status == \"new\"`,
		`{{ join(event.mail.labels, \", \") }}`, `{{ order.total }}`,
		`event.kind != \"\"`, `order.total > 1.0`,
		`order.status == \"new\" && event.total > 1.0`, `event.mail.subject == \"x\"`,
	).Replace(recordMailGraph)
	errs := graph.Validate(c, mustDecode(t, bad), graph.Options{})
	want := map[string]string{
		"nodes[0].params.expression": "node check reads event.mail, which trigger created (on order.created) does not have",
		"nodes[2].params.expression": "node urgent reads order, which trigger incoming (on mail.received) does not have",
		"nodes[3].params.text":       "node tell reads order, which trigger incoming (on mail.received) does not have",
		"nodes[4].params.expression": "node both reads order, which trigger incoming (on mail.received) does not have",
	}
	got := map[string]string{}
	for _, e := range errs {
		if e.Code != graph.CodeUnknownVariableForTrigger {
			t.Errorf("%s: code %s: %s", e.Path, e.Code, e.Message)
		}
		got[e.Path] = e.Message
	}
	for path, msg := range want {
		if got[path] != msg {
			t.Errorf("%s:\n got %q\nwant %q", path, got[path], msg)
		}
	}
	if len(got) != len(want) {
		t.Errorf("errors: %v", errs)
	}
}

// A node that takes only records is refused where a mail trigger leads to it.
func TestReachableTriggersNodeSubjects(t *testing.T) {
	c := recordMailCatalog(t, "")
	g := mustDecode(t, strings.Replace(recordMailGraph, `{"from": {"node": "urgent", "outcome": "false"}, "to": "both"}`, `{"from": {"node": "urgent", "outcome": "false"}, "to": "review"}`, 1))
	errs := graph.Validate(c, g, graph.Options{})
	if len(errs) != 1 || errs[0].Path != "nodes[1].type" || errs[0].Code != graph.CodeInvalid || !strings.Contains(errs[0].Message, "incoming") {
		t.Fatalf("errors: %v", errs)
	}
	// With a mixed subject a node both kinds lead to is about the mixed subject.
	c = recordMailCatalog(t, "mail")
	errs = graph.Validate(c, g, graph.Options{})
	if len(errs) != 1 || errs[0].Path != "nodes[1].type" {
		t.Fatalf("mixed: %v", errs)
	}
}

// A graph about one subject keeps the error of a variable no trigger has: expression_type, not a variable of
// another trigger.
func TestReachableTriggersTypo(t *testing.T) {
	c := recordMailCatalog(t, "")
	g := mustDecode(t, strings.Replace(recordMailGraph, `event.mail.subject.startsWith(\"URGENT\")`, `event.mail.subjekt == \"\"`, 1))
	errs := graph.Validate(c, g, graph.Options{})
	if len(errs) != 1 || errs[0].Code != graph.CodeExpressionType {
		t.Fatalf("errors: %v", errs)
	}
}

// The interpreter runs each branch by the scope of the triggers that lead to it.
func TestReachableTriggersRun(t *testing.T) {
	c := recordMailCatalog(t, "")
	in, err := graph.NewInterpreter(c, mustDecode(t, recordMailGraph), graph.Options{})
	if err != nil {
		t.Fatal(err)
	}
	ctx := graph.Context{Vars: map[string]any{"order": map[string]any{"total": 5.0, "status": "new"}}}
	trace := in.DryRun(graph.Subject{Entity: "order", Key: "1"}, ctx, []graph.DryRunInput{
		{Event: graph.Event{"kind": "order.created", "total": 5.0}},
	}, graph.DryRunOptions{})
	if len(trace) < 3 || trace[1].Node != "check" || trace[1].Outcome != "true" || trace[2].Node != "review" {
		t.Fatalf("record branch: %+v", trace)
	}
	trace = in.DryRun(graph.Subject{}, graph.Context{}, []graph.DryRunInput{
		{Event: graph.Event{"kind": "mail.received", "mail": map[string]any{"subject": "URGENT x", "labels": []any{"a", "b"}}}},
	}, graph.DryRunOptions{})
	if len(trace) < 3 || trace[1].Node != "urgent" || trace[1].Outcome != "true" || trace[2].Node != "tell" {
		t.Fatalf("mail branch: %+v", trace)
	}
	if got := trace[2].ContextPatch["effects"]; !strings.Contains(compactJSON(t, got), `"text":"a, b"`) {
		t.Fatalf("the join of the labels: %+v", trace[2])
	}
}

func compactJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// A mixed subject does not stand for the subjects of the triggers: a node both a record and a mail lead to
// reads what both have, and a node for records is refused after the mail.
func TestReachableTriggersMixedSubject(t *testing.T) {
	c := recordMailCatalog(t, "order")
	g := mustDecode(t, strings.Replace(recordMailGraph, `event.kind != \"\"`, `order.status == \"new\"`, 1))
	errs := graph.Validate(c, g, graph.Options{})
	if len(errs) != 1 || errs[0].Code != graph.CodeUnknownVariableForTrigger || errs[0].Path != "nodes[4].params.expression" || !strings.Contains(errs[0].Message, "incoming") {
		t.Fatalf("a shared node reads the record: %v", errs)
	}
	g = mustDecode(t, strings.Replace(recordMailGraph, `{"from": {"node": "urgent", "outcome": "false"}, "to": "both"}`, `{"from": {"node": "urgent", "outcome": "false"}, "to": "review"}`, 1))
	errs = graph.Validate(c, g, graph.Options{})
	if len(errs) != 1 || errs[0].Path != "nodes[1].type" {
		t.Fatalf("a node for records after the mail: %v", errs)
	}
}

// Two events that declare the same object with different fields share the fields both have.
func TestReachableTriggersNestedFields(t *testing.T) {
	c := recordMailCatalog(t, "")
	c.Events["mail.sent"] = graph.EventSpec{Key: "mail.sent", Subject: graph.SubjectNone, Payload: expr.Fields{
		{Name: "mail", Field: expr.ObjectOf(expr.Fields{{Name: "to", Field: expr.StringField}})},
	}}
	c.Events["mail.received"] = graph.EventSpec{Key: "mail.received", Subject: graph.SubjectNone, Payload: expr.Fields{
		{Name: "mail", Field: expr.ObjectOf(expr.Fields{{Name: "to", Field: expr.StringField}, {Name: "subject", Field: expr.StringField}})},
	}}
	graphOf := func(expression string) graph.Graph {
		return mustDecode(t, `{"name": "Mail", "triggers": [{"id": "inbox", "event": "mail.received"}, {"id": "outbox", "event": "mail.sent"}],
		"nodes": [{"id": "n", "type": "branch", "params": {"expression": "`+expression+`"}}],
		"edges": [{"from": {"node": "inbox", "outcome": "fired"}, "to": "n"}, {"from": {"node": "outbox", "outcome": "fired"}, "to": "n"},
		{"from": {"node": "n", "outcome": "true"}, "to": "ignore"}, {"from": {"node": "n", "outcome": "false"}, "to": "ignore"}]}`)
	}
	if errs := graph.Validate(c, graphOf(`event.mail.to != \"\"`), graph.Options{}); len(errs) > 0 {
		t.Fatalf("a field both have: %v", errs)
	}
	errs := graph.Validate(c, graphOf(`event.mail.subject != \"\"`), graph.Options{})
	if len(errs) != 1 || errs[0].Code != graph.CodeUnknownVariableForTrigger || errs[0].Message != "node n reads event.mail.subject, which trigger outbox (on mail.sent) does not have" {
		t.Fatalf("a field one has: %v", errs)
	}
}

// What the check allows at a shared node does not cut the event a run passes: it is whole.
func TestReachableTriggersRunKeepsEvent(t *testing.T) {
	c := recordMailCatalog(t, "")
	c.Events["mail.sent"] = graph.EventSpec{Key: "mail.sent", Subject: graph.SubjectNone, Payload: expr.Fields{
		{Name: "mail", Field: expr.ObjectOf(expr.Fields{{Name: "subject", Field: expr.StringField}, {Name: "to", Field: expr.StringField}})},
	}}
	g := mustDecode(t, `{"name": "Mail", "triggers": [{"id": "inbox", "event": "mail.received"}, {"id": "outbox", "event": "mail.sent"}],
		"nodes": [{"id": "n", "type": "notify", "params": {"text": "{{ event }}"}}],
		"edges": [{"from": {"node": "inbox", "outcome": "fired"}, "to": "n"}, {"from": {"node": "outbox", "outcome": "fired"}, "to": "n"},
		{"from": {"node": "n", "outcome": "done"}, "to": "ignore"}, {"from": {"node": "n", "outcome": "failed"}, "to": "ignore"}]}`)
	in, err := graph.NewInterpreter(c, g, graph.Options{})
	if err != nil {
		t.Fatal(err)
	}
	trace := in.DryRun(graph.Subject{}, graph.Context{}, []graph.DryRunInput{
		{Event: graph.Event{"kind": "mail.received", "mail": map[string]any{"subject": "s", "labels": []any{"a"}}}},
	}, graph.DryRunOptions{})
	if len(trace) < 2 || !strings.Contains(compactJSON(t, trace[1].ContextPatch), `labels`) {
		t.Fatalf("the event at the shared node: %+v", trace)
	}
}

// A run about a record starts on the record's trigger only, whether or not the catalog has a mixed subject.
func TestReachableTriggersDryRunSubject(t *testing.T) {
	for _, mixed := range []string{"", "order"} {
		c := recordMailCatalog(t, mixed)
		in, err := graph.NewInterpreter(c, mustDecode(t, recordMailGraph), graph.Options{})
		if err != nil {
			t.Fatal(err)
		}
		trace := in.DryRun(graph.Subject{Entity: "order", Key: "1"}, graph.Context{Vars: map[string]any{"order": map[string]any{"total": 1.0, "status": "new"}}}, []graph.DryRunInput{
			{Event: graph.Event{"kind": "mail.received", "mail": map[string]any{"subject": "URGENT", "labels": []any{}}}},
		}, graph.DryRunOptions{})
		for _, step := range trace {
			if step.Node == "incoming" {
				t.Errorf("mixed %q: a run about an order started on the mail trigger: %+v", mixed, trace)
			}
		}
	}
}

// A graph with one trigger reports a variable the catalog has for another subject by the trigger.
func TestReachableTriggersOneTrigger(t *testing.T) {
	c := recordMailCatalog(t, "")
	g := mustDecode(t, `{"name": "Mail", "triggers": [{"id": "incoming", "event": "mail.received"}],
		"nodes": [{"id": "n", "type": "branch", "params": {"expression": "order.status == \"new\""}}],
		"edges": [{"from": {"node": "incoming", "outcome": "fired"}, "to": "n"},
		{"from": {"node": "n", "outcome": "true"}, "to": "ignore"}, {"from": {"node": "n", "outcome": "false"}, "to": "ignore"}]}`)
	errs := graph.Validate(c, g, graph.Options{})
	if len(errs) != 1 || errs[0].Code != graph.CodeUnknownVariableForTrigger || errs[0].Message != "node n reads order, which trigger incoming (on mail.received) does not have" {
		t.Fatalf("errors: %v", errs)
	}
}

// twoMailGraph has two mail triggers (inbox, outbox) and one node both lead to.
func twoMailGraph(t *testing.T, expression string) graph.Graph {
	t.Helper()
	return mustDecode(t, `{"name": "Mail", "triggers": [{"id": "inbox", "event": "mail.received"}, {"id": "outbox", "event": "mail.sent"}],
		"nodes": [{"id": "n", "type": "branch", "params": {"expression": "`+expression+`"}}],
		"edges": [{"from": {"node": "inbox", "outcome": "fired"}, "to": "n"}, {"from": {"node": "outbox", "outcome": "fired"}, "to": "n"},
		{"from": {"node": "n", "outcome": "true"}, "to": "ignore"}, {"from": {"node": "n", "outcome": "false"}, "to": "ignore"}]}`)
}

func mailEvents(t *testing.T, received, sent expr.Fields) graph.Catalog {
	t.Helper()
	c := recordMailCatalog(t, "")
	c.Events["mail.received"] = graph.EventSpec{Key: "mail.received", Subject: graph.SubjectNone, Payload: received}
	c.Events["mail.sent"] = graph.EventSpec{Key: "mail.sent", Subject: graph.SubjectNone, Payload: sent}
	return c
}

// A shared node runs with the types the check gave: a double read from a JSON integer, an absent optional, a
// timestamp from its string.
func TestReachableTriggersRunKeepsTypes(t *testing.T) {
	payload := func(extra string) expr.Fields {
		return expr.Fields{
			{Name: "data", Field: expr.DoubleField}, {Name: "note", Field: expr.OptionalOf(expr.StringField)},
			{Name: "at", Field: expr.TimestampField}, {Name: extra, Field: expr.StringField},
		}
	}
	c := mailEvents(t, payload("from"), payload("to"))
	g := twoMailGraph(t, `event.data / 2.0 == 2.0 && event.note == \"\" && event.at < now`)
	in, err := graph.NewInterpreter(c, g, graph.Options{})
	if err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"mail.received", "mail.sent"} {
		trace := in.DryRun(graph.Subject{}, graph.Context{Now: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}, []graph.DryRunInput{
			{Event: graph.Event{"kind": kind, "data": 4, "at": "2020-01-01T00:00:00Z", "from": "a", "to": "b"}},
		}, graph.DryRunOptions{})
		if len(trace) < 2 || trace[1].Node != "n" || trace[1].Outcome != "true" {
			t.Fatalf("%s: %+v", kind, trace)
		}
	}
}

// Fields declared as an object by one event and as something else by the other are not shared.
func TestReachableTriggersIncompatibleFields(t *testing.T) {
	c := mailEvents(t,
		expr.Fields{{Name: "data", Field: expr.ObjectOf(expr.Fields{{Name: "subject", Field: expr.StringField}})}},
		expr.Fields{{Name: "data", Field: expr.StringField}})
	errs := graph.Validate(c, twoMailGraph(t, `event.data.subject == \"x\"`), graph.Options{})
	if len(errs) != 1 || errs[0].Code != graph.CodeUnknownVariableForTrigger || errs[0].Message != "node n reads event.data.subject, which trigger outbox (on mail.sent) does not have" {
		t.Fatalf("errors: %v", errs)
	}
}

// A field of a list element is checked by each trigger's element schema.
func TestReachableTriggersListElementField(t *testing.T) {
	list := func(name string) expr.Field {
		return expr.ListOf(expr.ObjectOf(expr.Fields{{Name: name, Field: expr.StringField}}))
	}
	c := mailEvents(t, expr.Fields{{Name: "data", Field: list("url")}}, expr.Fields{{Name: "data", Field: list("title")}})
	errs := graph.Validate(c, twoMailGraph(t, `event.data[0].url == \"x\"`), graph.Options{})
	if len(errs) != 1 || errs[0].Code != graph.CodeUnknownVariableForTrigger || errs[0].Message != "node n reads event.data[].url, which trigger outbox (on mail.sent) does not have" {
		t.Fatalf("errors: %v", errs)
	}
}

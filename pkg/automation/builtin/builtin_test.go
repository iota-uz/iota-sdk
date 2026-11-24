package builtin_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"

	"github.com/iota-uz/iota-sdk/pkg/automation/builtin"
	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/automation/engine"
	"github.com/iota-uz/iota-sdk/pkg/automation/graph"
)

func TestRegisterBuildsACheckedCatalog(t *testing.T) {
	reg := catalog.NewRegistry(nil)
	if err := reg.RegisterTriggers(engine.Triggers()); err != nil {
		t.Fatal(err)
	}
	if err := builtin.Register(reg, builtin.Deps{}); err != nil {
		t.Fatalf("Register: %v", err)
	}
	v, err := reg.ForTenant(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("ForTenant: %v", err)
	}
	for _, typ := range []string{"set", "code", "http", "start_automation", "run_automation"} {
		if _, ok := v.Catalog.Nodes[typ]; !ok {
			t.Errorf("%s is not in the catalog (unavailable: %v)", typ, v.Unavailable)
		}
	}
	if err := builtin.Register(reg, builtin.Deps{}); err == nil {
		t.Fatal("a second registration was accepted")
	}
}

// An HTTP body template is checked at save time and rendered in the step.
func TestHTTPBodyTemplateThroughAGraph(t *testing.T) {
	reg := catalog.NewRegistry(nil)
	if err := reg.RegisterTriggers(engine.Triggers()); err != nil {
		t.Fatal(err)
	}
	if err := builtin.Register(reg, builtin.Deps{}); err != nil {
		t.Fatal(err)
	}
	v, err := reg.ForTenant(context.Background(), uuid.New())
	if err != nil {
		t.Fatal(err)
	}
	g, errs := graph.Decode([]byte(`{"name": "Hello", "triggers": [{"id": "m", "event": "automation.manual.v1"}],
	 "nodes": [{"id": "say", "type": "http", "params": {"method": "POST", "url": "https://api.example.com/hello",
	   "body": {"kind": "text", "template": "Hello, {{ event.input.name }}"}}}],
	 "edges": [{"from": {"node": "m", "outcome": "fired"}, "to": "say"},
	  {"from": {"node": "say", "outcome": "success"}, "to": "ignore"}, {"from": {"node": "say", "outcome": "failed"}, "to": "ignore"}]}`))
	if len(errs) > 0 {
		t.Fatal(errs)
	}
	if errs := graph.Validate(v.Catalog, g, graph.Options{}); len(errs) > 0 {
		t.Fatalf("validate: %v", errs)
	}
	in, err := graph.NewInterpreter(v.Catalog, g, graph.Options{})
	if err != nil {
		t.Fatal(err)
	}
	ev := graph.Event{"kind": "automation.manual.v1", "input": map[string]any{"name": "Aziz"}, "initiator_user_id": 1}
	st := in.Start("say", &graph.StepContext{Ctx: graph.Context{Event: ev}})
	body, _ := st.Effects[0].Params["body"].(map[string]any)
	if st.Kind != graph.StepCall || body["template"] != "Hello, Aziz" {
		t.Fatalf("%+v", st)
	}
}

// body.value is CEL or, with {{ }}, a template (the contract's cel|tpl).
func TestHTTPBodyValueIsCELOrATemplate(t *testing.T) {
	reg := catalog.NewRegistry(nil)
	if err := reg.RegisterTriggers(engine.Triggers()); err != nil {
		t.Fatal(err)
	}
	if err := builtin.Register(reg, builtin.Deps{}); err != nil {
		t.Fatal(err)
	}
	v, err := reg.ForTenant(context.Background(), uuid.New())
	if err != nil {
		t.Fatal(err)
	}
	for value, want := range map[string]string{
		`Hello, {{ event.input.name }}`: "Hello, Aziz",
		`'Hello, ' + event.input.name`:  "Hello, Aziz",
	} {
		raw, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		g, errs := graph.Decode([]byte(`{"name": "Hello", "triggers": [{"id": "m", "event": "automation.manual.v1"}],
		 "nodes": [{"id": "say", "type": "http", "params": {"method": "POST", "url": "https://api.example.com/hello",
		   "body": {"kind": "text", "value": ` + string(raw) + `}}}],
		 "edges": [{"from": {"node": "m", "outcome": "fired"}, "to": "say"},
		  {"from": {"node": "say", "outcome": "success"}, "to": "ignore"}, {"from": {"node": "say", "outcome": "failed"}, "to": "ignore"}]}`))
		if len(errs) > 0 {
			t.Fatal(errs)
		}
		if errs := graph.Validate(v.Catalog, g, graph.Options{}); len(errs) > 0 {
			t.Fatalf("%s: %v", value, errs)
		}
		in, err := graph.NewInterpreter(v.Catalog, g, graph.Options{})
		if err != nil {
			t.Fatal(err)
		}
		st := in.Start("say", &graph.StepContext{Ctx: graph.Context{Event: graph.Event{"kind": "automation.manual.v1", "input": map[string]any{"name": "Aziz"}, "initiator_user_id": 1}}})
		body, _ := st.Effects[0].Params["body"].(map[string]any)
		if body["value"] != want {
			t.Fatalf("%s: %+v", value, body)
		}
	}
}

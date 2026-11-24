package actions_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/iota-uz/iota-sdk/pkg/automation/builtin/actions"
	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/automation/graph"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/native"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/pool"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
)

type fake struct {
	key  string
	spec actions.ActionSpec
	run  func(ctx context.Context, args json.RawMessage) (string, json.RawMessage, error)
}

func (f fake) Key() string              { return f.key }
func (f fake) Spec() actions.ActionSpec { return f.spec }
func (f fake) Run(ctx context.Context, _ spi.Subject, args json.RawMessage) (string, json.RawMessage, error) {
	return f.run(ctx, args)
}

func spec() actions.ActionSpec {
	return actions.ActionSpec{
		Label: manifest.Label{"en": "Assign", "ru": "Назначить"}, Permission: "Clients.Update", Outcomes: []string{"assigned", "skipped"},
		Errors: []string{"client.archived"},
		Args: &manifest.Type{Kind: manifest.KindObject, Properties: []manifest.Property{
			{Name: "client_id", Type: manifest.Type{Kind: manifest.KindUUID}},
			{Name: "user_id", Type: manifest.Type{Kind: manifest.KindInt}},
		}},
		Output: &manifest.Type{Kind: manifest.KindObject, Properties: []manifest.Property{{Name: "by", Type: manifest.Type{Kind: manifest.KindInt}}}},
	}
}

func TestNodesAreCheckedAndUnique(t *testing.T) {
	ok := fake{key: "crm.client.assign", spec: spec()}
	if _, err := actions.Nodes(ok, ok); err == nil {
		t.Fatal("a key twice was accepted")
	}
	noPerm := spec()
	noPerm.Permission = ""
	for _, bad := range []actions.Action{fake{key: "Bad Key", spec: spec()}, fake{key: "crm.client.x", spec: noPerm}} {
		if _, err := actions.Nodes(bad); err == nil {
			t.Errorf("accepted %s", bad.Key())
		}
	}
	reg := catalog.NewRegistry(nil)
	if err := actions.Register(reg, ok); err != nil {
		t.Fatal(err)
	}
	v, err := reg.ForTenant(context.Background(), uuid.New(), catalog.AsAuthor(spi.Subject{Permissions: []string{}}), catalog.WithLocale("ru"))
	if err != nil {
		t.Fatal(err)
	}
	if v.Unavailable["action.crm.client.assign"] != catalog.ReasonPermission {
		t.Fatalf("an author without the permission sees the action: %v", v.Unavailable)
	}
	v, err = reg.ForTenant(context.Background(), uuid.New(), catalog.AsAuthor(spi.Subject{Permissions: []string{"Clients.Update"}}), catalog.WithLocale("ru"))
	if err != nil {
		t.Fatal(err)
	}
	n := v.Catalog.Nodes["action.crm.client.assign"]
	if n.Title != "Назначить" || len(n.Outcomes) != 3 || n.FailedOutcome != "failed" {
		t.Fatalf("spec %+v", n)
	}
	if len(n.Templates) != 1 || n.Templates[0] != "args.client_id" || len(n.Values) != 1 || n.Values[0] != "args.user_id" {
		t.Fatalf("templates %v values %v", n.Templates, n.Values)
	}
}

func TestCommandValidatesAndRunMapsErrors(t *testing.T) {
	act := fake{key: "crm.client.assign", spec: spec(), run: func(ctx context.Context, args json.RawMessage) (string, json.RawMessage, error) {
		var in struct {
			UserID int `json:"user_id"`
		}
		_ = json.Unmarshal(args, &in)
		switch in.UserID {
		case 0:
			return "", nil, actions.ErrForbidden
		case 1:
			return "", nil, &actions.ActionError{Code: "client.archived", Message: "the client is archived"}
		}
		return "assigned", json.RawMessage(`{"by": 7}`), nil
	}}
	nodes, err := actions.Nodes(act)
	if err != nil {
		t.Fatal(err)
	}
	cc := nodes[0].(catalog.CommandCall)
	in := func(args map[string]any, perms ...string) catalog.CallInput {
		return catalog.CallInput{TenantID: uuid.New(), IdempotencyKey: "r:n:t:1", Params: map[string]any{"args": args}, RunAs: spi.Subject{Permissions: perms}}
	}
	code := func(err error) string {
		var ce *catalog.CallError
		if !errors.As(err, &ce) {
			t.Fatalf("want a CallError, got %v", err)
		}
		return ce.Code
	}
	cid := uuid.NewString()
	if _, _, err := cc.Command(in(map[string]any{"client_id": cid, "user_id": 5})); code(err) != actions.CodeForbidden {
		t.Fatal("a command without the permission")
	}
	if _, _, err := cc.Command(in(map[string]any{"client_id": "nope", "user_id": 5}, "Clients.Update")); code(err) != actions.CodeInvalidArgs {
		t.Fatal("a command with invalid args")
	}
	command, input, err := cc.Command(in(map[string]any{"client_id": cid, "user_id": 5}, "Clients.Update"))
	if err != nil || command != "crm.client.assign" {
		t.Fatalf("%s %v", command, err)
	}
	// The command's body.
	sub := spi.Subject{Permissions: []string{"Clients.Update"}}
	out, err := actions.Run(context.Background(), act, sub, input)
	if err != nil {
		t.Fatal(err)
	}
	res, err := cc.CommandResult(in(nil), out)
	if err != nil || res.Outcome != "assigned" || res.Output["by"] != float64(7) {
		t.Fatalf("%v %+v", err, res)
	}
	wire := func(err error) *pool.WireError {
		var we *pool.WireError
		if !errors.As(err, &we) {
			t.Fatalf("want a wire error, got %v", err)
		}
		return we
	}
	_, err = actions.Run(context.Background(), act, spi.Subject{}, input)
	if we := wire(err); we.Kind != pool.ErrKindDenied {
		t.Fatalf("no permission: %+v", we)
	}
	zero := json.RawMessage(`{"client_id": "` + cid + `", "user_id": 0}`)
	_, err = actions.Run(context.Background(), act, sub, zero)
	if we := wire(err); we.Kind != pool.ErrKindDomain || we.Code != "automation.forbidden" {
		t.Fatalf("ErrForbidden: %+v", we)
	}
	one := json.RawMessage(`{"client_id": "` + cid + `", "user_id": 1}`)
	_, err = actions.Run(context.Background(), act, sub, one)
	if we := wire(err); we.Code != "automation.client.archived" {
		t.Fatalf("ActionError: %+v", we)
	}
	// Direct execution has no command to call.
	if _, err := nodes[0].(catalog.CallNode).Execute(context.Background(), in(nil)); code(err) != catalog.CallUnavailable {
		t.Fatal("Execute ran an action")
	}
}

// Native registers the action's command contract once; the action commands of a set are its nodes.
func TestNativeIsTheOneRegistration(t *testing.T) {
	act := fake{key: "crm.client.reassign", spec: spec()}
	item, err := actions.Native(act, func(native.Deps) (actions.Action, error) { return act, nil })
	if err != nil {
		t.Fatal(err)
	}
	set := native.NewSet()
	if err := set.Add(item); err != nil {
		t.Fatal(err)
	}
	if err := set.Add(actions.MustNative(act, func(native.Deps) (actions.Action, error) { return act, nil })); err == nil {
		t.Fatal("a key twice was accepted")
	}
	nodes, err := actions.NodesOf(set)
	if err != nil || len(nodes) != 1 || nodes[0].Spec().Type != "action.crm.client.reassign" {
		t.Fatalf("%v %v", err, nodes)
	}
	d, ok := set.Contract("crm.client.reassign")
	if !ok || d.Permission() != "Clients.Update" {
		t.Fatalf("contract %v", d)
	}
}

// The action node dispatches to the action it names.
func TestFrontDispatches(t *testing.T) {
	reg := catalog.NewRegistry(nil)
	if err := reg.RegisterNode(actions.Front()); err != nil {
		t.Fatal(err)
	}
	if err := actions.Register(reg, fake{key: "crm.client.assign", spec: spec()}); err != nil {
		t.Fatal(err)
	}
	v, err := reg.ForTenant(context.Background(), uuid.New())
	if err != nil {
		t.Fatal(err)
	}
	g, errs := graph.Decode([]byte(`{"name": "A", "triggers": [{"id": "t", "event": "x"}], "nodes": [
		{"id": "a", "type": "action", "params": {"action": "crm.client.assign", "args": {"client_id": "{{ 'x' }}", "user_id": "1"}}}], "edges": []}`))
	if len(errs) > 0 {
		t.Fatal(errs)
	}
	r := graph.Resolve(v.Catalog, g)
	if r.Nodes[0].Type != "action.crm.client.assign" || r.Nodes[0].Params["action"] != nil || g.Nodes[0].Type != "action" {
		t.Fatalf("resolved %+v (original %+v)", r.Nodes[0], g.Nodes[0])
	}
}

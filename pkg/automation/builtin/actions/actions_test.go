package actions_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/iota-uz/iota-sdk/pkg/automation/builtin/actions"
	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
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

func TestExecuteValidatesChecksAndMapsErrors(t *testing.T) {
	var gotInvocation string
	act := fake{key: "crm.client.assign", spec: spec(), run: func(ctx context.Context, args json.RawMessage) (string, json.RawMessage, error) {
		gotInvocation = actions.InvocationID(ctx)
		var in struct {
			UserID int `json:"user_id"`
		}
		_ = json.Unmarshal(args, &in)
		switch in.UserID {
		case 0:
			return "", nil, actions.ErrForbidden
		case 1:
			return "", nil, &actions.ActionError{Code: "client_archived", Message: "the client is archived"}
		}
		return "assigned", json.RawMessage(`{"by": 7}`), nil
	}}
	nodes, err := actions.Nodes(act)
	if err != nil {
		t.Fatal(err)
	}
	cn := nodes[0].(catalog.CallNode)
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
	if _, err := cn.Execute(context.Background(), in(map[string]any{"client_id": cid, "user_id": 5})); code(err) != actions.CodeForbidden {
		t.Fatal("ran without the permission")
	}
	if _, err := cn.Execute(context.Background(), in(map[string]any{"client_id": "nope", "user_id": 5}, "Clients.Update")); code(err) != actions.CodeInvalidArgs {
		t.Fatal("ran with invalid args")
	}
	if _, err := cn.Execute(context.Background(), in(map[string]any{"client_id": cid, "user_id": 0}, "Clients.Update")); code(err) != actions.CodeForbidden {
		t.Fatal("ErrForbidden is not forbidden")
	}
	if _, err := cn.Execute(context.Background(), in(map[string]any{"client_id": cid, "user_id": 1}, "Clients.Update")); code(err) != "client_archived" {
		t.Fatal("an ActionError lost its code")
	}
	res, err := cn.Execute(context.Background(), in(map[string]any{"client_id": cid, "user_id": 5}, "Clients.Update"))
	if err != nil || res.Outcome != "assigned" || res.Output["by"] != float64(7) {
		t.Fatalf("%v %+v", err, res)
	}
	if gotInvocation != catalog.InvocationIDOf("r:n:t:1") {
		t.Fatalf("invocation %q", gotInvocation)
	}
}

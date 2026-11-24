package access_test

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/automation/builtin/access"
	"github.com/iota-uz/iota-sdk/pkg/automation/code"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors"
)

func TestTargetStaysOnTheConnectorsOrigin(t *testing.T) {
	c := access.NewConnection("crm", "acme", connectors.CodeAccess{BaseURL: "https://api.example.com/v2/"}, nil, nil)
	cases := map[string]string{
		"/deals?limit=1":                    "https://api.example.com/v2/deals?limit=1",
		"deals":                             "https://api.example.com/v2/deals",
		"https://api.example.com/other":     "https://api.example.com/other",
		"https://API.example.com:443/other": "https://API.example.com:443/other",
	}
	for in, want := range cases {
		u, err := access.Target(c, in)
		if err != nil || u.String() != want {
			t.Errorf("%s: %v %v", in, u, err)
		}
	}
	for _, bad := range []string{"https://evil.example.org/", "//evil.example.org/x", "http://api.example.com/", "https://api.example.com:8443/", "https://u:p@api.example.com/"} {
		_, err := access.Target(c, bad)
		var refused *code.ConnectionRefusedError
		if !errors.As(err, &refused) || refused.Code != code.ReasonConnectionOriginMismatch {
			t.Errorf("%s: %v", bad, err)
		}
	}
}

type recorder struct{ got code.FetchRequest }

func (r *recorder) Fetch(_ context.Context, req code.FetchRequest) (*code.FetchResponse, error) {
	r.got = req
	return &code.FetchResponse{Status: 200}, nil
}

func TestGatewayAuthorizesTheDeclaredMethod(t *testing.T) {
	ca := connectors.CodeAccess{BaseURL: "https://api.example.com", Methods: []connectors.CodeMethod{{Name: "list", HTTPMethod: "get"}}}
	c := access.NewConnection("crm", "acme", ca, []string{"tok"}, func(_ context.Context, h http.Header) error {
		h.Set("Authorization", "Bearer tok")
		return nil
	})
	rec := &recorder{}
	g := &access.Gateway{Fetcher: rec, Connections: map[string]access.Connection{"crm": c}}
	if _, err := g.CallConnection(context.Background(), code.ConnectionRequest{Connection: "crm", Method: "list", Path: "/deals"}); err != nil {
		t.Fatal(err)
	}
	if rec.got.Method != "GET" || rec.got.URL != "https://api.example.com/deals" || rec.got.Header.Get("Authorization") != "Bearer tok" {
		t.Fatalf("request %+v", rec.got)
	}
	if _, err := g.CallConnection(context.Background(), code.ConnectionRequest{Connection: "crm", Method: "delete", Path: "/deals"}); err == nil {
		t.Fatal("an undeclared method was called")
	}
	_, err := g.CallConnection(context.Background(), code.ConnectionRequest{Connection: "mail", Method: "list", Path: "/"})
	var refused *code.ConnectionRefusedError
	if !errors.As(err, &refused) || refused.Code != code.ReasonConnectionMissing {
		t.Fatalf("unknown connection: %v", err)
	}
}

func TestCodeAccessValidate(t *testing.T) {
	ok := connectors.CodeAccess{BaseURL: "https://api.example.com/v1", Methods: []connectors.CodeMethod{{Name: "get", HTTPMethod: "GET"}, {Name: "list", HTTPMethod: "GET"}}}
	if err := ok.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []connectors.CodeAccess{
		{BaseURL: "http://api.example.com"},
		{BaseURL: "https://api.example.com?x=1"},
		{BaseURL: "https://api.example.com", Methods: []connectors.CodeMethod{{Name: "Get", HTTPMethod: "GET"}}},
		{BaseURL: "https://api.example.com", Methods: []connectors.CodeMethod{{Name: "get", HTTPMethod: "GET"}, {Name: "get", HTTPMethod: "POST"}}},
		{BaseURL: "https://api.example.com", Methods: []connectors.CodeMethod{{Name: "get", HTTPMethod: "TRACE"}}},
	} {
		if bad.Validate() == nil {
			t.Errorf("accepted %+v", bad)
		}
	}
}

package application

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/iota-uz/applets"
)

func TestToBunDelegateMethodName(t *testing.T) {
	t.Parallel()

	got, err := toBunDelegateMethodName("bichat", "bichat.session.list")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "bichat.__go.session.list" {
		t.Fatalf("unexpected delegate method: %s", got)
	}
}

func TestToBunDelegateMethodNameRejectsMismatchedNamespace(t *testing.T) {
	t.Parallel()

	if _, err := toBunDelegateMethodName("bichat", "chat.session.list"); err == nil {
		t.Fatal("expected namespace validation error")
	}
}

func TestMakeBunPublicProxyMethodPreservesPermissionsAndRejectsDirectExecution(t *testing.T) {
	t.Parallel()

	method := makeBunPublicProxyMethod("bichat.session.list", "bichat.__go.session.list", applets.RPCMethod{
		RequirePermissions: []string{"BiChat.Access"},
	})
	if len(method.RequirePermissions) != 1 || method.RequirePermissions[0] != "BiChat.Access" {
		t.Fatalf("unexpected permissions: %#v", method.RequirePermissions)
	}
	if method.Handler == nil {
		t.Fatal("proxy handler is required")
	}
	if _, err := method.Handler(context.Background(), json.RawMessage(`{}`)); err == nil {
		t.Fatal("expected proxy handler error")
	}
}

package vault_test

import (
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/config/providers/vault"
)

func TestNew_ReturnsError(t *testing.T) {
	t.Parallel()

	p, err := vault.New("http://vault.local:8200", "token", "secret/app")
	if err == nil {
		t.Fatal("vault.New should return error (not implemented), got nil")
	}
	if p != nil {
		t.Error("vault.New should return nil provider when returning error")
	}
	if err.Error() == "" {
		t.Error("error message should not be empty")
	}
}

// Package vault provides a config.Provider stub for HashiCorp Vault.
//
// This is a placeholder implementation. The Load method is not yet implemented.
// Vault provider is stubbed; tracked by iota-uz/iota-sdk#754.
// Current New returns errNotImplemented so callers fail loudly instead of silently skipping.
package vault

import (
	"errors"

	"github.com/iota-uz/iota-sdk/pkg/config"
)

var errNotImplemented = errors.New("vault provider: not implemented")

// Ensure *vaultProvider implements config.Provider at compile time.
var _ config.Provider = (*vaultProvider)(nil)

// New returns a Vault Provider stub.
// addr, token, and path are accepted for API compatibility with future waves
// but are currently unused. Returns (nil, error) indicating the provider is
// not yet implemented.
func New(addr, token, path string) (config.Provider, error) {
	return nil, errNotImplemented
}

type vaultProvider struct{}

// Name returns "vault".
func (p *vaultProvider) Name() string {
	return "vault"
}

// Load is a no-op in W0.1.
func (p *vaultProvider) Load() (map[string]any, error) {
	return nil, errNotImplemented
}

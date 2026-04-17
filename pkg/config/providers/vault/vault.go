// Package vault provides a config.Provider stub for HashiCorp Vault.
//
// This is a placeholder implementation for Wave 0.1. The Load method is a
// no-op. Full Vault integration ships in a future wave.
//
// TODO(W-future): implement HashiCorp Vault lookup; spec at pkg/config/providers/vault/README.md
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

// Load is a no-op in W0.1.
func (p *vaultProvider) Load() (map[string]any, error) {
	return nil, errNotImplemented
}

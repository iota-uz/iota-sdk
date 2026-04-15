// Package vault provides a config.Provider stub for HashiCorp Vault.
//
// This is a placeholder implementation for Wave 0.1. The Load method is a
// no-op. Full Vault integration ships in a future wave.
//
// TODO(W-future): implement HashiCorp Vault lookup; spec at pkg/config/providers/vault/README.md
package vault

import (
	"github.com/knadh/koanf/v2"

	"github.com/iota-uz/iota-sdk/pkg/config"
)

// New returns a Vault Provider stub.
// addr, token, and path are accepted for API compatibility with future waves
// but are currently unused.
func New(addr, token, path string) config.Provider {
	return &vaultProvider{}
}

type vaultProvider struct{}

// Load is a no-op in W0.1. Full implementation ships in a future wave.
func (p *vaultProvider) Load(_ *koanf.Koanf) error {
	return nil
}

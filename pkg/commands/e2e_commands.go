// Package commands provides backward compatibility wrappers for e2e commands
package commands

import (
	"github.com/iota-uz/iota-sdk/pkg/commands/e2e"
)

// Backward compatibility constants
const (
	E2E_DB_NAME     = e2e.E2E_DB_NAME
	E2E_SERVER_PORT = e2e.E2E_SERVER_PORT
	E2E_SERVER_HOST = e2e.E2E_SERVER_HOST
)

// Backward compatibility functions - delegate to split e2e package

func E2ECreate() error {
	return e2e.Create()
}

func E2EDrop() error {
	return e2e.Drop()
}

func E2EMigrate() error {
	return e2e.Migrate()
}

func E2ESeed() error {
	return e2e.Seed()
}

func E2ESetup() error {
	return e2e.Setup()
}

func E2EReset() error {
	return e2e.Reset()
}

func E2ETest() error {
	return e2e.Test()
}

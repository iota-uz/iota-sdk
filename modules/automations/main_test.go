package automations_test

import (
	"os"
	"testing"
)

// The migrator reads ./migrations relative to the working directory; the locale test reads the
// screen's sources relative to the module, so it resolves them before moving.
func TestMain(m *testing.M) {
	if err := os.Chdir("../.."); err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}

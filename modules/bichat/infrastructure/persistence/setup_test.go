package persistence_test

import (
	"os"
	"testing"

	"github.com/iota-uz/iota-sdk/modules"
	"github.com/iota-uz/iota-sdk/pkg/itf"
)

func TestMain(m *testing.M) {
	if err := os.Chdir("../../../../"); err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}

// setupTest creates all necessary dependencies for tests
func setupTest(t *testing.T) *itf.TestEnvironment {
	t.Helper()

	return itf.Setup(t, itf.WithModules(modules.BuiltInModules...))
}

// setupBenchmark creates all necessary dependencies for benchmarks
func setupBenchmark(b *testing.B) *itf.TestEnvironment {
	b.Helper()

	return itf.Setup(b, itf.WithModules(modules.BuiltInModules...))
}

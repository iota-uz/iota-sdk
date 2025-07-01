package persistence_test

import (
	"os"
	"testing"

	"github.com/iota-uz/iota-sdk/modules"
	"github.com/iota-uz/iota-sdk/pkg/testutils/builder"
)

func TestMain(m *testing.M) {
	if err := os.Chdir("../../../../"); err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}

// setupTest creates all necessary dependencies for tests
func setupTest(t *testing.T) *builder.TestEnvironment {
	t.Helper()

	return builder.New().
		WithModules(modules.BuiltInModules...).
		Build(t)
}

// setupBenchmark creates all necessary dependencies for benchmarks
func setupBenchmark(b *testing.B) *builder.TestEnvironment {
	b.Helper()

	return builder.New().
		WithModules(modules.BuiltInModules...).
		Build(b)
}

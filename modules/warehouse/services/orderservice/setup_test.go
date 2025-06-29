package orderservice_test

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
	code := m.Run()
	os.Exit(code)
}

// setupTest creates all necessary dependencies for tests
func setupTest(t *testing.T) *builder.TestEnvironment {
	t.Helper()

	return builder.New().
		WithModules(modules.BuiltInModules...).
		Build(t)
}

package services_test

import (
	"os"
	"testing"

	"github.com/iota-uz/iota-sdk/modules"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/iota-uz/iota-sdk/pkg/itf"
)

func TestMain(m *testing.M) {
	if err := os.Chdir("../../../"); err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}

// setupTest creates all necessary dependencies for tests
func setupTest(tb testing.TB, permissions ...*permission.Permission) *itf.TestEnvironment {
	tb.Helper()

	user := itf.User(permissions...)
	return itf.Setup(tb,
		itf.WithModules(modules.BuiltInModules...),
		itf.WithUser(user),
	)
}

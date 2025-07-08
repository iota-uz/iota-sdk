package services_test

import (
	"os"
	"testing"

	"github.com/iota-uz/iota-sdk/modules"
	"github.com/iota-uz/iota-sdk/modules/billing/services"
	"github.com/iota-uz/iota-sdk/pkg/itf"
)

func TestMain(m *testing.M) {
	if err := os.Chdir("../../../"); err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}

// setupTest creates all necessary dependencies for tests
func setupTest(t *testing.T) *itf.TestEnvironment {
	t.Helper()

	return itf.Setup(t, itf.WithModules(modules.BuiltInModules...))
}

// Helper function to get BillingService from TestEnvironment
func getBillingService(env *itf.TestEnvironment) *services.BillingService {
	return env.Service(services.BillingService{}).(*services.BillingService)
}

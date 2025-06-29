package services_test

import (
	"os"
	"testing"

	"github.com/iota-uz/iota-sdk/modules"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/iota-uz/iota-sdk/modules/finance/services"
	"github.com/iota-uz/iota-sdk/pkg/testutils"
	"github.com/iota-uz/iota-sdk/pkg/testutils/builder"
)

func TestMain(m *testing.M) {
	if err := os.Chdir("../../../"); err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}

// setupTest creates all necessary dependencies for tests
func setupTest(t *testing.T, permissions ...*permission.Permission) *builder.TestEnvironment {
	t.Helper()

	user := testutils.MockUser(permissions...)
	return builder.New().
		WithModules(modules.BuiltInModules...).
		WithUser(user).
		Build(t)
}

// Helper functions to get services from TestEnvironment
func getPaymentService(env *builder.TestEnvironment) *services.PaymentService {
	return env.Service(services.PaymentService{}).(*services.PaymentService)
}

func getAccountService(env *builder.TestEnvironment) *services.MoneyAccountService {
	return env.Service(services.MoneyAccountService{}).(*services.MoneyAccountService)
}

func getPaymentCategoryService(env *builder.TestEnvironment) *services.PaymentCategoryService {
	return env.Service(services.PaymentCategoryService{}).(*services.PaymentCategoryService)
}

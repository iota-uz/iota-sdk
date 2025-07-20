package services_test

import (
	"os"
	"testing"

	"github.com/iota-uz/iota-sdk/modules"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/iota-uz/iota-sdk/modules/finance/services"
	"github.com/iota-uz/iota-sdk/pkg/itf"
)

func TestMain(m *testing.M) {
	if err := os.Chdir("../../../"); err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}

// setupTest creates all necessary dependencies for tests
func setupTest(t *testing.T, permissions ...*permission.Permission) *itf.TestEnvironment {
	t.Helper()

	user := itf.User(permissions...)
	return itf.Setup(t,
		itf.WithModules(modules.BuiltInModules...),
		itf.WithUser(user),
	)
}

// Helper functions to get services from TestEnvironment
func getPaymentService(env *itf.TestEnvironment) *services.PaymentService {
	return env.Service(services.PaymentService{}).(*services.PaymentService)
}

func getAccountService(env *itf.TestEnvironment) *services.MoneyAccountService {
	return env.Service(services.MoneyAccountService{}).(*services.MoneyAccountService)
}

func getPaymentCategoryService(env *itf.TestEnvironment) *services.PaymentCategoryService {
	return env.Service(services.PaymentCategoryService{}).(*services.PaymentCategoryService)
}

func getDebtService(env *itf.TestEnvironment) *services.DebtService {
	return env.Service(services.DebtService{}).(*services.DebtService)
}

func getFinancialReportService(env *itf.TestEnvironment) *services.FinancialReportService {
	return env.Service(services.FinancialReportService{}).(*services.FinancialReportService)
}

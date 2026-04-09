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
func setupTest(t *testing.T, permissions ...permission.Permission) *itf.TestEnvironment {
	t.Helper()

	user := itf.User(permissions...)
	return itf.Setup(t,
		itf.WithComponents(modules.Components()...),
		itf.WithUser(user),
	)
}

// Helper functions to get services from TestEnvironment
func getPaymentService(env *itf.TestEnvironment) *services.PaymentService {
	return itf.GetService[services.PaymentService](env)
}

func getAccountService(env *itf.TestEnvironment) *services.MoneyAccountService {
	return itf.GetService[services.MoneyAccountService](env)
}

func getPaymentCategoryService(env *itf.TestEnvironment) *services.PaymentCategoryService {
	return itf.GetService[services.PaymentCategoryService](env)
}

func getDebtService(env *itf.TestEnvironment) *services.DebtService {
	return itf.GetService[services.DebtService](env)
}

func getFinancialReportService(env *itf.TestEnvironment) *services.FinancialReportService {
	return itf.GetService[services.FinancialReportService](env)
}

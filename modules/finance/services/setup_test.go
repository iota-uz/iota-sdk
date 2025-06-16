package services_test

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/session"
	"github.com/iota-uz/iota-sdk/modules/finance/services"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/testutils"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestMain(m *testing.M) {
	if err := os.Chdir("../../../"); err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}

// testFixtures contains common test dependencies
type testFixtures struct {
	ctx                    context.Context
	pool                   *pgxpool.Pool
	app                    application.Application
	paymentsService        *services.PaymentService
	accountService         *services.MoneyAccountService
	paymentCategoryService *services.PaymentCategoryService
	tenantID               uuid.UUID
}

// setupTest creates all necessary dependencies for tests
func setupTest(t *testing.T, permissions ...*permission.Permission) *testFixtures {
	t.Helper()

	testutils.CreateDB(t.Name())
	pool := testutils.NewPool(testutils.DbOpts(t.Name()))

	ctx := composables.WithUser(context.Background(), testutils.MockUser(permissions...))

	t.Cleanup(func() {
		pool.Close()
	})

	ctx = composables.WithPool(ctx, pool)
	ctx = composables.WithSession(ctx, &session.Session{})

	// Setup application using testutils helper
	app, err := testutils.SetupApplication(pool, modules.BuiltInModules...)
	if err != nil {
		t.Fatal(err)
	}

	// Create a test tenant and add it to the context
	tenant, err := testutils.CreateTestTenant(ctx, pool)
	if err != nil {
		t.Fatal(err)
	}
	ctx = composables.WithTenantID(ctx, tenant.ID)

	return &testFixtures{
		ctx:                    ctx,
		pool:                   pool,
		app:                    app,
		paymentsService:        app.Service(services.PaymentService{}).(*services.PaymentService),
		accountService:         app.Service(services.MoneyAccountService{}).(*services.MoneyAccountService),
		paymentCategoryService: app.Service(services.PaymentCategoryService{}).(*services.PaymentCategoryService),
		tenantID:               tenant.ID,
	}
}

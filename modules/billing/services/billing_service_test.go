package services_test

import (
	"context"
	"github.com/iota-uz/iota-sdk/modules/billing/domain/aggregates/billing"
	"github.com/iota-uz/iota-sdk/modules/billing/domain/aggregates/details"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/iota-uz/iota-sdk/pkg/logging"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules"
	"github.com/iota-uz/iota-sdk/modules/billing/services"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/eventbus"
	"github.com/iota-uz/iota-sdk/pkg/testutils"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- testFixtures ---

type testFixtures struct {
	ctx            context.Context
	pool           *pgxpool.Pool
	publisher      eventbus.EventBus
	billingService *services.BillingService
}

// --- setup ---

func setupTest(t *testing.T, permissions ...*permission.Permission) *testFixtures {
	t.Helper()

	testutils.CreateDB(t.Name())
	pool := testutils.NewPool(testutils.DbOpts(t.Name()))

	ctx := context.Background()
	ctx = composables.WithUser(ctx, testutils.MockUser(permissions...))
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		if err := tx.Commit(ctx); err != nil {
			t.Fatal(err)
		}
		pool.Close()
	})

	ctx = composables.WithTx(ctx, tx)
	ctx = composables.WithPool(ctx, pool)
	ctx = composables.WithSession(ctx, testutils.MockSession())

	publisher := eventbus.NewEventPublisher(logging.ConsoleLogger(logrus.WarnLevel))
	app := setupApplication(t, pool, publisher)

	return &testFixtures{
		ctx:            ctx,
		pool:           pool,
		publisher:      publisher,
		billingService: app.Service(services.BillingService{}).(*services.BillingService),
	}
}

func setupApplication(t *testing.T, pool *pgxpool.Pool, publisher eventbus.EventBus) application.Application {
	t.Helper()
	app := application.New(pool, publisher)
	if err := modules.Load(app, modules.BuiltInModules...); err != nil {
		t.Fatal(err)
	}
	if err := app.Migrations().Run(); err != nil {
		t.Fatal(err)
	}
	return app
}

// --- Test ---

func TestBillingService_CreateTransaction(t *testing.T) {
	t.Helper()
	t.Parallel()
	f := setupTest(t)

	cmd := &services.CreateTransactionCommand{
		Quantity: 1001,
		Currency: billing.UZS,
		Gateway:  billing.Click,
		Details: details.NewClickDetails(
			"super_test_3",
			details.ClickWithParams(map[string]any{
				"additional_param3": "test123",
				"communal_param":    "1",
			}),
		),
	}

	result, err := f.billingService.Create(f.ctx, cmd)
	require.NoError(t, err, "Create should succeed")
	require.NotNil(t, result, "Transaction should not be nil")

	assert.Equal(t, billing.Created, result.Status())
	assert.Equal(t, billing.UZS, result.Amount().Currency())
	assert.InDelta(t, float64(1001), result.Amount().Quantity(), 0.0001)
	assert.NotEqual(t, uuid.Nil, result.ID(), "Expected non-nil transaction ID")
	assert.WithinDuration(t, time.Now(), result.CreatedAt(), time.Second*2)
}

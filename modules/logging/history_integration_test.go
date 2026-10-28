package logging_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules"
	"github.com/iota-uz/iota-sdk/modules/core"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/currency"
	corepersistence "github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/finance"
	moneyaccount "github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/money_account"
	"github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/payment"
	paymentcategory "github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/payment_category"
	"github.com/iota-uz/iota-sdk/modules/finance/domain/entities/counterparty"
	"github.com/iota-uz/iota-sdk/modules/finance/permissions"
	financeservices "github.com/iota-uz/iota-sdk/modules/finance/services"
	"github.com/iota-uz/iota-sdk/modules/logging/domain/aggregates/businessevent"
	"github.com/iota-uz/iota-sdk/modules/logging/history"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/composition"
	"github.com/iota-uz/iota-sdk/pkg/defaults"
	"github.com/iota-uz/iota-sdk/pkg/health"
	importpkg "github.com/iota-uz/iota-sdk/pkg/import"
	"github.com/iota-uz/iota-sdk/pkg/itf"
	"github.com/iota-uz/iota-sdk/pkg/jobs"
	"github.com/iota-uz/iota-sdk/pkg/money"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	if err := os.Chdir("../../"); err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}

func TestBusinessHistory_OffWithoutLoggingComponent(t *testing.T) {
	t.Parallel()
	env := itf.Setup(t,
		itf.WithComponents(core.NewComponent(&core.ModuleOptions{PermissionSchema: defaults.PermissionSchema()}), finance.NewComponent()),
		itf.WithUser(itf.User(permissions.Permissions...)),
		itf.WithCapabilities(composition.Capability("history-test")),
	)
	require.Equal(t, history.Disabled(), resolve[history.Writer](t, env))
	base := composables.WithSession(composables.WithParams(composables.WithTenantID(composables.WithPool(context.Background(), env.Pool), env.TenantID()), itf.DefaultParams()), itf.MockSession())
	fixture := newPaymentFixture(t, env, base)

	require.NoError(t, fixture.create(composables.WithUser(base, env.User), env, resolve[*financeservices.PaymentService](t, env), "no history"))

	var events int
	require.NoError(t, env.Pool.QueryRow(base, `SELECT COUNT(*) FROM business_events WHERE tenant_id = $1`, env.TenantID()).Scan(&events))
	require.Zero(t, events)
}

// setupCommitted compiles the application without the API and worker capabilities, so no
// background outbox loop races the test, and returns a context whose writes commit.
func setupCommitted(t *testing.T) (*itf.TestEnvironment, context.Context) {
	t.Helper()
	env := itf.Setup(t,
		itf.WithComponents(modules.Components()...),
		itf.WithUser(itf.User(permissions.Permissions...)),
		itf.WithCapabilities(composition.Capability("history-test")),
	)
	ctx := composables.WithPool(context.Background(), env.Pool)
	ctx = composables.WithTenantID(ctx, env.TenantID())
	ctx = composables.WithParams(ctx, itf.DefaultParams())
	ctx = composables.WithSession(ctx, itf.MockSession())
	return env, ctx
}

func resolve[T any](t *testing.T, env *itf.TestEnvironment) T {
	t.Helper()
	v, err := composition.Resolve[T](env.Container)
	require.NoError(t, err)
	return v
}

type paymentFixture struct {
	account      moneyaccount.Account
	category     paymentcategory.PaymentCategory
	counterparty counterparty.Counterparty
}

func newPaymentFixture(t *testing.T, env *itf.TestEnvironment, ctx context.Context) paymentFixture {
	t.Helper()
	ctx = history.SystemExecution(composables.WithUser(ctx, env.User), "test.setup", "Test setup")
	require.NoError(t, corepersistence.NewCurrencyRepository().Create(ctx, currency.USD))
	account, err := resolve[*financeservices.MoneyAccountService](t, env).Create(ctx, moneyaccount.New("Cash", money.New(0, "USD")))
	require.NoError(t, err)
	category, err := resolve[*financeservices.PaymentCategoryService](t, env).Create(ctx, paymentcategory.New("Sales", paymentcategory.WithTenantID(env.TenantID())))
	require.NoError(t, err)
	party, err := resolve[*financeservices.CounterpartyService](t, env).Create(ctx, counterparty.New("Client", counterparty.Customer, counterparty.Individual, counterparty.WithTenantID(env.TenantID())))
	require.NoError(t, err)
	return paymentFixture{account: account, category: category, counterparty: party}
}

func (f paymentFixture) create(ctx context.Context, env *itf.TestEnvironment, payments *financeservices.PaymentService, comment string) error {
	_, err := payments.Create(ctx, payment.New(money.New(1500, "USD"), f.category,
		payment.WithTenantID(env.TenantID()),
		payment.WithAccount(f.account),
		payment.WithCounterpartyID(f.counterparty.ID()),
		payment.WithTransactionDate(time.Date(2026, 10, 28, 0, 0, 0, 0, time.UTC)),
		payment.WithAccountingPeriod(time.Date(2026, 10, 28, 0, 0, 0, 0, time.UTC)),
		payment.WithComment(comment),
	))
	return err
}

type paymentEvent struct {
	channel, actorKind, actorID, initiatorID, operationID, requestID, runID string
}

func TestBusinessHistory_SameActionAcrossChannels(t *testing.T) {
	t.Parallel()
	env, base := setupCommitted(t)
	fixture := newPaymentFixture(t, env, base)
	payments := resolve[*financeservices.PaymentService](t, env)
	userCtx := composables.WithUser(base, env.User)
	userID := strconv.FormatUint(uint64(env.User.ID()), 10)

	handler := history.Middleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := fixture.create(r.Context(), env, payments, "ui"); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}))
	recorder := httptest.NewRecorder()
	recorder.Header().Set("X-Request-Id", "req-ui")
	handler.ServeHTTP(recorder, httptest.NewRequestWithContext(userCtx, http.MethodPost, "/finance/payments", nil))
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())

	runs := importpkg.NewImportRunner(importpkg.NewMemoryRunStore())
	runs.Start(userCtx, "import-run", importProcessor(func(ctx context.Context) error {
		return fixture.create(ctx, env, payments, "import")
	}), "", importpkg.ImportRunOptions{})
	require.Eventually(t, func() bool {
		st, _ := runs.Get("import-run")
		return st.Status == importpkg.RunDone || st.Status == importpkg.RunFailed
	}, 10*time.Second, 20*time.Millisecond)
	st, _ := runs.Get("import-run")
	require.Equal(t, importpkg.RunDone, st.Status, st.Err)

	registry := jobs.NewRegistry()
	registry.Register("payments.create", func(ctx context.Context, _ map[string]any, _ jobs.ProgressReporter) (jobs.Result, error) {
		return jobs.Result{}, fixture.create(ctx, env, payments, "job")
	})
	runner := jobs.NewRunner(jobs.NewMemoryStore(0), registry, jobs.RunnerOptions{})
	job, err := runner.Enqueue(userCtx, "payments.create", nil)
	require.NoError(t, err)
	runner.Wait()

	rows, err := env.Pool.Query(base, `SELECT channel, actor_kind, actor_id, COALESCE(initiator_snapshot->>'user_id', ''),
		operation_id, COALESCE(request_id, ''), COALESCE(run_id, '')
		FROM business_events WHERE tenant_id = $1 AND event_type = $2 AND outcome = 'succeeded'`, env.TenantID(), financeservices.PaymentCreated)
	require.NoError(t, err)
	byChannel := map[string]paymentEvent{}
	for rows.Next() {
		var e paymentEvent
		require.NoError(t, rows.Scan(&e.channel, &e.actorKind, &e.actorID, &e.initiatorID, &e.operationID, &e.requestID, &e.runID))
		byChannel[e.channel] = e
	}
	require.NoError(t, rows.Err())

	tests := []struct {
		channel string
		check   func(t *testing.T, e paymentEvent)
	}{
		{"ui", func(t *testing.T, e paymentEvent) {
			t.Helper()
			require.Equal(t, "user", e.actorKind)
			require.Equal(t, userID, e.actorID)
			require.Equal(t, "req-ui", e.requestID)
			require.NotEqual(t, "req-ui", e.operationID)
		}},
		{"import", func(t *testing.T, e paymentEvent) {
			t.Helper()
			require.Equal(t, "user", e.actorKind)
			require.Equal(t, userID, e.actorID)
			require.Equal(t, "import-run", e.runID)
		}},
		{"job", func(t *testing.T, e paymentEvent) {
			t.Helper()
			require.Equal(t, "service", e.actorKind)
			require.Equal(t, "job:payments.create", e.actorID)
			require.Equal(t, userID, e.initiatorID)
			require.Equal(t, job.ID.String(), e.operationID)
			require.Equal(t, job.ID.String(), e.runID)
		}},
	}
	require.Len(t, byChannel, len(tests))
	for _, tt := range tests {
		t.Run(tt.channel, func(t *testing.T) {
			e, ok := byChannel[tt.channel]
			require.True(t, ok, "no %s event", financeservices.PaymentCreated)
			tt.check(t, e)
		})
	}
}

func TestBusinessHistory_MutationWithoutExecutionFails(t *testing.T) {
	t.Parallel()
	env, base := setupCommitted(t)
	fixture := newPaymentFixture(t, env, base)
	payments := resolve[*financeservices.PaymentService](t, env)

	err := fixture.create(composables.WithUser(base, env.User), env, payments, "no execution")
	require.ErrorIs(t, err, history.ErrExecutionMissing)

	var count int
	require.NoError(t, env.Pool.QueryRow(base, `SELECT COUNT(*) FROM payments WHERE tenant_id = $1`, env.TenantID()).Scan(&count))
	require.Zero(t, count, "the payment must roll back with its missing history")
}

type importProcessor func(ctx context.Context) error

func (p importProcessor) Process(ctx context.Context, _ string, _ importpkg.ImportRunOptions, _ importpkg.ProgressSink) (importpkg.ImportResult, error) {
	return importpkg.ImportResult{}, p(ctx)
}

type testConsumer struct {
	name string
	mu   sync.Mutex
	seen map[uuid.UUID]int
	fail bool
}

func (c *testConsumer) Name() string { return c.name }

func (c *testConsumer) Handle(_ context.Context, e businessevent.Event) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.fail {
		return errors.New("projection store is down")
	}
	c.seen[e.ID]++
	return nil
}

func (c *testConsumer) deliveries(id uuid.UUID) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.seen[id]
}

func (c *testConsumer) setFail(fail bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.fail = fail
}

func TestBusinessHistory_OutboxSurvivesCrashAndConsumerFailure(t *testing.T) {
	t.Parallel()
	env, base := setupCommitted(t)
	registry := resolve[*history.Registry](t, env)
	require.NoError(t, registry.Register(history.Definition{
		Type: "test.widget.created", Version: 1, Category: businessevent.CategoryBusiness,
		TitleKey: "Test.Title", DescriptionKey: "Test.Summary",
		Resources: []history.Resource{{Name: "test.widget"}},
	}))
	projection := &testConsumer{name: "test.projection", seen: map[uuid.UUID]int{}}
	integration := &testConsumer{name: "test.integration", seen: map[uuid.UUID]int{}, fail: true}
	require.NoError(t, resolve[*history.Consumers](t, env).Register(projection, integration))
	writer := resolve[history.Writer](t, env)
	dispatcher := resolve[*history.Dispatcher](t, env)

	ctx := history.SystemExecution(base, "test.writer", "Test writer")
	exec, _ := history.UseExecution(ctx)
	in := history.Input{Type: "test.widget.created", Version: 1, Outcome: businessevent.OutcomeSucceeded,
		Subjects: []businessevent.Subject{{Resource: "test.widget", ID: "w-1"}}}
	var eventID uuid.UUID
	record := func() {
		require.NoError(t, composables.InTx(ctx, func(txCtx context.Context) error {
			id, err := writer.Record(txCtx, in)
			eventID = id
			return err
		}))
	}
	record()

	// The process stops after commit, before any delivery: the event and its pending
	// deliveries are durable, and repeating the operation adds neither.
	record()
	pending := func() int {
		var n int
		require.NoError(t, env.Pool.QueryRow(base, `SELECT COUNT(*) FROM business_event_deliveries
			WHERE tenant_id = $1 AND event_id = $2 AND delivered_at IS NULL`, env.TenantID(), eventID).Scan(&n))
		return n
	}
	require.Equal(t, 2, pending())

	deliverAll := func() {
		for range 10 {
			due, _ := dispatcher.Deliver(context.Background())
			if !due {
				return
			}
		}
		t.Fatal("outbox did not drain")
	}
	deliverAll()
	require.Equal(t, 1, projection.deliveries(eventID))
	require.Zero(t, integration.deliveries(eventID))
	require.Equal(t, 1, pending(), "the failing consumer must not hold back the others")

	events := resolve[businessevent.Repository](t, env)
	stored, err := events.Get(composables.WithPool(base, env.Pool), eventID)
	require.NoError(t, err, "a consumer failure never touches stored history")
	require.Equal(t, exec.OperationID, stored.OperationID)

	failed, err := dispatcher.ListFailed(base, 10)
	require.NoError(t, err)
	require.Len(t, failed, 1)
	require.Equal(t, eventID, failed[0].EventID)
	require.Equal(t, exec.OperationID, failed[0].OperationID)
	require.Equal(t, "test.integration", failed[0].Consumer)
	require.Equal(t, 1, failed[0].Attempts)
	require.Contains(t, failed[0].LastError, "projection store is down")

	probe := dispatcher.Probe(context.Background())
	require.Equal(t, health.StatusDegraded, probe.Status)
	require.Contains(t, probe.Message, eventID.String())
	require.Contains(t, probe.Message, exec.OperationID)

	deliverAll()
	require.Zero(t, integration.deliveries(eventID), "a failed delivery waits for its backoff")

	integration.setFail(false)
	require.NoError(t, dispatcher.Retry(base, eventID, "test.integration"))
	deliverAll()
	require.Equal(t, 1, integration.deliveries(eventID))
	require.Equal(t, 1, projection.deliveries(eventID), "redelivery reaches only the consumer that failed")
	require.Zero(t, pending())
	require.ErrorIs(t, dispatcher.Retry(base, eventID, "test.integration"), businessevent.ErrDeliveryNotFound)

	redelivered, err := events.Get(composables.WithPool(base, env.Pool), eventID)
	require.NoError(t, err)
	require.Equal(t, stored, redelivered, "delivery never rewrites the actor or time of the event")
	require.Equal(t, health.StatusHealthy, dispatcher.Probe(context.Background()).Status)
}

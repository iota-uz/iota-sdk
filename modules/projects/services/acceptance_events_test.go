package services_test

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/modules/core"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/currency"
	coreservices "github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/modules/finance"
	"github.com/iota-uz/iota-sdk/modules/finance/domain/entities/counterparty"
	financeservices "github.com/iota-uz/iota-sdk/modules/finance/services"
	"github.com/iota-uz/iota-sdk/modules/projects"
	"github.com/iota-uz/iota-sdk/modules/projects/domain/aggregates/acceptance"
	"github.com/iota-uz/iota-sdk/modules/projects/domain/aggregates/project"
	"github.com/iota-uz/iota-sdk/modules/projects/services"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/defaults"
	"github.com/iota-uz/iota-sdk/pkg/itf"
	"github.com/iota-uz/iota-sdk/pkg/money"
)

type outboxRow struct {
	Subject   string
	Initiator *int64
	Payload   json.RawMessage
}

// outboxRows are the rows of the event in the transaction held by ctx, oldest
// first: the rows the transaction wrote are visible to it before they commit.
func outboxRows(t *testing.T, ctx context.Context, tenantID uuid.UUID, event string) []outboxRow {
	t.Helper()
	tx, err := composables.UseTx(ctx)
	require.NoError(t, err)
	rows, err := tx.Query(ctx, `SELECT subject_key, initiator_user_id, payload FROM extruntime.outbox_events WHERE tenant_id = $1 AND event = $2 ORDER BY seq`, tenantID, event)
	require.NoError(t, err)
	defer rows.Close()
	var out []outboxRow
	for rows.Next() {
		var r outboxRow
		require.NoError(t, rows.Scan(&r.Subject, &r.Initiator, &r.Payload))
		out = append(out, r)
	}
	require.NoError(t, rows.Err())
	return out
}

// commitsAndRollsBack runs act in a savepoint, sees the event in it, rolls the
// savepoint back and sees the event gone, then runs act in the test
// transaction itself and returns the event it published.
func commitsAndRollsBack(t *testing.T, ctx context.Context, tenantID uuid.UUID, event string, act func(ctx context.Context)) outboxRow {
	t.Helper()
	before := len(outboxRows(t, ctx, tenantID, event))
	tx, err := composables.UseTx(ctx)
	require.NoError(t, err)
	savepoint, err := tx.(pgx.Tx).Begin(ctx)
	require.NoError(t, err)
	spCtx := composables.WithTx(ctx, savepoint)
	act(spCtx)
	require.Len(t, outboxRows(t, spCtx, tenantID, event), before+1, "the event is in the transaction of the change")
	require.NoError(t, savepoint.Rollback(ctx))
	require.Len(t, outboxRows(t, ctx, tenantID, event), before, "rolling the change back takes its event with it")

	act(ctx)
	rows := outboxRows(t, ctx, tenantID, event)
	require.Len(t, rows, before+1, "committing the change keeps its event")
	return rows[len(rows)-1]
}

func TestAcceptanceEvents_SignedCommitsAndRollsBackWithTheDocument(t *testing.T) {
	env := itf.Setup(t, itf.WithComponents(
		core.NewComponent(&core.ModuleOptions{PermissionSchema: defaults.PermissionSchema()}),
		finance.NewComponent(),
		projects.NewComponent(),
	), itf.WithUser(itf.User()))
	// The acting user belongs to the tenant, as a signed-in user does.
	env.User = user.New("", "", env.User.Email(), "", user.WithID(env.User.ID()), user.WithRoles(env.User.Roles()), user.WithTenantID(env.Tenant.ID))
	env.Ctx = composables.WithUser(env.Ctx, env.User)
	ctx := env.Ctx

	require.NoError(t, itf.GetService[coreservices.CurrencyService](env).Create(ctx, &currency.CreateDTO{
		Code: string(currency.USD.Code()), Name: currency.USD.Name(), Symbol: string(currency.USD.Symbol()),
	}))
	client, err := itf.GetService[financeservices.CounterpartyService](env).Create(ctx, counterparty.New(
		"Signing Client", counterparty.Customer, counterparty.LLC, counterparty.WithTenantID(env.Tenant.ID),
	))
	require.NoError(t, err)
	p := project.New("Signing Project", client.ID(), project.WithTenantID(env.Tenant.ID))
	require.NoError(t, itf.GetService[services.ProjectService](env).Create(ctx, p))
	service := itf.GetService[services.AcceptanceService](env)
	day := time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC)
	newDraft := func(number string) acceptance.Document {
		draft, err := service.Create(ctx, acceptance.New(p.ID(), acceptance.KindAct, number, day,
			money.New(1250075, "USD"), acceptance.WithTenantID(env.Tenant.ID)))
		require.NoError(t, err)
		return draft
	}

	// Creating a draft signs nothing.
	draft := newDraft("A-1")
	require.Empty(t, outboxRows(t, ctx, env.Tenant.ID, services.AcceptanceSignedEventID))

	row := commitsAndRollsBack(t, ctx, env.Tenant.ID, services.AcceptanceSignedEventID, func(ctx context.Context) {
		signed, err := service.Sign(ctx, p.ID(), draft.ID())
		require.NoError(t, err)
		require.Equal(t, acceptance.StatusSigned, signed.Status())
	})
	require.Equal(t, draft.ID().String(), row.Subject)
	require.NotNil(t, row.Initiator)
	var payload services.AcceptanceSignedPayload
	require.NoError(t, json.Unmarshal(row.Payload, &payload))
	require.Equal(t, draft.ID(), payload.AcceptanceID)
	require.Equal(t, p.ID(), payload.ProjectID)
	require.Equal(t, "ACT", payload.Kind)
	require.Equal(t, "A-1", payload.Number)
	require.EqualValues(t, 1250075, payload.Amount.Amount)
	require.Equal(t, "USD", payload.Amount.Currency)
	require.Regexp(t, `^\d{4}-\d{2}-\d{2}$`, payload.SignedOn)

	// Cancelling a signed document announces nothing more.
	count := len(outboxRows(t, ctx, env.Tenant.ID, services.AcceptanceSignedEventID))
	_, err = service.Cancel(ctx, p.ID(), draft.ID())
	require.NoError(t, err)
	require.Len(t, outboxRows(t, ctx, env.Tenant.ID, services.AcceptanceSignedEventID), count)

	// A revision with a signing day signs the draft: the same event.
	other := newDraft("A-2")
	row = commitsAndRollsBack(t, ctx, env.Tenant.ID, services.AcceptanceSignedEventID, func(ctx context.Context) {
		signed, err := service.Update(ctx, p.ID(), other.ID(), acceptance.Revision{
			Kind: acceptance.KindAct, Number: "A-2", Date: day, SignedOn: day, Amount: money.New(1250075, "USD"),
		})
		require.NoError(t, err)
		require.Equal(t, acceptance.StatusSigned, signed.Status())
	})
	require.Equal(t, other.ID().String(), row.Subject)
	require.JSONEq(t, `{"acceptanceId":"`+other.ID().String()+`","projectId":"`+p.ID().String()+`","kind":"ACT","number":"A-2","amount":{"amount":"1250075","currency":"USD"},"signedOn":"2026-09-20"}`, string(row.Payload))
}

// Writers racing to sign one draft announce the signing once: the stored status
// is read under the row lock, so the second one finds the document signed.
func TestAcceptanceEvents_ConcurrentSigningsAnnounceOnce(t *testing.T) {
	env := itf.Setup(t, itf.WithComponents(
		core.NewComponent(&core.ModuleOptions{PermissionSchema: defaults.PermissionSchema()}),
		finance.NewComponent(),
		projects.NewComponent(),
	), itf.WithUser(itf.User()))
	ctx := env.Ctx
	require.NoError(t, itf.GetService[coreservices.CurrencyService](env).Create(ctx, &currency.CreateDTO{
		Code: string(currency.USD.Code()), Name: currency.USD.Name(), Symbol: string(currency.USD.Symbol()),
	}))
	client, err := itf.GetService[financeservices.CounterpartyService](env).Create(ctx, counterparty.New(
		"Race Client", counterparty.Customer, counterparty.LLC, counterparty.WithTenantID(env.Tenant.ID),
	))
	require.NoError(t, err)
	p := project.New("Race Project", client.ID(), project.WithTenantID(env.Tenant.ID))
	require.NoError(t, itf.GetService[services.ProjectService](env).Create(ctx, p))
	service := itf.GetService[services.AcceptanceService](env)
	day := time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC)
	newDraft := func(number string) acceptance.Document {
		draft, err := service.Create(ctx, acceptance.New(p.ID(), acceptance.KindAct, number, day,
			money.New(50000, "USD"), acceptance.WithTenantID(env.Tenant.ID)))
		require.NoError(t, err)
		return draft
	}
	twoUpdates, updateAndSign := newDraft("R-1"), newDraft("R-2")
	env.CommitTx(t)
	pool := itf.TestExecution(composables.WithTenantID(composables.WithPool(context.Background(), env.Pool), env.Tenant.ID))
	signedEvents := func(id uuid.UUID) int {
		var n int
		require.NoError(t, env.Pool.QueryRow(context.Background(),
			`SELECT count(*) FROM extruntime.outbox_events WHERE tenant_id = $1 AND event = $2 AND subject_key = $3`,
			env.Tenant.ID, services.AcceptanceSignedEventID, id.String()).Scan(&n))
		return n
	}
	revision := func(number string) acceptance.Revision {
		return acceptance.Revision{Kind: acceptance.KindAct, Number: number, Date: day, SignedOn: day, Amount: money.New(50000, "USD")}
	}
	race := func(calls ...func() error) []error {
		var wg sync.WaitGroup
		errs := make([]error, len(calls))
		for i, call := range calls {
			wg.Add(1)
			go func() {
				defer wg.Done()
				errs[i] = call()
			}()
		}
		wg.Wait()
		return errs
	}

	errs := race(
		func() error { _, err := service.Update(pool, p.ID(), twoUpdates.ID(), revision("R-1")); return err },
		func() error { _, err := service.Update(pool, p.ID(), twoUpdates.ID(), revision("R-1")); return err },
	)
	require.NoError(t, errs[0])
	require.NoError(t, errs[1])
	require.Equal(t, 1, signedEvents(twoUpdates.ID()), "two updates signing one draft announce one signing")

	errs = race(
		func() error { _, err := service.Update(pool, p.ID(), updateAndSign.ID(), revision("R-2")); return err },
		func() error { _, err := service.Sign(pool, p.ID(), updateAndSign.ID()); return err },
	)
	failed := 0
	for _, err := range errs {
		if err != nil {
			require.ErrorIs(t, err, acceptance.ErrStatus)
			failed++
		}
	}
	require.LessOrEqual(t, failed, 1)
	require.Equal(t, 1, signedEvents(updateAndSign.ID()), "an update and a sign racing on one draft announce one signing")
}

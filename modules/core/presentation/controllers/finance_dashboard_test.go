package controllers_test

import (
	"context"
	"net/url"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/modules"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/controllers"
	"github.com/iota-uz/iota-sdk/pkg/itf"
	"github.com/iota-uz/iota-sdk/pkg/lens"
	"github.com/iota-uz/iota-sdk/pkg/lens/action"
	"github.com/iota-uz/iota-sdk/pkg/lens/datasource"
	lenspostgres "github.com/iota-uz/iota-sdk/pkg/lens/postgres"
	"github.com/iota-uz/iota-sdk/pkg/lens/runtime"
)

type financeFixture struct {
	food, acme uuid.UUID
}

// seedFinance records operations around June 2026 in two currencies:
//
//	05-30 deposit   +1000.00 UZS cash (payment from Acme)     before the period
//	06-02 expense    -200.00 UZS cash (Food)
//	06-02 transfer    -50.00 UZS cash → bank, same currency
//	06-10 exchange  -1270.00 UZS cash → +0.10 USD wallet
//	06-15 deposit    +300.00 UZS cash (payment from Acme)
//	07-02 expense     -40.00 UZS cash (Food)                  after the period
func seedFinance(t *testing.T, env *itf.TestEnvironment) financeFixture {
	t.Helper()
	ctx, tx, tenant := env.Ctx, env.Tx, env.TenantID()
	exec := func(sql string, args ...any) {
		t.Helper()
		_, err := tx.Exec(ctx, sql, args...)
		require.NoError(t, err)
	}
	exec(`INSERT INTO currencies (code, name, symbol) VALUES ('UZS', 'Som', 'S'), ('USD', 'Dollar', '$') ON CONFLICT (code) DO NOTHING`)
	cash, bank, wallet := uuid.New(), uuid.New(), uuid.New()
	for _, account := range []struct {
		id       uuid.UUID
		number   string
		currency string
	}{{cash, "cash", "UZS"}, {bank, "bank", "UZS"}, {wallet, "wallet", "USD"}} {
		exec(`INSERT INTO money_accounts (id, tenant_id, name, account_number, balance, balance_currency_id) VALUES ($1, $2, $3, $3, 0, $4)`,
			account.id, tenant, account.number, account.currency)
	}
	fx := financeFixture{food: uuid.New(), acme: uuid.New()}
	exec(`INSERT INTO expense_categories (id, tenant_id, name) VALUES ($1, $2, 'Food')`, fx.food, tenant)
	exec(`INSERT INTO counterparty (id, tenant_id, name, type, legal_type) VALUES ($1, $2, 'Acme', 'CUSTOMER', 'LLC')`, fx.acme, tenant)

	transaction := func(day, kind string, amount int64, origin, destination *uuid.UUID, destinationAmount *int64) uuid.UUID {
		id := uuid.New()
		exec(`INSERT INTO transactions (id, tenant_id, amount, origin_account_id, destination_account_id, transaction_date, accounting_period, transaction_type, destination_amount)
			VALUES ($1, $2, $3, $4, $5, $6::date, $6::date, $7, $8)`, id, tenant, amount, origin, destination, day, kind, destinationAmount)
		return id
	}
	expense := func(day string, amount int64) {
		exec(`INSERT INTO expenses (tenant_id, transaction_id, category_id) VALUES ($1, $2, $3)`,
			tenant, transaction(day, "WITHDRAWAL", amount, &cash, nil, nil), fx.food)
	}
	payment := func(day string, amount int64) {
		exec(`INSERT INTO payments (tenant_id, transaction_id, counterparty_id) VALUES ($1, $2, $3)`,
			tenant, transaction(day, "DEPOSIT", amount, nil, &cash, nil), fx.acme)
	}
	cents := int64(10)
	payment("2026-05-30", 100000)
	expense("2026-06-02", 20000)
	transaction("2026-06-02", "TRANSFER", 5000, &cash, &bank, nil)
	transaction("2026-06-10", "EXCHANGE", 127000, &cash, &wallet, &cents)
	payment("2026-06-15", 30000)
	expense("2026-07-02", 4000)
	return fx
}

func juneRequest(env *itf.TestEnvironment, extra url.Values) runtime.Request {
	query := url.Values{"period-start": {"2026-06-01"}, "period-end": {"2026-06-30"}}
	for key, values := range extra {
		query[key] = values
	}
	return runtime.Request{
		Locale: "en", Timezone: "UTC", Request: query, DataScope: env.TenantID().String(),
		DataSources:          map[string]datasource.DataSource{"primary": lenspostgres.NewFromPool(env.Pool)},
		DataSourceIdentities: map[string]string{"primary": "test:" + uuid.NewString()},
	}
}

func datasetRows(t *testing.T, ctx context.Context, spec lens.DashboardSpec, req runtime.Request, name string) []map[string]any {
	t.Helper()
	result, err := runtime.New(runtime.Options{}).Execute(ctx, spec, req, runtime.DashboardScope())
	require.NoError(t, err)
	dataset, ok := result.Datasets[name]
	require.True(t, ok, name)
	require.NoError(t, dataset.Error, name)
	return dataset.Frames.Primary().Rows()
}

func rowsWhere(rows []map[string]any, match func(map[string]any) bool) []map[string]any {
	var out []map[string]any
	for _, row := range rows {
		if match(row) {
			out = append(out, row)
		}
	}
	return out
}

func number(t *testing.T, value any) float64 {
	t.Helper()
	f, ok := value.(float64)
	require.True(t, ok, "%T is not float64", value)
	return f
}

func drillTotal(t *testing.T, env *itf.TestEnvironment, query url.Values) (float64, int) {
	t.Helper()
	spec, err := controllers.FinanceOperationsSpec(env.Ctx, env.TenantID(), query)
	require.NoError(t, err)
	rows := datasetRows(t, env.Ctx, spec, juneRequest(env, query), "finance-operations")
	total := 0.0
	for _, row := range rows {
		total += number(t, row["amount"])
	}
	return total, len(rows)
}

func TestFinanceDashboard_PeriodPanelsReconcileWithOperations(t *testing.T) {
	t.Parallel()
	env := itf.Setup(t, itf.WithComponents(modules.Components()...))
	fx := seedFinance(t, env)
	spec := controllers.FinanceDashboardSpec(env.Ctx, env.TenantID())
	req := juneRequest(env, nil)

	summary := datasetRows(t, env.Ctx, spec, req, "finance-summary")
	byCurrency := map[string]map[string]any{}
	for _, row := range summary {
		byCurrency[row["currency"].(string)] = row
	}
	uzs := byCurrency["UZS"]
	require.InDelta(t, 1000.0, number(t, uzs["opening"]), 1e-9)
	require.InDelta(t, 350.0, number(t, uzs["inflow"]), 1e-9, "payment and the incoming transfer leg")
	require.InDelta(t, 1520.0, number(t, uzs["outflow"]), 1e-9)
	require.InDelta(t, -170.0, number(t, uzs["closing"]), 1e-9, "opening + inflow - outflow")
	require.InDelta(t, 0.10, number(t, byCurrency["USD"]["closing"]), 1e-9, "exchange credits the converted amount")

	daily := datasetRows(t, env.Ctx, spec, req, "finance-daily-balance")
	require.Len(t, daily, 60, "every day of June for both currencies, including days without operations")
	balance := func(day, currency string) float64 {
		matched := rowsWhere(daily, func(row map[string]any) bool { return row["day_key"] == day && row["currency"] == currency })
		require.Len(t, matched, 1)
		return number(t, matched[0]["balance"])
	}
	require.InDelta(t, 1000.0, balance("2026-06-01", "UZS"), 1e-9)
	require.InDelta(t, 800.0, balance("2026-06-02", "UZS"), 1e-9, "the same-currency transfer nets to zero")
	require.InDelta(t, number(t, uzs["closing"]), balance("2026-06-30", "UZS"), 1e-9, "the last point is the closing balance")

	// A daily point drills into the legs of that day; their sum is the day's change.
	dayTotal, dayRows := drillTotal(t, env, url.Values{"currency": {"UZS"}, "day": {"2026-06-02"}})
	require.Equal(t, 3, dayRows, "expense plus both transfer legs")
	require.InDelta(t, balance("2026-06-02", "UZS")-balance("2026-06-01", "UZS"), dayTotal, 1e-9)

	// A summary row drills into the period's legs; their sum is closing - opening.
	periodTotal, _ := drillTotal(t, env, url.Values{"currency": {"UZS"}})
	require.InDelta(t, number(t, uzs["closing"])-number(t, uzs["opening"]), periodTotal, 1e-9)

	expenses := datasetRows(t, env.Ctx, spec, req, "finance-expenses-by-category")
	require.Len(t, expenses, 1)
	require.Equal(t, fx.food.String(), expenses[0]["category_id"])
	categoryTotal, _ := drillTotal(t, env, url.Values{"currency": {"UZS"}, "category": {fx.food.String()}, "label": {"Renamed label is display only"}})
	require.InDelta(t, -number(t, expenses[0]["amount"]), categoryTotal, 1e-9, "the July expense stays outside the period")

	income := datasetRows(t, env.Ctx, spec, req, "finance-income-by-counterparty")
	require.Len(t, income, 1)
	require.Equal(t, fx.acme.String(), income[0]["counterparty_id"])
	counterpartyTotal, _ := drillTotal(t, env, url.Values{"currency": {"UZS"}, "counterparty": {fx.acme.String()}})
	require.InDelta(t, number(t, income[0]["amount"]), counterpartyTotal, 1e-9)
}

func TestFinanceDashboard_AllTimePeriodKeepsEveryOperation(t *testing.T) {
	t.Parallel()
	env := itf.Setup(t, itf.WithComponents(modules.Components()...))
	seedFinance(t, env)
	spec := controllers.FinanceDashboardSpec(env.Ctx, env.TenantID())
	req := juneRequest(env, nil)
	req.Request = url.Values{"period-start": {""}, "period-end": {""}}

	summary := datasetRows(t, env.Ctx, spec, req, "finance-summary")
	for _, row := range summary {
		if row["currency"] == "UZS" {
			require.InDelta(t, 0.0, number(t, row["opening"]), 1e-9)
			require.InDelta(t, -210.0, number(t, row["closing"]), 1e-9, "includes the July expense")
		}
	}
}

func TestFinanceDashboard_EveryPanelUsesThePeriodAndStableDrillKeys(t *testing.T) {
	t.Parallel()
	spec := controllers.FinanceDashboardSpec(context.Background(), uuid.New())

	require.Len(t, spec.Variables, 1)
	require.Equal(t, lens.VariableDateRange, spec.Variables[0].Kind)
	for _, dataset := range spec.Datasets {
		require.NotNil(t, dataset.Query, dataset.Name)
		require.Equal(t, "period", dataset.Query.Params["since"].Variable, dataset.Name)
		require.Equal(t, "period", dataset.Query.Params["until"].Variable, dataset.Name)
	}

	stableKeys := map[string]string{"currency": "currency", "day": "day_key", "category": "category_id", "counterparty": "counterparty_id", "label": "label"}
	for _, row := range spec.Rows {
		for _, p := range row.Panels {
			require.NotNil(t, p.Action, p.ID)
			require.Equal(t, action.KindOpenDrawer, p.Action.Kind, p.ID)
			require.True(t, p.Action.PreserveQuery, "%s keeps the period", p.ID)
			require.True(t, strings.HasPrefix(p.Action.URL, controllers.FinanceOperationsDocument+"?"), p.ID)
			for _, param := range p.Action.Params {
				require.Equal(t, action.SourceField, param.Source.Kind, p.ID)
				require.Equal(t, stableKeys[param.Name], param.Source.Name, "%s.%s", p.ID, param.Name)
			}
		}
	}
}

func TestFinanceOperations_RejectsMalformedDrillKeys(t *testing.T) {
	t.Parallel()
	for _, query := range []url.Values{
		{"currency": {"uzs"}},
		{"currency": {"UZS' OR 1=1"}},
		{"day": {"02.06.2026"}},
		{"category": {"food"}},
		{"counterparty": {"1"}},
	} {
		_, err := controllers.FinanceOperationsSpec(context.Background(), uuid.New(), query)
		require.Error(t, err, query.Encode())
	}
}

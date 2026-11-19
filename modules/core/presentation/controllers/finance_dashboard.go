package controllers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/iota-uz/iota-sdk/pkg/intl"
	"github.com/iota-uz/iota-sdk/pkg/lens"
	"github.com/iota-uz/iota-sdk/pkg/lens/action"
	lensbuild "github.com/iota-uz/iota-sdk/pkg/lens/build"
	"github.com/iota-uz/iota-sdk/pkg/lens/document"
	"github.com/iota-uz/iota-sdk/pkg/lens/format"
	"github.com/iota-uz/iota-sdk/pkg/lens/panel"
	"github.com/iota-uz/iota-sdk/pkg/lens/runtime"
)

// The finance dashboard shares one period across every panel and opens the
// account movements behind any mark in a drawer. Both documents read the same
// movement legs, so a panel value and the drawer total reconcile by
// construction.

const (
	financePeriodVariable       = "period"
	financePeriodDefault        = 30 * 24 * time.Hour
	financeOperationsBasePath   = dashboardLensBasePath + "/operations"
	financeOperationsDocument   = financeOperationsBasePath + "/document"
	financeOperationsTitleLimit = 120
	financeTopRows              = 15
	financeRecentRows           = 20
)

// Drill parameters carry stable keys only: a currency code, an ISO day, and
// UUIDs. The optional label is shown in the drawer header and never filters.
const (
	drillParamCurrency     = "currency"
	drillParamDay          = "day"
	drillParamCategory     = "category"
	drillParamCounterparty = "counterparty"
	drillParamLabel        = "label"
)

var currencyCodePattern = regexp.MustCompile(`^[A-Z]{3}$`)

// financeLegsCTE expands every transaction into its account legs: the origin
// account loses the amount and the destination gains it (the converted amount
// for an exchange). This is the same rule money account balances are
// recalculated with, so summing legs up to a day gives that day's balance.
const financeLegsCTE = `legs AS (
	SELECT t.id AS transaction_id, t.transaction_date AS day, t.transaction_type,
		ma.id AS account_id, ma.name AS account_name, ma.balance_currency_id AS currency,
		-t.amount AS delta, t.comment, t.created_at
	FROM transactions t
	JOIN money_accounts ma ON ma.id = t.origin_account_id AND ma.tenant_id = t.tenant_id
	WHERE t.tenant_id = @tenant_id
	UNION ALL
	SELECT t.id, t.transaction_date, t.transaction_type,
		ma.id, ma.name, ma.balance_currency_id,
		CASE WHEN t.transaction_type = 'EXCHANGE' AND t.destination_amount IS NOT NULL
			THEN t.destination_amount ELSE t.amount END,
		t.comment, t.created_at
	FROM transactions t
	JOIN money_accounts ma ON ma.id = t.destination_account_id AND ma.tenant_id = t.tenant_id
	WHERE t.tenant_id = @tenant_id
)`

// financeInPeriod is true for a leg dated inside the selected period. An
// all-time period binds NULL bounds and keeps every leg.
const financeInPeriod = `(@since::date IS NULL OR l.day >= @since::date)
	AND (@until::date IS NULL OR l.day < @until::date)`

type financeDashboardText struct {
	Title, Period                                                       string
	Summary, Currency, Opening, Inflow, Outflow, Closing, Operations    string
	DailyBalance, ExpensesByCategory, IncomeByCounterparty, RecentOps   string
	Date, Type, Account, Counterpart, Comment, Amount, OperationsDrawer string
	TransactionTypes                                                    map[string]string
}

func financeText(ctx context.Context) financeDashboardText {
	t := func(key, fallback string) string {
		return intl.TWithDefault(ctx, "Dashboard.Finance."+key, fallback)
	}
	types := make(map[string]string, 5)
	for _, kind := range []string{"DEPOSIT", "WITHDRAWAL", "TRANSFER", "EXCHANGE", "ADJUSTMENT"} {
		types[kind] = intl.TWithDefault(ctx, "Transactions.Types."+kind, kind)
	}
	return financeDashboardText{
		Title:                t("Title", "Finance overview"),
		Period:               t("Period", "Period"),
		Summary:              t("Summary", "Balances by currency"),
		Currency:             t("Currency", "Currency"),
		Opening:              t("Opening", "Opening balance"),
		Inflow:               t("Inflow", "Inflow"),
		Outflow:              t("Outflow", "Outflow"),
		Closing:              t("Closing", "Closing balance"),
		Operations:           t("Operations", "Operations"),
		DailyBalance:         t("DailyBalance", "Daily balance"),
		ExpensesByCategory:   t("ExpensesByCategory", "Expenses by category"),
		IncomeByCounterparty: t("IncomeByCounterparty", "Income by counterparty"),
		RecentOps:            t("RecentOperations", "Recent operations"),
		Date:                 t("Date", "Date"),
		Type:                 t("Type", "Type"),
		Account:              t("Account", "Account"),
		Counterpart:          t("Counterpart", "Category / counterparty"),
		Comment:              t("Comment", "Comment"),
		Amount:               t("Amount", "Amount"),
		OperationsDrawer:     t("OperationsDrawer", "Account operations"),
		TransactionTypes:     types,
	}
}

func financePeriodParams(tenantID uuid.UUID) map[string]lens.ParamValue {
	return map[string]lens.ParamValue{
		"tenant_id": {Literal: tenantID},
		"since":     lens.RangeParam(financePeriodVariable, lens.RangeStart),
		"until":     lens.RangeParam(financePeriodVariable, lens.RangeUntil),
	}
}

func financeQuery(name, text string, params map[string]lens.ParamValue) lens.DatasetSpec {
	spec := lensbuild.QueryDataset(name, "primary", text)
	if spec.Query != nil {
		spec.Query.Params = params
	}
	return spec
}

// operationsAction opens the operations drawer for one mark. The URL template
// names only the stable keys of that mark; the dashboard query (the period)
// is carried along so the drawer lists the same slice the mark summarizes.
func operationsAction(params ...action.Param) action.Spec {
	query := make([]string, 0, len(params))
	for _, param := range params {
		query = append(query, param.Name+"={"+param.Name+"}")
	}
	return action.OpenDrawer(financeOperationsDocument+"?"+strings.Join(query, "&"), params...).WithPreservedQuery()
}

func financeDashboardSpec(tenantID uuid.UUID, text financeDashboardText) lens.DashboardSpec {
	params := financePeriodParams(tenantID)
	amount := format.NumberCompact()
	return lensbuild.Dashboard("finance-overview", text.Title,
		lensbuild.Row(
			panel.Table("finance-summary", text.Summary, "finance-summary").
				IDField(panel.Ref("currency")).
				Columns(
					panel.TableColumn{Field: panel.Ref("currency"), Label: text.Currency},
					panel.TableColumn{Field: panel.Ref("opening"), Label: text.Opening, Formatter: &amount, Align: "right"},
					panel.TableColumn{Field: panel.Ref("inflow"), Label: text.Inflow, Formatter: &amount, Align: "right"},
					panel.TableColumn{Field: panel.Ref("outflow"), Label: text.Outflow, Formatter: &amount, Align: "right"},
					panel.TableColumn{Field: panel.Ref("closing"), Label: text.Closing, Formatter: &amount, Align: "right"},
					panel.TableColumn{Field: panel.Ref("operations"), Label: text.Operations, Align: "right"},
				).
				Action(operationsAction(action.FieldParam(drillParamCurrency, "currency"))).
				Span(12).Build(),
		),
		lensbuild.Row(
			panel.TimeSeries("finance-daily-balance", text.DailyBalance, "finance-daily-balance").
				CategoryField(panel.Ref("day")).
				ValueField(panel.Ref("balance")).
				SeriesField(panel.Ref("currency")).
				Format(amount).
				Legend().
				Action(operationsAction(
					action.FieldParam(drillParamCurrency, "currency"),
					action.FieldParam(drillParamDay, "day_key"),
				)).
				Span(12).Build(),
		),
		lensbuild.Row(
			panel.HorizontalBar("finance-expenses-by-category", text.ExpensesByCategory, "finance-expenses-by-category").
				IDField(panel.Ref("key")).
				LabelField(panel.Ref("label")).
				ValueField(panel.Ref("amount")).
				Format(amount).
				Action(operationsAction(
					action.FieldParam(drillParamCurrency, "currency"),
					action.FieldParam(drillParamCategory, "category_id"),
					action.FieldParam(drillParamLabel, "label"),
				)).
				Span(6).Build(),
			panel.HorizontalBar("finance-income-by-counterparty", text.IncomeByCounterparty, "finance-income-by-counterparty").
				IDField(panel.Ref("key")).
				LabelField(panel.Ref("label")).
				ValueField(panel.Ref("amount")).
				Format(amount).
				Action(operationsAction(
					action.FieldParam(drillParamCurrency, "currency"),
					action.FieldParam(drillParamCounterparty, "counterparty_id"),
					action.FieldParam(drillParamLabel, "label"),
				)).
				Span(6).Build(),
		),
		lensbuild.Row(
			operationsTable("finance-recent-operations", text.RecentOps, "finance-recent-operations", text, "", false).
				Action(operationsAction(
					action.FieldParam(drillParamCurrency, "currency"),
					action.FieldParam(drillParamDay, "day_key"),
				)).
				Span(12).Build(),
		),
	).Variables(
		lensbuild.DateRangeVariable(financePeriodVariable, text.Period, financePeriodDefault),
	).Datasets(
		financeQuery("finance-summary", `WITH `+financeLegsCTE+`,
			currencies AS (
				SELECT DISTINCT balance_currency_id AS currency FROM money_accounts WHERE tenant_id = @tenant_id
			)
			SELECT c.currency,
				(COALESCE(SUM(l.delta) FILTER (WHERE @since::date IS NOT NULL AND l.day < @since::date), 0) / 100.0)::float8 AS opening,
				(COALESCE(SUM(l.delta) FILTER (WHERE l.delta > 0 AND `+financeInPeriod+`), 0) / 100.0)::float8 AS inflow,
				(COALESCE(-SUM(l.delta) FILTER (WHERE l.delta < 0 AND `+financeInPeriod+`), 0) / 100.0)::float8 AS outflow,
				(COALESCE(SUM(l.delta) FILTER (WHERE @until::date IS NULL OR l.day < @until::date), 0) / 100.0)::float8 AS closing,
				COUNT(DISTINCT l.transaction_id) FILTER (WHERE `+financeInPeriod+`)::float8 AS operations
			FROM currencies c
			LEFT JOIN legs l ON l.currency = c.currency
			GROUP BY c.currency
			ORDER BY c.currency`, params),
		financeQuery("finance-daily-balance", `WITH `+financeLegsCTE+`,
			bounds AS (
				SELECT COALESCE(@since::date, (SELECT MIN(day) FROM legs)) AS start_day,
					COALESCE(@until::date - 1, GREATEST((SELECT MAX(day) FROM legs), CURRENT_DATE)) AS end_day
			),
			days AS (
				SELECT generate_series(b.start_day, b.end_day, INTERVAL '1 day')::date AS day
				FROM bounds b WHERE b.start_day IS NOT NULL AND b.start_day <= b.end_day
			),
			currencies AS (
				SELECT DISTINCT balance_currency_id AS currency FROM money_accounts WHERE tenant_id = @tenant_id
			),
			opening AS (
				SELECT c.currency, COALESCE(SUM(l.delta), 0) AS amount
				FROM currencies c
				LEFT JOIN legs l ON l.currency = c.currency AND l.day < (SELECT start_day FROM bounds)
				GROUP BY c.currency
			),
			daily AS (
				SELECT d.day, c.currency, COALESCE(SUM(l.delta), 0) AS delta
				FROM days d
				CROSS JOIN currencies c
				LEFT JOIN legs l ON l.day = d.day AND l.currency = c.currency
				GROUP BY d.day, c.currency
			)
			SELECT daily.day, TO_CHAR(daily.day, 'YYYY-MM-DD') AS day_key, daily.currency,
				((o.amount + SUM(daily.delta) OVER (PARTITION BY daily.currency ORDER BY daily.day)) / 100.0)::float8 AS balance
			FROM daily
			JOIN opening o ON o.currency = daily.currency
			ORDER BY daily.day, daily.currency`, params),
		financeQuery("finance-expenses-by-category", `WITH `+financeLegsCTE+`
			SELECT ec.id::text || ':' || l.currency AS key,
				ec.id::text AS category_id,
				l.currency,
				ec.name || ' · ' || l.currency AS label,
				(-SUM(l.delta) / 100.0)::float8 AS amount
			FROM legs l
			JOIN expenses e ON e.transaction_id = l.transaction_id AND e.tenant_id = @tenant_id
			JOIN expense_categories ec ON ec.id = e.category_id AND ec.tenant_id = @tenant_id
			WHERE l.delta < 0 AND `+financeInPeriod+`
			GROUP BY ec.id, ec.name, l.currency
			ORDER BY amount DESC
			LIMIT `+fmt.Sprint(financeTopRows), params),
		financeQuery("finance-income-by-counterparty", `WITH `+financeLegsCTE+`
			SELECT cp.id::text || ':' || l.currency AS key,
				cp.id::text AS counterparty_id,
				l.currency,
				cp.name || ' · ' || l.currency AS label,
				(SUM(l.delta) / 100.0)::float8 AS amount
			FROM legs l
			JOIN payments p ON p.transaction_id = l.transaction_id AND p.tenant_id = @tenant_id
			JOIN counterparty cp ON cp.id = p.counterparty_id AND cp.tenant_id = @tenant_id
			WHERE l.delta > 0 AND `+financeInPeriod+`
			GROUP BY cp.id, cp.name, l.currency
			ORDER BY amount DESC
			LIMIT `+fmt.Sprint(financeTopRows), params),
		financeQuery("finance-recent-operations", operationsQuery(financeRecentRows), operationsParams(params, operationsDrill{}, text)),
	).Build()
}

// operationsDrill is the slice the operations drawer lists. Zero fields do
// not filter.
type operationsDrill struct {
	Currency     string
	Day          *time.Time
	Category     *uuid.UUID
	Counterparty *uuid.UUID
	Label        string
}

func parseOperationsDrill(values url.Values) (operationsDrill, error) {
	var drill operationsDrill
	if raw := strings.TrimSpace(values.Get(drillParamCurrency)); raw != "" {
		if !currencyCodePattern.MatchString(raw) {
			return drill, fmt.Errorf("invalid currency %q", raw)
		}
		drill.Currency = raw
	}
	if raw := strings.TrimSpace(values.Get(drillParamDay)); raw != "" {
		day, err := time.Parse(time.DateOnly, raw)
		if err != nil {
			return drill, fmt.Errorf("invalid day %q", raw)
		}
		drill.Day = &day
	}
	parseID := func(name string) (*uuid.UUID, error) {
		raw := strings.TrimSpace(values.Get(name))
		if raw == "" {
			return nil, nil //nolint:nilnil // an absent key means the drill does not filter by it
		}
		id, err := uuid.Parse(raw)
		if err != nil {
			return nil, fmt.Errorf("invalid %s %q", name, raw)
		}
		return &id, nil
	}
	var err error
	if drill.Category, err = parseID(drillParamCategory); err != nil {
		return drill, err
	}
	if drill.Counterparty, err = parseID(drillParamCounterparty); err != nil {
		return drill, err
	}
	label := []rune(strings.TrimSpace(values.Get(drillParamLabel)))
	if len(label) > financeOperationsTitleLimit {
		label = label[:financeOperationsTitleLimit]
	}
	drill.Label = string(label)
	return drill, nil
}

func operationsParams(base map[string]lens.ParamValue, drill operationsDrill, text financeDashboardText) map[string]lens.ParamValue {
	params := make(map[string]lens.ParamValue, len(base)+5)
	for key, value := range base {
		params[key] = value
	}
	labels, err := json.Marshal(text.TransactionTypes)
	if err != nil {
		labels = []byte("{}")
	}
	params["type_labels"] = lens.ParamValue{Literal: string(labels)}
	params["currency"] = lens.ParamValue{Literal: nil}
	params["day"] = lens.ParamValue{Literal: nil}
	params["category"] = lens.ParamValue{Literal: nil}
	params["counterparty"] = lens.ParamValue{Literal: nil}
	if drill.Currency != "" {
		params["currency"] = lens.ParamValue{Literal: drill.Currency}
	}
	if drill.Day != nil {
		params["day"] = lens.ParamValue{Literal: drill.Day.Format(time.DateOnly)}
	}
	if drill.Category != nil {
		params["category"] = lens.ParamValue{Literal: drill.Category.String()}
	}
	if drill.Counterparty != nil {
		params["counterparty"] = lens.ParamValue{Literal: drill.Counterparty.String()}
	}
	return params
}

// operationsSelect lists account legs inside the period, narrowed by the
// drill keys. The type is filtered and keyed by its code; @type_labels only
// supplies the localized caption. A category keeps only the expense (outgoing)
// leg of its transactions and a counterparty only the incoming payment leg,
// matching how the dashboard panels aggregate them.
func operationsSelect() string {
	return `WITH ` + financeLegsCTE + `
		SELECT l.transaction_id::text || ':' || l.account_id::text AS id,
			l.day, TO_CHAR(l.day, 'YYYY-MM-DD') AS day_key, l.transaction_type,
			COALESCE(@type_labels::jsonb ->> l.transaction_type, l.transaction_type) AS type_label,
			l.account_name, l.currency,
			CASE
				WHEN l.delta < 0 AND ec.id IS NOT NULL THEN ec.name
				WHEN l.delta > 0 AND cp.id IS NOT NULL THEN cp.name
				ELSE ''
			END AS counterpart,
			COALESCE(l.comment, '') AS comment,
			(l.delta / 100.0)::float8 AS amount
		FROM legs l
		LEFT JOIN expenses e ON e.transaction_id = l.transaction_id AND e.tenant_id = @tenant_id
		LEFT JOIN expense_categories ec ON ec.id = e.category_id AND ec.tenant_id = @tenant_id
		LEFT JOIN payments p ON p.transaction_id = l.transaction_id AND p.tenant_id = @tenant_id
		LEFT JOIN counterparty cp ON cp.id = p.counterparty_id AND cp.tenant_id = @tenant_id
		WHERE ` + financeInPeriod + `
			AND (@currency::text IS NULL OR l.currency = @currency::text)
			AND (@day::date IS NULL OR l.day = @day::date)
			AND (@category::uuid IS NULL OR (l.delta < 0 AND e.category_id = @category::uuid))
			AND (@counterparty::uuid IS NULL OR (l.delta > 0 AND p.counterparty_id = @counterparty::uuid))
		ORDER BY l.day DESC, l.created_at DESC, l.transaction_id, l.account_id`
}

// operationsQuery bounds operationsSelect for the recent-operations panel. The
// drawer passes 0 and lists the complete slice, so its footer total reconciles
// with the mark it was opened from.
func operationsQuery(limit int) string {
	if limit <= 0 {
		return operationsSelect()
	}
	return operationsSelect() + `
		LIMIT ` + fmt.Sprint(limit)
}

func operationsTable(id, title, dataset string, text financeDashboardText, currency string, totals bool) *panel.Builder {
	day := format.Date("2006-01-02")
	amount := format.NumberCompact()
	if currency != "" {
		amount = format.Money(currency, 2)
	}
	amountColumn := panel.TableColumn{Field: panel.Ref("amount"), Label: text.Amount, Formatter: &amount, Align: "right"}
	if totals {
		amountColumn = amountColumn.WithTotal()
	}
	columns := []panel.TableColumn{
		{Field: panel.Ref("day"), Label: text.Date, Formatter: &day},
		{Field: panel.Ref("type_label"), Label: text.Type},
		{Field: panel.Ref("account_name"), Label: text.Account},
	}
	if currency == "" {
		columns = append(columns, panel.TableColumn{Field: panel.Ref("currency"), Label: text.Currency})
	}
	columns = append(columns,
		panel.TableColumn{Field: panel.Ref("counterpart"), Label: text.Counterpart},
		panel.TableColumn{Field: panel.Ref("comment"), Label: text.Comment}.Clamp(2),
		amountColumn,
	)
	return panel.Table(id, title, dataset).IDField(panel.Ref("id")).Columns(columns...)
}

// financeOperationsSpec is the drawer document behind every dashboard mark.
// It declares the same period variable, so the preserved dashboard query
// binds it and the drawer shows the period it lists.
func financeOperationsSpec(tenantID uuid.UUID, drill operationsDrill, text financeDashboardText) lens.DashboardSpec {
	params := operationsParams(financePeriodParams(tenantID), drill, text)
	return lensbuild.Dashboard("finance-operations", text.OperationsDrawer,
		lensbuild.Row(
			operationsTable("finance-operations", text.OperationsDrawer, "finance-operations", text, drill.Currency, drill.Currency != "").
				Terminal().Span(12).Build(),
		),
	).Variables(
		lensbuild.DateRangeVariable(financePeriodVariable, text.Period, financePeriodDefault),
	).Datasets(
		financeQuery("finance-operations", operationsQuery(0), params),
	).Build()
}

// operationsDrawerTitle names the drawer's slice from its keys and the
// display-only label; it never reads a filter from the label.
func operationsDrawerTitle(drill operationsDrill) string {
	parts := make([]string, 0, 3)
	if drill.Label != "" {
		parts = append(parts, drill.Label)
	} else if drill.Currency != "" {
		parts = append(parts, drill.Currency)
	}
	if drill.Day != nil {
		parts = append(parts, drill.Day.Format("02.01.2006"))
	}
	return strings.Join(parts, " · ")
}

// operationsDrawerCaption states the period the drawer lists, as resolved for
// this request, so the slice is visible without reading the URL.
func operationsDrawerCaption(spec lens.DashboardSpec, req runtime.Request) string {
	values, err := runtime.ResolveVariables(spec.Variables, req)
	if err != nil {
		return ""
	}
	start, okStart := lens.ResolveRangePart(values[financePeriodVariable], lens.RangeStart)
	end, okEnd := lens.ResolveRangePart(values[financePeriodVariable], lens.RangeEnd)
	from, fromSet := start.(time.Time)
	to, toSet := end.(time.Time)
	if !okStart || !okEnd || !fromSet || !toSet {
		return ""
	}
	return from.Format("02.01.2006") + " – " + to.Format("02.01.2006")
}

// operationsDrillQuery is the canonical query string of a drill. The drawer
// advertises its query, export and release endpoints with it, so every
// follow-up request rebuilds the same filtered spec the document was built
// from instead of an unfiltered one.
func operationsDrillQuery(drill operationsDrill) string {
	values := url.Values{}
	if drill.Currency != "" {
		values.Set(drillParamCurrency, drill.Currency)
	}
	if drill.Day != nil {
		values.Set(drillParamDay, drill.Day.Format(time.DateOnly))
	}
	if drill.Category != nil {
		values.Set(drillParamCategory, drill.Category.String())
	}
	if drill.Counterparty != nil {
		values.Set(drillParamCounterparty, drill.Counterparty.String())
	}
	if drill.Label != "" {
		values.Set(drillParamLabel, drill.Label)
	}
	return values.Encode()
}

func operationsEndpoints(drill operationsDrill) document.Endpoints {
	suffix := ""
	if query := operationsDrillQuery(drill); query != "" {
		suffix = "?" + query
	}
	return document.Endpoints{
		Query:   financeOperationsBasePath + "/lens/query" + suffix,
		Export:  financeOperationsBasePath + "/export" + suffix,
		Release: financeOperationsBasePath + "/lens/release" + suffix,
	}
}

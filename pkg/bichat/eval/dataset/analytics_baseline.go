package dataset

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const AnalyticsBaselineV1ID = "analytics_baseline_v1"

type analyticsBaselineDataset struct{}

func newAnalyticsBaselineDataset() Dataset {
	return &analyticsBaselineDataset{}
}

func (d *analyticsBaselineDataset) ID() string {
	return AnalyticsBaselineV1ID
}

func (d *analyticsBaselineDataset) Seed(ctx context.Context, tx pgx.Tx, tenantID uuid.UUID) error {
	marker := datasetMarker(d.ID())
	now := time.Date(2025, time.January, 1, 12, 0, 0, 0, time.UTC)

	if _, err := tx.Exec(ctx, `
		INSERT INTO tenants (id, name, domain, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, TRUE, NOW(), NOW())
		ON CONFLICT (id) DO NOTHING
	`, tenantID, "Eval Tenant - "+d.ID(), d.ID()+".eval.local"); err != nil {
		return fmt.Errorf("seed tenant: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO currencies (code, name, symbol, created_at, updated_at)
		VALUES ('USD', 'US Dollar', '$', NOW(), NOW())
		ON CONFLICT (code) DO NOTHING
	`); err != nil {
		return fmt.Errorf("seed currency: %w", err)
	}

	cpAcme := deterministicID(d.ID(), "counterparty-acme")
	cpBeta := deterministicID(d.ID(), "counterparty-beta")
	cpNorthwind := deterministicID(d.ID(), "counterparty-northwind")

	counterparties := []struct {
		ID   uuid.UUID
		Tin  string
		Name string
	}{
		{ID: cpAcme, Tin: "900000000001", Name: "Acme Retail"},
		{ID: cpBeta, Tin: "900000000002", Name: "Beta Supplies"},
		{ID: cpNorthwind, Tin: "900000000003", Name: "Northwind Partners"},
	}

	for _, cp := range counterparties {
		if _, err := tx.Exec(ctx, `
			INSERT INTO counterparty (id, tenant_id, tin, name, type, legal_type, legal_address, created_at, updated_at)
			VALUES ($1, $2, $3, $4, 'COMPANY', 'LLC', 'Eval Street 1', NOW(), NOW())
			ON CONFLICT (id) DO UPDATE
			SET name = EXCLUDED.name,
				tenant_id = EXCLUDED.tenant_id,
				tin = EXCLUDED.tin,
				updated_at = NOW()
		`, cp.ID, tenantID, cp.Tin, cp.Name); err != nil {
			return fmt.Errorf("seed counterparty %s: %w", cp.Name, err)
		}
	}

	mainAccountID := deterministicID(d.ID(), "money-account-main")
	cashAccountID := deterministicID(d.ID(), "money-account-cash")

	accounts := []struct {
		ID            uuid.UUID
		Name          string
		AccountNumber string
		Balance       int64
	}{
		{ID: mainAccountID, Name: "Eval Main Bank", AccountNumber: "EVAL-MAIN-001", Balance: 100_000_000},
		{ID: cashAccountID, Name: "Eval Cash", AccountNumber: "EVAL-CASH-001", Balance: 10_000_000},
	}

	for _, acc := range accounts {
		if _, err := tx.Exec(ctx, `
			INSERT INTO money_accounts (
				id, tenant_id, name, account_number, description, balance, balance_currency_id, created_at, updated_at
			)
			VALUES ($1, $2, $3, $4, $5, $6, 'USD', NOW(), NOW())
			ON CONFLICT (id) DO UPDATE
			SET tenant_id = EXCLUDED.tenant_id,
				name = EXCLUDED.name,
				account_number = EXCLUDED.account_number,
				description = EXCLUDED.description,
				balance = EXCLUDED.balance,
				updated_at = NOW()
		`, acc.ID, tenantID, acc.Name, acc.AccountNumber, "Seeded by "+marker, acc.Balance); err != nil {
			return fmt.Errorf("seed money account %s: %w", acc.Name, err)
		}
	}

	paymentCategoryID := deterministicID(d.ID(), "payment-category-sales")
	if _, err := tx.Exec(ctx, `
		INSERT INTO payment_categories (id, tenant_id, name, description, created_at, updated_at)
		VALUES ($1, $2, 'Sales Income', $3, NOW(), NOW())
		ON CONFLICT (id) DO UPDATE
		SET tenant_id = EXCLUDED.tenant_id,
			name = EXCLUDED.name,
			description = EXCLUDED.description,
			updated_at = NOW()
	`, paymentCategoryID, tenantID, "Seeded by "+marker); err != nil {
		return fmt.Errorf("seed payment category: %w", err)
	}

	expenseCategoryID := deterministicID(d.ID(), "expense-category-ops")
	if _, err := tx.Exec(ctx, `
		INSERT INTO expense_categories (id, tenant_id, name, description, is_cogs, created_at, updated_at)
		VALUES ($1, $2, 'Operations', $3, FALSE, NOW(), NOW())
		ON CONFLICT (id) DO UPDATE
		SET tenant_id = EXCLUDED.tenant_id,
			name = EXCLUDED.name,
			description = EXCLUDED.description,
			is_cogs = EXCLUDED.is_cogs,
			updated_at = NOW()
	`, expenseCategoryID, tenantID, "Seeded by "+marker); err != nil {
		return fmt.Errorf("seed expense category: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO inventory (id, tenant_id, name, description, currency_id, price, quantity, created_at, updated_at)
		VALUES
			($1, $2, 'EVAL_WIDGET_A', $3, 'USD', 2500, 40, NOW(), NOW()),
			($4, $2, 'EVAL_WIDGET_B', $3, 'USD', 3000, 15, NOW(), NOW()),
			($5, $2, 'EVAL_GADGET_C', $3, 'USD', 4500, 15, NOW(), NOW())
		ON CONFLICT (id) DO UPDATE
		SET tenant_id = EXCLUDED.tenant_id,
			description = EXCLUDED.description,
			price = EXCLUDED.price,
			quantity = EXCLUDED.quantity,
			updated_at = NOW()
	`,
		deterministicID(d.ID(), "inventory-widget-a"), tenantID, "Seeded by "+marker,
		deterministicID(d.ID(), "inventory-widget-b"),
		deterministicID(d.ID(), "inventory-gadget-c"),
	); err != nil {
		return fmt.Errorf("seed inventory: %w", err)
	}

	paymentTransactions := []struct {
		ID           uuid.UUID
		Counterparty uuid.UUID
		Amount       int64
		Date         time.Time
	}{
		{ID: deterministicID(d.ID(), "tx-payment-1"), Counterparty: cpAcme, Amount: 120_000, Date: time.Date(2025, 1, 5, 0, 0, 0, 0, time.UTC)},
		{ID: deterministicID(d.ID(), "tx-payment-2"), Counterparty: cpAcme, Amount: 180_000, Date: time.Date(2025, 2, 15, 0, 0, 0, 0, time.UTC)},
		{ID: deterministicID(d.ID(), "tx-payment-3"), Counterparty: cpBeta, Amount: 150_000, Date: time.Date(2025, 3, 20, 0, 0, 0, 0, time.UTC)},
		{ID: deterministicID(d.ID(), "tx-payment-4"), Counterparty: cpNorthwind, Amount: 150_000, Date: time.Date(2025, 3, 25, 0, 0, 0, 0, time.UTC)},
		{ID: deterministicID(d.ID(), "tx-payment-5"), Counterparty: cpAcme, Amount: 90_000, Date: time.Date(2025, 4, 10, 0, 0, 0, 0, time.UTC)},
	}

	for _, txItem := range paymentTransactions {
		comment := fmt.Sprintf("%s|payment", marker)
		if _, err := tx.Exec(ctx, `
			INSERT INTO transactions (
				id, tenant_id, amount, origin_account_id, destination_account_id,
				transaction_date, accounting_period, transaction_type, comment,
				exchange_rate, destination_amount, created_at
			)
			VALUES ($1, $2, $3, NULL, $4, $5, $5, 'INCOME', $6, NULL, NULL, $7)
			ON CONFLICT (id) DO UPDATE
			SET tenant_id = EXCLUDED.tenant_id,
				amount = EXCLUDED.amount,
				destination_account_id = EXCLUDED.destination_account_id,
				transaction_date = EXCLUDED.transaction_date,
				accounting_period = EXCLUDED.accounting_period,
				transaction_type = EXCLUDED.transaction_type,
				comment = EXCLUDED.comment
		`, txItem.ID, tenantID, txItem.Amount, mainAccountID, txItem.Date, comment, now); err != nil {
			return fmt.Errorf("seed payment transaction %s: %w", txItem.ID, err)
		}

		paymentID := deterministicID(d.ID(), "payment-"+txItem.ID.String())
		if _, err := tx.Exec(ctx, `
			INSERT INTO payments (
				id, transaction_id, counterparty_id, payment_category_id, tenant_id, created_at, updated_at
			)
			VALUES ($1, $2, $3, $4, $5, $6, $6)
			ON CONFLICT (id) DO UPDATE
			SET transaction_id = EXCLUDED.transaction_id,
				counterparty_id = EXCLUDED.counterparty_id,
				payment_category_id = EXCLUDED.payment_category_id,
				tenant_id = EXCLUDED.tenant_id,
				updated_at = NOW()
		`, paymentID, txItem.ID, txItem.Counterparty, paymentCategoryID, tenantID, now); err != nil {
			return fmt.Errorf("seed payment %s: %w", paymentID, err)
		}
	}

	expenseTransactions := []struct {
		ID     uuid.UUID
		Amount int64
		Date   time.Time
	}{
		{ID: deterministicID(d.ID(), "tx-expense-1"), Amount: 50_000, Date: time.Date(2025, 1, 12, 0, 0, 0, 0, time.UTC)},
		{ID: deterministicID(d.ID(), "tx-expense-2"), Amount: 70_000, Date: time.Date(2025, 2, 18, 0, 0, 0, 0, time.UTC)},
		{ID: deterministicID(d.ID(), "tx-expense-3"), Amount: 40_000, Date: time.Date(2025, 3, 22, 0, 0, 0, 0, time.UTC)},
		{ID: deterministicID(d.ID(), "tx-expense-4"), Amount: 60_000, Date: time.Date(2025, 4, 2, 0, 0, 0, 0, time.UTC)},
	}

	for _, txItem := range expenseTransactions {
		comment := fmt.Sprintf("%s|expense", marker)
		if _, err := tx.Exec(ctx, `
			INSERT INTO transactions (
				id, tenant_id, amount, origin_account_id, destination_account_id,
				transaction_date, accounting_period, transaction_type, comment,
				exchange_rate, destination_amount, created_at
			)
			VALUES ($1, $2, $3, $4, NULL, $5, $5, 'EXPENSE', $6, NULL, NULL, $7)
			ON CONFLICT (id) DO UPDATE
			SET tenant_id = EXCLUDED.tenant_id,
				amount = EXCLUDED.amount,
				origin_account_id = EXCLUDED.origin_account_id,
				transaction_date = EXCLUDED.transaction_date,
				accounting_period = EXCLUDED.accounting_period,
				transaction_type = EXCLUDED.transaction_type,
				comment = EXCLUDED.comment
		`, txItem.ID, tenantID, txItem.Amount, mainAccountID, txItem.Date, comment, now); err != nil {
			return fmt.Errorf("seed expense transaction %s: %w", txItem.ID, err)
		}

		expenseID := deterministicID(d.ID(), "expense-"+txItem.ID.String())
		if _, err := tx.Exec(ctx, `
			INSERT INTO expenses (
				id, transaction_id, category_id, tenant_id, created_at, updated_at
			)
			VALUES ($1, $2, $3, $4, $5, $5)
			ON CONFLICT (id) DO UPDATE
			SET transaction_id = EXCLUDED.transaction_id,
				category_id = EXCLUDED.category_id,
				tenant_id = EXCLUDED.tenant_id,
				updated_at = NOW()
		`, expenseID, txItem.ID, expenseCategoryID, tenantID, now); err != nil {
			return fmt.Errorf("seed expense %s: %w", expenseID, err)
		}
	}

	return nil
}

func (d *analyticsBaselineDataset) BuildOracle(ctx context.Context, tx pgx.Tx, tenantID uuid.UUID) (map[string]Fact, error) {
	marker := datasetMarker(d.ID()) + "%"
	facts := make(map[string]Fact)

	acmeTotal, acmeCount, err := queryAcmeQ1(ctx, tx, tenantID, marker)
	if err != nil {
		return nil, err
	}
	facts[factKey(d.ID(), "acme_q1_total_minor")] = Fact{
		Key:           factKey(d.ID(), "acme_q1_total_minor"),
		Description:   "Total incoming payment amount from Acme in Q1 2025 (minor currency units)",
		ExpectedValue: fmt.Sprintf("%d", acmeTotal),
		ValueType:     "number",
		Normalization: "currency_minor_units",
	}
	facts[factKey(d.ID(), "acme_q1_tx_count")] = Fact{
		Key:           factKey(d.ID(), "acme_q1_tx_count"),
		Description:   "Incoming payment transaction count from Acme in Q1 2025",
		ExpectedValue: fmt.Sprintf("%d", acmeCount),
		ValueType:     "number",
	}

	topRows, err := queryTopCounterpartiesQ1(ctx, tx, tenantID, marker)
	if err != nil {
		return nil, err
	}
	if len(topRows) > 0 {
		facts[factKey(d.ID(), "q1_top_counterparty")] = Fact{
			Key:           factKey(d.ID(), "q1_top_counterparty"),
			Description:   "Top counterparty by incoming payments in Q1 2025",
			ExpectedValue: topRows[0].Name,
			ValueType:     "entity",
		}
		facts[factKey(d.ID(), "q1_top_amount_minor")] = Fact{
			Key:           factKey(d.ID(), "q1_top_amount_minor"),
			Description:   "Incoming payment amount for top counterparty in Q1 2025 (minor units)",
			ExpectedValue: fmt.Sprintf("%d", topRows[0].Total),
			ValueType:     "number",
			Normalization: "currency_minor_units",
		}
	}
	if len(topRows) > 1 {
		secondTotal := topRows[1].Total
		tiedNames := make([]string, 0)
		for i := 1; i < len(topRows); i++ {
			if topRows[i].Total == secondTotal {
				tiedNames = append(tiedNames, topRows[i].Name)
			}
		}
		sort.Strings(tiedNames)
		facts[factKey(d.ID(), "q1_second_place_tied_names")] = Fact{
			Key:           factKey(d.ID(), "q1_second_place_tied_names"),
			Description:   "Counterparties tied for second place in Q1 2025 incoming payments",
			ExpectedValue: strings.Join(tiedNames, ","),
			ValueType:     "list",
		}
		facts[factKey(d.ID(), "q1_second_amount_minor")] = Fact{
			Key:           factKey(d.ID(), "q1_second_amount_minor"),
			Description:   "Incoming payment amount shared by second-place tie in Q1 2025 (minor units)",
			ExpectedValue: fmt.Sprintf("%d", secondTotal),
			ValueType:     "number",
			Normalization: "currency_minor_units",
		}
	}

	q1Income, q2Income, err := queryIncomeByQuarter(ctx, tx, tenantID, marker)
	if err != nil {
		return nil, err
	}
	delta := q2Income - q1Income
	pctChange := 0.0
	if q1Income != 0 {
		pctChange = (float64(delta) / float64(q1Income)) * 100
	}

	facts[factKey(d.ID(), "q1_total_income_minor")] = Fact{
		Key:           factKey(d.ID(), "q1_total_income_minor"),
		Description:   "Total incoming payment amount in Q1 2025 (minor units)",
		ExpectedValue: fmt.Sprintf("%d", q1Income),
		ValueType:     "number",
		Normalization: "currency_minor_units",
	}
	facts[factKey(d.ID(), "q2_total_income_minor")] = Fact{
		Key:           factKey(d.ID(), "q2_total_income_minor"),
		Description:   "Total incoming payment amount in Q2 2025 (minor units)",
		ExpectedValue: fmt.Sprintf("%d", q2Income),
		ValueType:     "number",
		Normalization: "currency_minor_units",
	}
	facts[factKey(d.ID(), "q1_vs_q2_delta_minor")] = Fact{
		Key:           factKey(d.ID(), "q1_vs_q2_delta_minor"),
		Description:   "Q2 minus Q1 incoming payment delta (minor units)",
		ExpectedValue: fmt.Sprintf("%d", delta),
		ValueType:     "number",
		Normalization: "currency_minor_units",
	}
	facts[factKey(d.ID(), "q1_vs_q2_pct_change")] = Fact{
		Key:           factKey(d.ID(), "q1_vs_q2_pct_change"),
		Description:   "Q2 vs Q1 percentage change in incoming payments",
		ExpectedValue: fmt.Sprintf("%.2f", pctChange),
		ValueType:     "number",
		Tolerance:     0.15,
		Normalization: "percent",
	}

	q1Expense, febExpense, err := queryExpenseTotals(ctx, tx, tenantID, marker)
	if err != nil {
		return nil, err
	}
	facts[factKey(d.ID(), "q1_total_expense_minor")] = Fact{
		Key:           factKey(d.ID(), "q1_total_expense_minor"),
		Description:   "Total expense amount in Q1 2025 (minor units)",
		ExpectedValue: fmt.Sprintf("%d", q1Expense),
		ValueType:     "number",
		Normalization: "currency_minor_units",
	}
	facts[factKey(d.ID(), "feb_total_expense_minor")] = Fact{
		Key:           factKey(d.ID(), "feb_total_expense_minor"),
		Description:   "Total expense amount in February 2025 (minor units)",
		ExpectedValue: fmt.Sprintf("%d", febExpense),
		ValueType:     "number",
		Normalization: "currency_minor_units",
	}

	netQ1 := q1Income - q1Expense
	facts[factKey(d.ID(), "q1_net_cashflow_minor")] = Fact{
		Key:           factKey(d.ID(), "q1_net_cashflow_minor"),
		Description:   "Q1 2025 net cashflow: incoming payments minus expenses (minor units)",
		ExpectedValue: fmt.Sprintf("%d", netQ1),
		ValueType:     "number",
		Normalization: "currency_minor_units",
	}

	return facts, nil
}

type counterpartyTotal struct {
	Name  string
	Total int64
}

func queryAcmeQ1(ctx context.Context, tx pgx.Tx, tenantID uuid.UUID, markerLike string) (int64, int64, error) {
	const query = `
		SELECT COALESCE(SUM(t.amount), 0) AS total, COUNT(*) AS cnt
		FROM payments p
		JOIN transactions t ON t.id = p.transaction_id
		JOIN counterparty c ON c.id = p.counterparty_id
		WHERE p.tenant_id = $1
		  AND t.tenant_id = $1
		  AND c.tenant_id = $1
		  AND t.comment LIKE $2
		  AND c.name = 'Acme Retail'
		  AND t.transaction_date >= DATE '2025-01-01'
		  AND t.transaction_date < DATE '2025-04-01'
	`
	var total int64
	var cnt int64
	if err := tx.QueryRow(ctx, query, tenantID, markerLike).Scan(&total, &cnt); err != nil {
		return 0, 0, fmt.Errorf("query acme q1 oracle: %w", err)
	}
	return total, cnt, nil
}

func queryTopCounterpartiesQ1(ctx context.Context, tx pgx.Tx, tenantID uuid.UUID, markerLike string) ([]counterpartyTotal, error) {
	const query = `
		SELECT c.name, COALESCE(SUM(t.amount), 0) AS total
		FROM payments p
		JOIN transactions t ON t.id = p.transaction_id
		JOIN counterparty c ON c.id = p.counterparty_id
		WHERE p.tenant_id = $1
		  AND t.tenant_id = $1
		  AND c.tenant_id = $1
		  AND t.comment LIKE $2
		  AND t.transaction_date >= DATE '2025-01-01'
		  AND t.transaction_date < DATE '2025-04-01'
		GROUP BY c.name
		ORDER BY total DESC, c.name ASC
		LIMIT 3
	`

	rows, err := tx.Query(ctx, query, tenantID, markerLike)
	if err != nil {
		return nil, fmt.Errorf("query top counterparties oracle: %w", err)
	}
	defer rows.Close()

	res := make([]counterpartyTotal, 0, 3)
	for rows.Next() {
		var item counterpartyTotal
		if err := rows.Scan(&item.Name, &item.Total); err != nil {
			return nil, fmt.Errorf("scan top counterparties oracle: %w", err)
		}
		res = append(res, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate top counterparties oracle: %w", err)
	}

	return res, nil
}

func queryIncomeByQuarter(ctx context.Context, tx pgx.Tx, tenantID uuid.UUID, markerLike string) (int64, int64, error) {
	const query = `
		SELECT
			COALESCE(SUM(CASE WHEN t.transaction_date >= DATE '2025-01-01' AND t.transaction_date < DATE '2025-04-01' THEN t.amount ELSE 0 END), 0) AS q1,
			COALESCE(SUM(CASE WHEN t.transaction_date >= DATE '2025-04-01' AND t.transaction_date < DATE '2025-07-01' THEN t.amount ELSE 0 END), 0) AS q2
		FROM payments p
		JOIN transactions t ON t.id = p.transaction_id
		WHERE p.tenant_id = $1
		  AND t.tenant_id = $1
		  AND t.comment LIKE $2
	`

	var q1 int64
	var q2 int64
	if err := tx.QueryRow(ctx, query, tenantID, markerLike).Scan(&q1, &q2); err != nil {
		return 0, 0, fmt.Errorf("query income by quarter oracle: %w", err)
	}
	return q1, q2, nil
}

func queryExpenseTotals(ctx context.Context, tx pgx.Tx, tenantID uuid.UUID, markerLike string) (int64, int64, error) {
	const query = `
		SELECT
			COALESCE(SUM(CASE WHEN t.transaction_date >= DATE '2025-01-01' AND t.transaction_date < DATE '2025-04-01' THEN t.amount ELSE 0 END), 0) AS q1,
			COALESCE(SUM(CASE WHEN t.transaction_date >= DATE '2025-02-01' AND t.transaction_date < DATE '2025-03-01' THEN t.amount ELSE 0 END), 0) AS feb
		FROM expenses e
		JOIN transactions t ON t.id = e.transaction_id
		WHERE e.tenant_id = $1
		  AND t.tenant_id = $1
		  AND t.comment LIKE $2
	`

	var q1 int64
	var feb int64
	if err := tx.QueryRow(ctx, query, tenantID, markerLike).Scan(&q1, &feb); err != nil {
		return 0, 0, fmt.Errorf("query expense totals oracle: %w", err)
	}
	return q1, feb, nil
}

func deterministicID(datasetID, name string) uuid.UUID {
	return uuid.NewSHA1(uuid.NameSpaceOID, []byte("bichat-eval:"+datasetID+":"+name))
}

func datasetMarker(datasetID string) string {
	return "EVAL_" + strings.ToUpper(datasetID)
}

func factKey(datasetID, key string) string {
	return datasetID + "." + key
}

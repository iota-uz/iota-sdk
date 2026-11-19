package persistence

import (
	"context"

	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// CounterpartyHistoryLabels captions counterparties in business history by name.
func CounterpartyHistoryLabels(ctx context.Context, ids []string) (map[string]string, error) {
	return historyLabels(ctx, "CounterpartyHistoryLabels", `SELECT id::text, name FROM counterparty WHERE tenant_id = $1 AND id::text = ANY($2)`, ids)
}

// MoneyAccountHistoryLabels captions money accounts in business history by name.
func MoneyAccountHistoryLabels(ctx context.Context, ids []string) (map[string]string, error) {
	return historyLabels(ctx, "MoneyAccountHistoryLabels", `SELECT id::text, name FROM money_accounts WHERE tenant_id = $1 AND id::text = ANY($2)`, ids)
}

func historyLabels(ctx context.Context, op serrors.Op, query string, ids []string) (map[string]string, error) {
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, serrors.Wrap(op, err)
	}
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, serrors.Wrap(op, err)
	}
	rows, err := tx.Query(ctx, query, tenantID, ids)
	if err != nil {
		return nil, serrors.Wrap(op, err)
	}
	defer rows.Close()
	labels := make(map[string]string, len(ids))
	for rows.Next() {
		var id, name string
		if err := rows.Scan(&id, &name); err != nil {
			return nil, serrors.Wrap(op, err)
		}
		labels[id] = name
	}
	if err := rows.Err(); err != nil {
		return nil, serrors.Wrap(op, err)
	}
	return labels, nil
}

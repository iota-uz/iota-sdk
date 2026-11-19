package persistence

import (
	"context"

	"github.com/google/uuid"

	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// CounterpartyHistoryLabels captions counterparties in business history by name.
func CounterpartyHistoryLabels(ctx context.Context, ids []string) (map[string]string, error) {
	return historyLabels(ctx, "CounterpartyHistoryLabels", `SELECT id::text, name FROM counterparty WHERE tenant_id = $1 AND id = ANY($2::uuid[])`, ids)
}

// MoneyAccountHistoryLabels captions money accounts in business history by name.
func MoneyAccountHistoryLabels(ctx context.Context, ids []string) (map[string]string, error) {
	return historyLabels(ctx, "MoneyAccountHistoryLabels", `SELECT id::text, name FROM money_accounts WHERE tenant_id = $1 AND id = ANY($2::uuid[])`, ids)
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
	// Compare the UUID column uncast so its index serves the lookup; an ID that is not a UUID names no row.
	keys := make([]uuid.UUID, 0, len(ids))
	for _, id := range ids {
		if key, err := uuid.Parse(id); err == nil {
			keys = append(keys, key)
		}
	}
	if len(keys) == 0 {
		return map[string]string{}, nil
	}
	rows, err := tx.Query(ctx, query, tenantID, keys)
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

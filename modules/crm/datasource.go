package crm

import (
	"context"
	"fmt"
	"strings"

	icons "github.com/iota-uz/icons/phosphor"
	"github.com/iota-uz/iota-sdk/modules/crm/infrastructure/persistence/models"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/repo"
	"github.com/iota-uz/iota-sdk/pkg/spotlight"
)

var _ spotlight.DataSource = &dataSource{}

type dataSource struct {
}

func (d *dataSource) Find(ctx context.Context, q string) []spotlight.Item {
	logger := composables.UseLogger(ctx)
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return []spotlight.Item{}
	}

	ilikeQ := "%" + q + "%"

	filters := make([]string, 0, 10)
	for _, f := range []string{
		"first_name",
		"last_name",
		"middle_name",
		"email",
		"phone_number",
		"address",
		"pin",
	} {
		filters = append(filters, repo.ILike(ilikeQ).String(f, 1))
	}
	query := repo.Join(
		"SELECT id, first_name, last_name FROM clients",
		repo.JoinWhere(strings.Join(filters, " OR ")),
	)
	rows, err := tx.Query(ctx, query, ilikeQ)
	if err != nil {
		logger.Error("failed to query clients", "error", err)
		return []spotlight.Item{}
	}
	defer rows.Close()

	items := make([]spotlight.Item, 0, 10)
	for rows.Next() {
		var c models.Client

		if err := rows.Scan(
			&c.ID,
			&c.FirstName,
			&c.LastName,
		); err != nil {
			logger.Error("failed to scan client", "error", err)
			return []spotlight.Item{}
		}
		items = append(items, spotlight.NewItem(
			icons.Users(icons.Props{Size: "20"}),
			c.FirstName+" "+c.LastName.String,
			fmt.Sprintf("/crm/clients?tab=profile&view=%d", c.ID),
		))
	}
	return items
}

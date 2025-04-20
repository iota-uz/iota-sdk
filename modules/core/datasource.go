package core

import (
	"context"
	"fmt"

	icons "github.com/iota-uz/icons/phosphor"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence/models"
	"github.com/iota-uz/iota-sdk/pkg/composables"
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
	query := `SELECT id, first_name, last_name FROM users 
	WHERE first_name ILIKE $1 OR last_name ILIKE $1 OR email ILIKE $1 OR phone ILIKE $1`
	rows, err := tx.Query(ctx, query, "%"+q+"%")
	if err != nil {
		logger.Error("failed to query users", "error", err)
		return []spotlight.Item{}
	}
	defer rows.Close()

	items := make([]spotlight.Item, 0, 10)
	for rows.Next() {
		var u models.User

		if err := rows.Scan(
			&u.ID,
			&u.FirstName,
			&u.LastName,
		); err != nil {
			logger.Error("failed to scan user", "error", err)
			return []spotlight.Item{}
		}
		items = append(items, spotlight.NewItem(
			icons.UserCircle(icons.Props{Size: "20"}),
			u.FirstName+" "+u.LastName,
			fmt.Sprintf("/users/%d", u.ID),
		))
	}
	return items
}

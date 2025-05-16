package core

import (
	"context"
	"fmt"
	"strings"

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

	// Split query by spaces to handle cases like "Firstname Lastname"
	queryParts := strings.Fields(q)

	// If no query parts, return empty results
	if len(queryParts) == 0 {
		return []spotlight.Item{}
	}

	// Fields to search in
	searchFields := []string{
		"first_name",
		"last_name",
		"email",
		"phone",
	}

	// Build the WHERE clause for each part of the query
	whereConditions := make([]string, 0, len(queryParts))
	args := make([]interface{}, 0, len(queryParts)*len(searchFields))
	argIndex := 1

	for _, part := range queryParts {
		fieldConditions := make([]string, 0, len(searchFields))
		for _, field := range searchFields {
			fieldConditions = append(fieldConditions, fmt.Sprintf("%s ILIKE $%d", field, argIndex))
			args = append(args, "%"+part+"%")
			argIndex++
		}
		whereConditions = append(whereConditions, "("+strings.Join(fieldConditions, " OR ")+")")
	}

	// Join the conditions with AND (each part must match at least one field)
	whereClause := strings.Join(whereConditions, " AND ")

	query := fmt.Sprintf("SELECT id, first_name, last_name FROM users WHERE %s", whereClause)

	rows, err := tx.Query(ctx, query, args...)
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

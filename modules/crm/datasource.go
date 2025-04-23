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

var _ spotlight.DataSource = &ClientDataSource{}

type ClientDataSource struct {
}

func (d *ClientDataSource) Find(ctx context.Context, q string) []spotlight.Item {
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
		"middle_name",
		"email",
		"phone_number",
		"address",
		"pin",
	}

	// Build filter conditions for each query part and each field
	allFilters := make([]string, 0)
	args := make([]interface{}, 0)
	argIndex := 1

	for _, part := range queryParts {
		ilikePart := "%" + part + "%"
		partFilters := make([]string, 0, len(searchFields))

		for _, field := range searchFields {
			partFilters = append(partFilters, repo.ILike(fmt.Sprintf("$%d", argIndex)).String(field, argIndex))
			args = append(args, ilikePart)
			argIndex++
		}

		// Group filters for each part with OR
		if len(partFilters) > 0 {
			allFilters = append(allFilters, "("+strings.Join(partFilters, " OR ")+")")
		}
	}

	// Combine all part filters with AND (each part must match at least one field)
	whereClause := strings.Join(allFilters, " AND ")

	// Build the query
	query := repo.Join(
		"SELECT id, first_name, last_name FROM clients",
		repo.JoinWhere(whereClause),
	)

	// Execute the query with the collected arguments
	rows, err := tx.Query(ctx, query, args...)
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

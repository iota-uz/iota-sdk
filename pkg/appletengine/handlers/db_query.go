package handlers

import (
	"fmt"
	"strings"
)

type DBConstraint struct {
	Field string
	Op    string
	Value any
}

type DBIndexConstraint struct {
	Name string
	DBConstraint
}

type DBQueryOptions struct {
	Index  *DBIndexConstraint
	Filter []DBConstraint
	Order  string
	Limit  int
}

func parseDBQueryOptions(raw any) (DBQueryOptions, error) {
	options := DBQueryOptions{
		Order: "desc",
	}
	payload, ok := raw.(map[string]any)
	if !ok || payload == nil {
		return options, nil
	}

	if rawOrder, ok := payload["order"].(string); ok {
		order := strings.ToLower(strings.TrimSpace(rawOrder))
		if order == "asc" || order == "desc" {
			options.Order = order
		}
	}
	if rawTake, ok := payload["take"]; ok {
		switch v := rawTake.(type) {
		case float64:
			options.Limit = int(v)
		case int:
			options.Limit = v
		}
		if options.Limit < 0 {
			options.Limit = 0
		}
	}

	if rawIndex, ok := payload["index"].(map[string]any); ok {
		indexName, _ := rawIndex["name"].(string)
		field, _ := rawIndex["field"].(string)
		op, _ := rawIndex["op"].(string)
		field = strings.TrimSpace(field)
		op = normalizeConstraintOp(op)
		if field != "" {
			options.Index = &DBIndexConstraint{
				Name: strings.TrimSpace(indexName),
				DBConstraint: DBConstraint{
					Field: field,
					Op:    op,
					Value: rawIndex["value"],
				},
			}
		}
	}

	if rawFilters, ok := payload["filters"].([]any); ok {
		filters := make([]DBConstraint, 0, len(rawFilters))
		for _, item := range rawFilters {
			filterMap, ok := item.(map[string]any)
			if !ok {
				continue
			}
			field, _ := filterMap["field"].(string)
			field = strings.TrimSpace(field)
			if field == "" {
				continue
			}
			op, _ := filterMap["op"].(string)
			filters = append(filters, DBConstraint{
				Field: field,
				Op:    normalizeConstraintOp(op),
				Value: filterMap["value"],
			})
		}
		options.Filter = filters
	}

	if options.Index != nil && options.Index.Op != "eq" {
		return DBQueryOptions{}, fmt.Errorf("unsupported index op: %q", options.Index.Op)
	}
	for _, filter := range options.Filter {
		if filter.Op != "eq" {
			return DBQueryOptions{}, fmt.Errorf("unsupported filter op: %q", filter.Op)
		}
	}

	return options, nil
}

func normalizeConstraintOp(op string) string {
	normalized := strings.ToLower(strings.TrimSpace(op))
	if normalized == "" {
		return "eq"
	}
	return normalized
}

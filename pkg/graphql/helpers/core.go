package helpers

import (
	"context"
	"fmt"
	"github.com/99designs/gqlgen/graphql"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
	"strings"
)

type FieldsFilter func(f *schema.Field) bool

func GetPreloads(ctx context.Context) []string {
	return GetNestedPreloads(
		graphql.GetOperationContext(ctx),
		graphql.CollectFieldsCtx(ctx, nil),
		"",
	)
}

func HasAssociation(preloads []string, association string) bool {
	for _, preload := range preloads {
		if strings.Contains(preload, association+".") {
			return true
		}
	}
	return false
}

func GetNestedPreloads(
	ctx *graphql.OperationContext,
	fields []graphql.CollectedField,
	prefix string,
) []string {
	var preloads []string
	for _, column := range fields {
		prefixColumn := GetPreloadString(prefix, column.Name)
		preloads = append(preloads, prefixColumn)
		preloads = append(preloads, GetNestedPreloads(
			ctx,
			graphql.CollectFields(ctx, column.Selections, nil),
			prefixColumn)...,
		)
	}
	return preloads
}

func GetPreloadString(prefix, name string) string {
	if len(prefix) > 0 {
		return prefix + "." + name
	}
	return name
}

func ApplySort(query *gorm.DB, sortBy []string) (*gorm.DB, error) {
	for _, s := range sortBy {
		parts := strings.Split(s, " ")
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid sort field %s", s)
		}
		field := parts[0]
		order := parts[1]
		if order != "asc" && order != "desc" {
			return nil, fmt.Errorf("invalid sort order %s", order)
		}
		query = query.Order(fmt.Sprintf("%s %s", field, order))
	}
	return query, nil
}

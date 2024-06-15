package helpers

import (
	"context"
	"errors"
	"fmt"
	"github.com/99designs/gqlgen/graphql"
	"github.com/iota-agency/iota-erp/sdk/db/dbutils"
	"github.com/iota-agency/iota-erp/sdk/utils/sequence"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
	"strings"
)

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

func GetNestedPreloads(ctx *graphql.OperationContext, fields []graphql.CollectedField, prefix string) (preloads []string) {
	for _, column := range fields {
		prefixColumn := GetPreloadString(prefix, column.Name)
		preloads = append(preloads, prefixColumn)
		preloads = append(preloads, GetNestedPreloads(ctx, graphql.CollectFields(ctx, column.Selections, nil), prefixColumn)...)
	}
	return
}

func GetPreloadString(prefix, name string) string {
	if len(prefix) > 0 {
		return prefix + "." + name
	}
	return name
}

func ApplySort(query *gorm.DB, sortBy []string, model interface{}) (*gorm.DB, error) {
	mapping, err := dbutils.GetGormFields(model, func(f *schema.Field) bool {
		return f.Readable
	})
	if err != nil {
		return nil, err
	}
	for _, s := range sortBy {
		parts := strings.Split(s, " ")
		if len(parts) != 2 {
			return nil, errors.New(fmt.Sprintf("invalid sort field %s", s))
		}
		sortByField := sequence.Title(parts[0])
		order := parts[1]
		if order != "asc" && order != "desc" {
			return nil, errors.New(fmt.Sprintf("invalid sort order %s", order))
		}
		field, ok := mapping[sortByField]
		if !ok {
			return nil, errors.New(fmt.Sprintf("field %s not found", s))
		}
		query = query.Order(fmt.Sprintf("%s %s", field.DBName, order))
	}
	return query, nil
}

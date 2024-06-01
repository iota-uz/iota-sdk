package helpers

import (
	"context"
	"errors"
	"fmt"
	"github.com/99designs/gqlgen/graphql"
	adapters2 "github.com/iota-agency/iota-erp/sdk/graphql/old/adapters"
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
	mapping, err := adapters2.GetGormFields(model, func(f *schema.Field, gf *adapters2.GqlFieldMeta) bool {
		return f.Readable && gf.Readable
	})
	if err != nil {
		return nil, err
	}
	for _, s := range sortBy {
		parts := strings.Split(s, " ")
		if len(parts) > 2 {
			return nil, errors.New(fmt.Sprintf("invalid sort field %s", s))
		}
		if len(parts) == 2 && parts[1] != "asc" && parts[1] != "desc" {
			return nil, errors.New(fmt.Sprintf("invalid sort order %s", parts[1]))
		}
		field, ok := mapping[parts[0]]
		if !ok {
			return nil, errors.New(fmt.Sprintf("field %s not found", s))
		}
		query = query.Order(fmt.Sprintf("%s %s", field.DBName, parts[1]))
	}
	return query, nil
}

package helpers

import (
	"context"
	"errors"
	"fmt"
	"github.com/99designs/gqlgen/graphql"
	"github.com/iota-agency/iota-erp/sdk/utils/sequence"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
	"strings"
	"sync"
)

type FieldsFilter func(f *schema.Field) bool

// GetGormFields returns a map of fields of a model that are readable and match the filter
// The key of the map is the alias of the field in the graphql schema
// The value is the field itself
func GetGormFields(model interface{}, filter FieldsFilter) (map[string]*schema.Field, error) {
	s, err := schema.Parse(model, &sync.Map{}, schema.NamingStrategy{})
	if err != nil {
		return nil, err
	}
	fields := map[string]*schema.Field{}
	for _, field := range s.Fields {
		if filter(field) {
			fields[field.Name] = field
		}
	}
	return fields, nil
}

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
	mapping, err := GetGormFields(model, func(f *schema.Field) bool {
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

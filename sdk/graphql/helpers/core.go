package helpers

import (
	"context"
	"fmt"
	"github.com/99designs/gqlgen/graphql"
	"github.com/iota-agency/iota-erp/sdk/utils/sequence"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
	"reflect"
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
		if filter == nil {
			fields[field.Name] = field
		} else if filter(field) {
			fields[field.Name] = field
		}
	}
	return fields, nil
}

func CheckModelIsInSync(db *gorm.DB, model interface{}) error {
	modelType := reflect.TypeOf(model).Elem()
	columns, err := db.Migrator().ColumnTypes(model)
	if err != nil {
		return fmt.Errorf("error retrieving columns: %v", err)
	}

	columnsMap := make(map[string]bool, len(columns))
	for _, column := range columns {
		columnsMap[column.Name()] = true
	}
	s, err := schema.Parse(model, &sync.Map{}, schema.NamingStrategy{})
	if err != nil {
		return err
	}
	fields := make(map[string]*schema.Field, len(s.Fields))
	for _, field := range s.Fields {
		if field.DataType == "" {
			continue
		}
		fields[field.DBName] = field
	}

	for _, column := range columns {
		columnName := column.Name()
		if _, exists := fields[columnName]; !exists {
			return fmt.Errorf(
				"column '%s' is present in the database but missing in the model '%s'",
				columnName,
				modelType.Name(),
			)
		}

	}
	for _, field := range fields {
		if !db.Migrator().HasColumn(model, field.DBName) {
			return fmt.Errorf(
				"column '%s' is missing in the database but present in the model: '%s'",
				field.Name,
				modelType.Name(),
			)
		}
	}
	return nil
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
			return nil, fmt.Errorf("invalid sort field %s", s)
		}
		sortByField := sequence.Title(parts[0])
		order := parts[1]
		if order != "asc" && order != "desc" {
			return nil, fmt.Errorf("invalid sort order %s", order)
		}
		field, ok := mapping[sortByField]
		if !ok {
			return nil, fmt.Errorf("field %s not found", s)
		}
		query = query.Order(fmt.Sprintf("%s %s", field.DBName, order))
	}
	return query, nil
}

package adapters

import (
	"errors"
	"fmt"
	"github.com/doug-martin/goqu/v9"
	"github.com/doug-martin/goqu/v9/exp"
	"github.com/graphql-go/graphql"
	"gorm.io/gorm/schema"
	"reflect"
	"sync"
)

var Sql2graphql = map[schema.DataType]*graphql.Scalar{
	schema.Int:    graphql.Int,
	schema.String: graphql.String,
	schema.Float:  graphql.Float,
	schema.Time:   graphql.DateTime,
	schema.Bool:   graphql.Boolean,
	schema.Bytes:  graphql.String,
	schema.Uint:   graphql.Int,
}

// QueryToExpression is a map of functions that convert a query string to a goqu expression
// The key is the query string
// The value is a function that takes a column name and a value and returns a goqu expression
var QueryToExpression = map[string]func(string, interface{}) exp.BooleanExpression{
	"gt": func(col string, val interface{}) exp.BooleanExpression {
		return goqu.C(col).Gt(val)
	},
	"gte": func(col string, val interface{}) exp.BooleanExpression {
		return goqu.C(col).Gte(val)
	},
	"lt": func(col string, val interface{}) exp.BooleanExpression {
		return goqu.C(col).Lt(val)
	},
	"lte": func(col string, val interface{}) exp.BooleanExpression {
		return goqu.C(col).Lte(val)
	},
	"in": func(col string, val interface{}) exp.BooleanExpression {
		return goqu.C(col).In(val)
	},
	"out": func(col string, val interface{}) exp.BooleanExpression {
		return goqu.C(col).NotIn(val)
	},
}

type FieldsFilter func(f *schema.Field, gf *GqlFieldMeta) bool

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
		as := field.Tag.Get("gql")
		if as == "" {
			return nil, errors.New("gql tag is required")
		}
		if as == "-" {
			continue
		}
		gqlField, err := GqlFieldMetaFromTag(as)
		if err != nil {
			return nil, err
		}
		if filter(field, gqlField) {
			fields[gqlField.Name] = field
		}
	}
	return fields, nil
}

func GormFieldToGraphqlField(field *schema.Field) *graphql.Field {
	if field.DataType == "" {
		panic("Field is a reference")
	}
	t, ok := Sql2graphql[field.DataType]
	if !ok {
		panic(fmt.Sprintf("Type %v not found for field %s", field.DataType, field.DBName))
	}
	if field.NotNull {
		return &graphql.Field{
			Type: graphql.NewNonNull(t),
		}
	}
	return &graphql.Field{
		Type: t,
	}
}

func GormFieldToGraphqlInputField(field *schema.Field) *graphql.InputObjectFieldConfig {
	if field.DataType == "" {
		panic("Field is a reference")
	}
	t, ok := Sql2graphql[field.DataType]
	if !ok {
		panic(fmt.Sprintf("Type %v not found for field %s", field.DataType, field.DBName))
	}
	return &graphql.InputObjectFieldConfig{
		Type: t,
	}
}

func CastId(args map[string]interface{}, pk *schema.Field) (interface{}, error) {
	as := pk.Tag.Get("gql")
	id, ok := args[as]
	if !ok {
		return nil, errors.New(fmt.Sprintf("%s is required", as))
	}
	if pk.DataType == schema.String {
		if _, ok := id.(string); !ok {
			return nil, errors.New(fmt.Sprintf("Expected type: int, got %v", reflect.TypeOf(id)))
		}
		return id, nil
	}
	if pk.DataType == schema.Int {
		if _, ok := id.(int); !ok {
			return nil, errors.New(fmt.Sprintf("Expected type: int, got %v", reflect.TypeOf(id)))
		}
		return id, nil
	}
	return nil, errors.New(fmt.Sprintf("Unsupported id type: %v", reflect.TypeOf(id)))
}

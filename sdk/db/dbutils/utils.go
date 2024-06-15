package dbutils

import (
	"errors"
	"github.com/iota-agency/iota-erp/sdk/utils/sequence"
	"gorm.io/gorm/schema"
	"sync"
)

type FieldsFilter func(f *schema.Field) bool

func GetModelPk(model interface{}) (*schema.Field, error) {
	s, err := schema.Parse(model, &sync.Map{}, schema.NamingStrategy{})
	if err != nil {
		return nil, err
	}
	for _, field := range s.Fields {
		if field.PrimaryKey {
			return field, nil
		}
	}
	return nil, errors.New("primary key not found")
}

func IsNumeric(kind schema.DataType) bool {
	numerics := []schema.DataType{
		schema.Int,
		schema.Float,
		schema.Uint,
	}
	return sequence.Includes(numerics, kind)
}

func IsString(kind schema.DataType) bool {
	vals := []schema.DataType{
		schema.String,
		schema.Bytes,
	}
	return sequence.Includes(vals, kind)
}

func IsTime(kind schema.DataType) bool {
	times := []schema.DataType{
		schema.Time,
	}
	return sequence.Includes(times, kind)
}

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

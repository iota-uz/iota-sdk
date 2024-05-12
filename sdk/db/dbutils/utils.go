package dbutils

import (
	"errors"
	"github.com/iota-agency/iota-erp/pkg/utils"
	"gorm.io/gorm/schema"
	"sync"
)

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
	return utils.Includes(numerics, kind)
}

func IsString(kind schema.DataType) bool {
	vals := []schema.DataType{
		schema.String,
		schema.Bytes,
	}
	return utils.Includes(vals, kind)
}

func IsTime(kind schema.DataType) bool {
	times := []schema.DataType{
		schema.Time,
	}
	return utils.Includes(times, kind)
}

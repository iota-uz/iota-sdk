package currency

import (
	"context"
	"github.com/iota-uz/iota-sdk/pkg/crud"
)

type currencyMapper struct {
	fields crud.Fields
}

func NewMapper(fields crud.Fields) crud.Mapper[Currency] {
	return &currencyMapper{
		fields: fields,
	}
}

func (m *currencyMapper) ToEntity(_ context.Context, values []crud.FieldValue) (Currency, error) {
	result := Currency{}

	for _, v := range values {
		switch v.Field().Name() {
		case "code":
			code, err := v.AsString()
			if err != nil {
				return result, err
			}
			result.Code = Code(code)
		case "name":
			name, err := v.AsString()
			if err != nil {
				return result, err
			}
			result.Name = name
		case "symbol":
			symbol, err := v.AsString()
			if err != nil {
				return result, err
			}
			result.Symbol = Symbol(symbol)
		case "created_at":
			createdAt, err := v.AsTime()
			if err != nil {
				return result, err
			}
			result.CreatedAt = createdAt
		case "updated_at":
			updatedAt, err := v.AsTime()
			if err != nil {
				return result, err
			}
			result.UpdatedAt = updatedAt
		}
	}

	return result, nil
}

func (m *currencyMapper) ToFieldValues(_ context.Context, entity Currency) ([]crud.FieldValue, error) {
	return m.fields.FieldValues(map[string]any{
		"code":       string(entity.Code),
		"name":       entity.Name,
		"symbol":     string(entity.Symbol),
		"created_at": entity.CreatedAt,
		"updated_at": entity.UpdatedAt,
	})
}

package currency

import (
	"context"
	"github.com/iota-uz/iota-sdk/pkg/crud"
)

func NewMapper(fields crud.Fields) crud.Mapper[Currency] {
	return &currencyMapper{
		fields: fields,
	}
}

type currencyMapper struct {
	fields crud.Fields
}

func (m *currencyMapper) ToEntities(_ context.Context, values ...[]crud.FieldValue) ([]Currency, error) {
	result := make([]Currency, len(values))

	for i, fvs := range values {
		entity := Currency{}
		for _, v := range fvs {
			switch v.Field().Name() {
			case "code":
				code, err := v.AsString()
				if err != nil {
					return result, err
				}
				entity.Code = Code(code)
			case "name":
				name, err := v.AsString()
				if err != nil {
					return result, err
				}
				entity.Name = name
			case "symbol":
				symbol, err := v.AsString()
				if err != nil {
					return result, err
				}
				entity.Symbol = Symbol(symbol)
			case "created_at":
				createdAt, err := v.AsTime()
				if err != nil {
					return result, err
				}
				entity.CreatedAt = createdAt
			case "updated_at":
				updatedAt, err := v.AsTime()
				if err != nil {
					return result, err
				}
				entity.UpdatedAt = updatedAt
			}
		}
		result[i] = entity
	}

	return result, nil
}

func (m *currencyMapper) ToFieldValuesList(_ context.Context, entities ...Currency) ([][]crud.FieldValue, error) {
	result := make([][]crud.FieldValue, len(entities))

	for i, entity := range entities {
		fvs, err := m.fields.FieldValues(
			map[string]any{
				"code":       string(entity.Code),
				"name":       entity.Name,
				"symbol":     string(entity.Symbol),
				"created_at": entity.CreatedAt,
				"updated_at": entity.UpdatedAt,
			},
		)
		if err != nil {
			return nil, err
		}
		result[i] = fvs
	}

	return result, nil
}

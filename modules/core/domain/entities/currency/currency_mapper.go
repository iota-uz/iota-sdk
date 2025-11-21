package currency

import (
	"context"
	"time"

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
		var code Code
		var name string
		var symbol Symbol
		var createdAt, updatedAt time.Time

		for _, v := range fvs {
			switch v.Field().Name() {
			case "code":
				codeStr, err := v.AsString()
				if err != nil {
					return result, err
				}
				code = Code(codeStr)
			case "name":
				n, err := v.AsString()
				if err != nil {
					return result, err
				}
				name = n
			case "symbol":
				symbolStr, err := v.AsString()
				if err != nil {
					return result, err
				}
				symbol = Symbol(symbolStr)
			case "created_at":
				ct, err := v.AsTime()
				if err != nil {
					return result, err
				}
				createdAt = ct
			case "updated_at":
				ut, err := v.AsTime()
				if err != nil {
					return result, err
				}
				updatedAt = ut
			}
		}

		result[i] = New(code, name, symbol, WithCreatedAt(createdAt), WithUpdatedAt(updatedAt))
	}

	return result, nil
}

func (m *currencyMapper) ToFieldValuesList(_ context.Context, entities ...Currency) ([][]crud.FieldValue, error) {
	result := make([][]crud.FieldValue, len(entities))

	for i, entity := range entities {
		fvs, err := m.fields.FieldValues(
			map[string]any{
				"code":       string(entity.Code()),
				"name":       entity.Name(),
				"symbol":     string(entity.Symbol()),
				"created_at": entity.CreatedAt(),
				"updated_at": entity.UpdatedAt(),
			},
		)
		if err != nil {
			return nil, err
		}
		result[i] = fvs
	}

	return result, nil
}

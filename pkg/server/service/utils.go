package service

import (
	"github.com/graphql-go/graphql"
	"github.com/graphql-go/graphql/language/ast"
	"github.com/iota-agency/iota-erp/pkg/utils"
	"strings"
)

func getAttrs(p graphql.ResolveParams) []interface{} {
	var attrs []interface{}
	if p.Info.FieldASTs[0].SelectionSet != nil {
		for _, field := range p.Info.FieldASTs[0].SelectionSet.Selections {
			attrs = append(attrs, field.(*ast.Field).Name.Value)
		}
	}
	return attrs
}

func nestMap(data map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	for k, v := range data {
		parts := strings.Split(k, ".")
		lastKey := parts[len(parts)-1]
		m := result
		for _, part := range parts[:len(parts)-1] {
			if _, ok := m[part]; !ok {
				m[part] = make(map[string]interface{})
			}
			m = m[part].(map[string]interface{})
		}
		m[lastKey] = v
	}
	return result
}

func isNumeric(kind DataType) bool {
	numerics := []DataType{
		Integer,
		BigSerial,
		SmallSerial,
		Serial,
		Numeric,
		Real,
		DoublePrecision,
	}
	return utils.Includes(numerics, kind)
}

func isString(kind DataType) bool {
	vals := []DataType{
		Character,
		CharacterVarying,
		Text,
	}
	return utils.Includes(vals, kind)
}

func isTime(kind DataType) bool {
	times := []DataType{
		Date,
		Time,
		Timestamp,
	}
	return utils.Includes(times, kind)
}

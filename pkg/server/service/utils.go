package service

import (
	"fmt"
	"github.com/doug-martin/goqu/v9"
	"github.com/graphql-go/graphql"
	"github.com/graphql-go/graphql/language/ast"
	"github.com/iota-agency/iota-erp/pkg/utils"
	"strings"
)

func _getAttrs(parent string, fields []ast.Selection) []interface{} {
	var attrs []interface{}
	for _, _f := range fields {
		field := _f.(*ast.Field)
		selections := field.GetSelectionSet()
		var base string
		if parent != "" {
			base = fmt.Sprintf("%s.%s", parent, field.Name.Value)
		} else {
			base = field.Name.Value
		}
		if selections == nil {
			attrs = append(attrs, base)
		} else {
			attrs = append(attrs, _getAttrs(base, selections.Selections)...)
		}
	}
	return attrs
}

func getAttrs(p graphql.ResolveParams) []interface{} {
	if p.Info.FieldASTs[0].SelectionSet == nil {
		return []interface{}{}
	}
	return _getAttrs("", p.Info.FieldASTs[0].SelectionSet.Selections)
}

func ResolveToQuery(p graphql.ResolveParams, model *Model) *goqu.SelectDataset {
	allAttrs := getAttrs(p)
	var attrs []interface{}
	for _, attr := range allAttrs {
		parts := strings.Split(attr.(string), ".")
		if len(parts) == 1 {
			attrs = append(attrs, goqu.I(fmt.Sprintf("%s.%s", model.Table, attr)))
			continue
		}
		dest := parts[len(parts)-2]
		attr := parts[len(parts)-1]

		for _, field := range model.Refs() {
			if field.Association.As == dest {
				source := fmt.Sprintf("%s.%s", field.Association.To.Table, attr)
				target := fmt.Sprintf("%s.%s", field.Association.As, attr)
				attrs = append(attrs, goqu.I(source).As(goqu.C(target)))
			}
		}
	}
	query := goqu.From(model.Table).Select(attrs...)
	for _, field := range model.Refs() {
		refTable := field.Association.To.Table
		query = query.Join(
			goqu.I(refTable),
			goqu.On(
				goqu.Ex{
					fmt.Sprintf("%s.%s", refTable, field.Association.Column): goqu.I(fmt.Sprintf("%s.%s", model.Table, field.Name)),
				},
			),
		)
	}
	return query
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

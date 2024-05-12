package resolvers

import (
	"fmt"
	"github.com/doug-martin/goqu/v9"
	"github.com/doug-martin/goqu/v9/exp"
	"github.com/graphql-go/graphql"
	"github.com/graphql-go/graphql/language/ast"
	"gorm.io/gorm/schema"
	"strings"
	"sync"
)

func OrderedExpressionsFromResolveParams(p graphql.ResolveParams) []exp.OrderedExpression {
	fields, ok := p.Args["sortBy"].([]interface{})
	if !ok {
		return nil
	}
	sortBy := make([]string, len(fields))
	for i, f := range fields {
		sortBy[i] = f.(string)
	}
	return OrderStringToExpression(sortBy)
}

func _getAttrs(parent string, fields []ast.Selection) []interface{} {
	var attrs []interface{}
	for _, _f := range fields {
		field := _f.(*ast.Field)
		if field.Name.Value == "__typename" {
			continue
		}
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

func GetAssociations(model interface{}, selectionSet *ast.SelectionSet) []string {
	s, err := schema.Parse(model, &sync.Map{}, schema.NamingStrategy{})
	if err != nil {
		panic(err)
	}
	var selected []string
	for _, selection := range selectionSet.Selections {
		field := selection.(*ast.Field)
		selections := field.GetSelectionSet()
		if selections == nil {
			continue
		}
		selected = append(selected, field.Name.Value)
	}
	associations := make([]string, len(selected))
	for i, sel := range selected {
		for _, rel := range s.Fields {
			if rel.DataType == "" && rel.Tag.Get("gql") == sel {
				associations[i] = rel.Name
			}
		}
	}
	return associations
}

func NestMap(model interface{}, data map[string]interface{}) map[string]interface{} {
	s, err := schema.Parse(model, &sync.Map{}, schema.NamingStrategy{})
	if err != nil {
		panic(err)
	}
	nested := make(map[string]interface{})
	for _, field := range s.Fields {
		as := field.Tag.Get("gql")
		prefix := field.Name + "__"
		for key, value := range data {
			if value == nil {
				continue
			}
			if field.DataType != "" && field.DBName == key && as != "-" {
				nested[as] = value
				continue
			}
			if strings.HasPrefix(key, prefix) {
				nakedKey := strings.TrimPrefix(key, prefix)
				if nested[as] == nil {
					nested[as] = map[string]interface{}{}
				}
				nested[as].(map[string]interface{})[nakedKey] = value
			}
		}
	}
	return nested
}

func GetAttrs(selectionSet *ast.SelectionSet) []interface{} {
	return _getAttrs("", selectionSet.Selections)
}

func OrderStringToExpression(order []string) []exp.OrderedExpression {
	var orderExpr []exp.OrderedExpression
	for _, sort := range order {
		if sort[0] == '-' {
			orderExpr = append(orderExpr, goqu.I(sort[1:]).Desc())
		} else {
			orderExpr = append(orderExpr, goqu.I(sort).Asc())
		}
	}
	return orderExpr
}

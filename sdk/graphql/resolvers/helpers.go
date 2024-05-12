package resolvers

import (
	"fmt"
	"github.com/doug-martin/goqu/v9"
	"github.com/doug-martin/goqu/v9/exp"
	"github.com/graphql-go/graphql"
	"github.com/graphql-go/graphql/language/ast"
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

func GetAttrs(selectionSet *ast.SelectionSet) []interface{} {
	return _getAttrs("", selectionSet.Selections)
}

//func ResolveToQuery(selectionSet *ast.SelectionSet, model models.Model) *goqu.SelectDataset {
//	allAttrs := GetAttrs(selectionSet)
//	var attrs []interface{}
//	for _, attr := range allAttrs {
//		parts := strings.Split(attr.(string), ".")
//		if len(parts) == 1 {
//			attrs = append(attrs, goqu.I(fmt.Sprintf("%s.%s", model.Table(), attr)))
//			continue
//		}
//		dest := parts[len(parts)-2]
//		attr := parts[len(parts)-1]
//		for _, field := range models.Refs(model) {
//			if field.Association.As == dest {
//				source := fmt.Sprintf("%s.%s", field.Association.To.Table(), attr)
//				target := fmt.Sprintf("%s.%s", field.Association.As, attr)
//				attrs = append(attrs, goqu.I(source).As(goqu.C(target)))
//			}
//		}
//	}
//	query := goqu.From(model.Table()).Select(attrs...)
//	for _, field := range models.Refs(model) {
//		refTable := field.Association.To.Table()
//		query = query.LeftJoin(
//			goqu.I(refTable),
//			goqu.On(
//				goqu.Ex{
//					fmt.Sprintf("%s.%s", refTable, field.Association.Column): goqu.I(fmt.Sprintf("%s.%s", model.Table(), field.Name)),
//				},
//			),
//		)
//	}
//	return query
//}

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

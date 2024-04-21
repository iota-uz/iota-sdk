package adapters

import (
	"fmt"
	"github.com/doug-martin/goqu/v9"
	"github.com/doug-martin/goqu/v9/exp"
	"github.com/graphql-go/graphql"
	"github.com/graphql-go/graphql/language/ast"
	"github.com/iota-agency/iota-erp/models"
	"github.com/iota-agency/iota-erp/pkg/server/graphql/dbutils"
	"github.com/iota-agency/iota-erp/pkg/server/graphql/resolvers"
	"github.com/iota-agency/iota-erp/pkg/utils"
	"github.com/jmoiron/sqlx"
)

var StringOpToExpression = map[string]func(col interface{}) exp.SQLFunctionExpression{
	"min":   goqu.MIN,
	"max":   goqu.MAX,
	"avg":   goqu.AVG,
	"sum":   goqu.SUM,
	"count": goqu.COUNT,
}

func aggregateSubQuery(model models.Model, name string) *graphql.Object {
	fields := graphql.Fields{}
	aggregationQuery := func(f *models.Field) *graphql.Object {
		queryFields := graphql.Fields{
			"count": &graphql.Field{
				Type: graphql.Int,
			},
		}
		if dbutils.IsTime(f.Type) || dbutils.IsNumeric(f.Type) {
			queryFields["min"] = &graphql.Field{
				Type: sql2graphql[f.Type],
			}
			queryFields["max"] = &graphql.Field{
				Type: sql2graphql[f.Type],
			}
		}
		if dbutils.IsNumeric(f.Type) {
			queryFields["avg"] = &graphql.Field{
				Type: sql2graphql[f.Type],
			}
			queryFields["sum"] = &graphql.Field{
				Type: sql2graphql[f.Type],
			}
		}
		return graphql.NewObject(
			graphql.ObjectConfig{
				Name:   fmt.Sprintf("%s%sAggregationQuery", utils.Title(name), utils.Title(f.Name)),
				Fields: queryFields,
			},
		)
	}
	for _, field := range models.Fields(model) {
		gqlType, ok := sql2graphql[field.Type]
		if !ok {
			panic(fmt.Sprintf("Type %v not found", field.Type))
		}
		args := graphql.FieldConfigArgument{
			"in": &graphql.ArgumentConfig{
				Type: graphql.NewList(gqlType),
			},
			"out": &graphql.ArgumentConfig{
				Type: graphql.NewList(gqlType),
			},
		}
		if dbutils.IsTime(field.Type) || dbutils.IsNumeric(field.Type) {
			args["gt"] = &graphql.ArgumentConfig{
				Type: gqlType,
			}
			args["gte"] = &graphql.ArgumentConfig{
				Type: gqlType,
			}
			args["lt"] = &graphql.ArgumentConfig{
				Type: gqlType,
			}
			args["lte"] = &graphql.ArgumentConfig{
				Type: gqlType,
			}
		}
		fields[field.Name] = &graphql.Field{
			Type: aggregationQuery(field),
			Args: args,
		}
	}
	return graphql.NewObject(
		graphql.ObjectConfig{
			Name:   fmt.Sprintf("%sAggregate", utils.Title(name)),
			Fields: fields,
		},
	)
}

func AggregateQuery(db *sqlx.DB, model models.Model, name string) *graphql.Field {
	pk := model.PkField()
	return &graphql.Field{
		Name:        name,
		Type:        graphql.NewList(aggregateSubQuery(model, model.Table())),
		Description: "Get aggregated data",
		Args: graphql.FieldConfigArgument{
			"groupBy": &graphql.ArgumentConfig{
				Type: graphql.NewList(graphql.String),
			},
			"limit": &graphql.ArgumentConfig{
				Type:         graphql.Int,
				DefaultValue: 50,
			},
			"offset": &graphql.ArgumentConfig{
				Type:         graphql.Int,
				DefaultValue: 0,
			},
			"sortBy": &graphql.ArgumentConfig{
				Type: graphql.NewList(graphql.String),
				DefaultValue: []string{
					pk.Name,
				},
			},
		},
		Resolve: func(p graphql.ResolveParams) (interface{}, error) {
			root := p.Info.FieldASTs[0]
			query := goqu.From(model.Table())
			var exprs []interface{}
			for _, _field := range root.SelectionSet.Selections {
				field := _field.(*ast.Field)
				if field.Name.Value == "__typename" {
					continue
				}
				for _, arg := range field.Arguments {
					c := QueryToExpression[arg.Name.Value]
					if arg.Value.GetKind() == "Variable" {
						query = query.Where(c(field.Name.Value, p.Info.VariableValues[arg.Value.GetValue().(*ast.Name).Value]))
					} else {
						query = query.Where(c(field.Name.Value, arg.Value.GetValue()))
					}
				}
				for _, _op := range field.SelectionSet.Selections {
					op := _op.(*ast.Field)
					opName := op.Name.Value
					if opName == "__typename" {
						continue
					}
					sqlOp := StringOpToExpression[opName]
					exprs = append(
						exprs,
						sqlOp(goqu.I(field.Name.Value)).As(goqu.C(fmt.Sprintf("%s.%s", field.Name.Value, opName))),
					)
				}
			}
			query = query.Select(exprs...)
			groupByFields, ok := p.Args["groupBy"].([]interface{})
			groupBy := make([]interface{}, len(groupByFields))
			for i, f := range groupByFields {
				groupBy[i] = f
			}
			query = query.GroupBy(groupBy...).Order(resolvers.OrderedExpressionsFromResolveParams(p)...)
			limit, ok := p.Args["limit"].(int)
			if ok {
				query = query.Limit(uint(limit))
			}
			offset, ok := p.Args["offset"].(int)
			if ok {
				query = query.Offset(uint(offset))
			}
			return dbutils.Find(db, query)
		},
	}
}

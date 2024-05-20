package resolvers

import (
	"fmt"
	"github.com/doug-martin/goqu/v9"
	"github.com/doug-martin/goqu/v9/exp"
	"github.com/graphql-go/graphql"
	"github.com/graphql-go/graphql/language/ast"
	"github.com/iota-agency/iota-erp/pkg/utils"
	"github.com/iota-agency/iota-erp/sdk/db/dbutils"
	"github.com/iota-agency/iota-erp/sdk/graphql/adapters"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
	"sync"
)

var StringOpToExpression = map[string]func(col interface{}) exp.SQLFunctionExpression{
	"min":   goqu.MIN,
	"max":   goqu.MAX,
	"avg":   goqu.AVG,
	"sum":   goqu.SUM,
	"count": goqu.COUNT,
}

func aggregateSubQuery(model interface{}, name string) *graphql.Object {
	fields := graphql.Fields{}
	aggregationQuery := func(f *schema.Field) *graphql.Object {
		queryFields := graphql.Fields{
			"count": &graphql.Field{
				Type: graphql.Int,
			},
		}
		if dbutils.IsTime(f.DataType) || dbutils.IsNumeric(f.DataType) {
			queryFields["min"] = &graphql.Field{
				Type: adapters.Sql2graphql[f.DataType],
			}
			queryFields["max"] = &graphql.Field{
				Type: adapters.Sql2graphql[f.DataType],
			}
		}
		if dbutils.IsNumeric(f.DataType) {
			queryFields["avg"] = &graphql.Field{
				Type: adapters.Sql2graphql[f.DataType],
			}
			queryFields["sum"] = &graphql.Field{
				Type: adapters.Sql2graphql[f.DataType],
			}
		}
		return graphql.NewObject(
			graphql.ObjectConfig{
				Name:   fmt.Sprintf("%s%sAggregationQuery", utils.Title(name), utils.Title(f.Name)),
				Fields: queryFields,
			},
		)
	}
	s, err := schema.Parse(model, &sync.Map{}, schema.NamingStrategy{})
	if err != nil {
		panic(err)
	}
	for _, field := range s.Fields {
		if !field.Readable || field.DataType == "" {
			continue
		}
		gqlType, ok := adapters.Sql2graphql[field.DataType]
		if !ok {
			panic(fmt.Sprintf("Type %v not found for field %s", field.DataType, field.Name))
		}
		args := graphql.FieldConfigArgument{
			"in": &graphql.ArgumentConfig{
				Type: graphql.NewList(gqlType),
			},
			"out": &graphql.ArgumentConfig{
				Type: graphql.NewList(gqlType),
			},
		}
		if dbutils.IsTime(field.DataType) || dbutils.IsNumeric(field.DataType) {
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

func AggregateQuery(db *gorm.DB, model interface{}, name string) *graphql.Field {
	pk, err := dbutils.GetModelPk(model)
	if err != nil {
		panic(err)
	}
	return &graphql.Field{
		Name:        name,
		Type:        graphql.NewList(aggregateSubQuery(model, name)),
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
			// TODO: come back to this
			query := goqu.From("")
			var exprs []interface{}
			for _, _field := range root.SelectionSet.Selections {
				field := _field.(*ast.Field)
				if field.Name.Value == "__typename" {
					continue
				}
				for _, arg := range field.Arguments {
					c := adapters.QueryToExpression[arg.Name.Value]
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
			query = query.GroupBy(groupBy...).Order(OrderedExpressionsFromResolveParams(p)...)
			limit, ok := p.Args["limit"].(int)
			if ok {
				query = query.Limit(uint(limit))
			}
			offset, ok := p.Args["offset"].(int)
			if ok {
				query = query.Offset(uint(offset))
			}
			return nil, nil
		},
	}
}

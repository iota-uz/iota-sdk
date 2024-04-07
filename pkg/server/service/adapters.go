package service

import (
	"errors"
	"fmt"
	"github.com/doug-martin/goqu/v9"
	"github.com/doug-martin/goqu/v9/exp"
	"github.com/graphql-go/graphql"
	"github.com/graphql-go/graphql/language/ast"
	"reflect"
)

type GraphQLAdapterOptions struct {
	Service Service
	Name    string
}

var sql2graphql = map[any]*graphql.Scalar{
	reflect.String:  graphql.String,
	reflect.Int:     graphql.Int,
	reflect.Int32:   graphql.Int,
	reflect.Int64:   graphql.Int,
	reflect.Float32: graphql.Float,
	reflect.Float64: graphql.Float,
	reflect.Bool:    graphql.Boolean,
}

func getAttrs(p graphql.ResolveParams) []string {
	var attrs []string
	if p.Info.FieldASTs[0].SelectionSet != nil {
		for _, field := range p.Info.FieldASTs[0].SelectionSet.Selections {
			attrs = append(attrs, field.(*ast.Field).Name.Value)
		}
	}
	return attrs
}

func paginatedQuery(service Service, modelType *graphql.Object) *graphql.Object {
	return graphql.NewObject(graphql.ObjectConfig{
		Name: "Paginated" + modelType.Name(),
		Fields: graphql.Fields{
			"total": &graphql.Field{
				Type: graphql.Int,
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					return service.Count(&CountQuery{})
				},
			},
			"data": &graphql.Field{
				Type: graphql.NewList(modelType),
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					limit := p.Info.VariableValues["limit"].(int)
					offset := p.Info.VariableValues["offset"].(int)
					_sortBy, ok := p.Info.VariableValues["sortBy"].([]interface{})
					var sortBy []string
					if ok {
						for _, s := range _sortBy {
							sortBy = append(sortBy, s.(string))
						}
					}
					data, err := service.Find(&FindQuery{
						Attrs:  getAttrs(p),
						Limit:  limit,
						Offset: offset,
						Sort:   sortBy,
					})
					if err != nil {
						return nil, err
					}
					return data, nil
				},
			},
		},
	})
}

func modelToGraphQLObject(name string, model *Model) *graphql.Object {
	fields := graphql.Fields{
		model.Pk: &graphql.Field{
			Type: graphql.Int,
		},
	}
	for _, field := range model.Fields {
		t, ok := sql2graphql[field.Type]
		if !ok {
			continue
		}
		fields[field.Name] = &graphql.Field{
			Type: t,
		}
	}
	return graphql.NewObject(
		graphql.ObjectConfig{
			Name:   fmt.Sprintf("%sType", name),
			Fields: fields,
		},
	)
}

func graphqlAggregateQuery(opts *GraphQLAdapterOptions, modelType *graphql.Object) *graphql.Object {
	expressionsMap := map[string]func(col interface{}) exp.SQLFunctionExpression{
		"min": goqu.MIN,
		"max": goqu.MAX,
		"avg": goqu.AVG,
		"sum": goqu.SUM,
	}

	fields := graphql.Fields{}
	aggregationQuery := graphql.NewObject(
		graphql.ObjectConfig{
			Name: fmt.Sprintf("%sAggregationQuery", opts.Name),
			Fields: graphql.Fields{
				"min": &graphql.Field{
					Type: graphql.Float,
				},
				"max": &graphql.Field{
					Type: graphql.Float,
				},
				"avg": &graphql.Field{
					Type: graphql.Float,
				},
				"sum": &graphql.Field{
					Type: graphql.Float,
				},
			},
		},
	)
	for _, field := range opts.Service.Model().Fields {
		fields[field.Name] = &graphql.Field{
			Type: graphql.NewList(aggregationQuery),
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				groupBy, ok := p.Info.VariableValues["groupBy"].([]interface{})
				if !ok {
					return nil, errors.New("groupBy not found")
				}
				attrs := getAttrs(p)
				var expressions []goqu.Expression
				for _, attr := range attrs {
					expressions = append(expressions, expressionsMap[attr](goqu.I(p.Info.FieldName)))
				}
				var groupByString []string
				for _, g := range groupBy {
					groupByString = append(groupByString, g.(string))
				}
				return opts.Service.Aggregate(&AggregateQuery{
					Query:       nil,
					Expressions: expressions,
					GroupBy:     groupByString,
				})
			},
		}
	}
	return graphql.NewObject(
		graphql.ObjectConfig{
			Name:   fmt.Sprintf("%sAggregate", opts.Name),
			Fields: fields,
		},
	)
}

func GraphQLAdapter(opts *GraphQLAdapterOptions) (*graphql.Object, *graphql.Object) {
	pkCol := opts.Service.Model().Pk
	modelType := modelToGraphQLObject(opts.Name, opts.Service.Model())
	paginatedQ := paginatedQuery(opts.Service, modelType)
	aggregateType := graphqlAggregateQuery(opts, modelType)

	queryType := graphql.NewObject(
		graphql.ObjectConfig{
			Name: opts.Name,
			Fields: graphql.Fields{
				"get": &graphql.Field{
					Type:        modelType,
					Description: "Get by id",
					Args: graphql.FieldConfigArgument{
						pkCol: &graphql.ArgumentConfig{
							Type: graphql.Int,
						},
					},
					Resolve: func(p graphql.ResolveParams) (interface{}, error) {
						id, ok := p.Args[pkCol].(int)
						if !ok {
							return nil, nil
						}
						return opts.Service.Get(&GetQuery{
							Id:    int64(id),
							Attrs: getAttrs(p),
						})
					},
				},
				"list": &graphql.Field{
					Type:        graphql.NewList(modelType),
					Description: "Get list",
					Resolve: func(p graphql.ResolveParams) (interface{}, error) {
						return opts.Service.Find(&FindQuery{
							Attrs: getAttrs(p),
						})
					},
				},
				"listPaginated": &graphql.Field{
					Type:        paginatedQ,
					Description: "Get list paginated",
					Args: graphql.FieldConfigArgument{
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
								pkCol,
							},
						},
					},
					Resolve: func(p graphql.ResolveParams) (interface{}, error) {
						return paginatedQ, nil
					},
				},
				"aggregate": &graphql.Field{
					Type: aggregateType,
					Args: graphql.FieldConfigArgument{
						"groupBy": &graphql.ArgumentConfig{
							Type: graphql.NewList(graphql.String),
						},
					},
					Resolve: func(p graphql.ResolveParams) (interface{}, error) {
						return aggregateType, nil
					},
				},
			},
		},
	)
	createArgs := graphql.FieldConfigArgument{}
	for _, field := range opts.Service.Model().Fields {
		createArgs[field.Name] = &graphql.ArgumentConfig{
			Type: sql2graphql[field.Type],
		}
	}
	mutationType := graphql.NewObject(
		graphql.ObjectConfig{
			Name: opts.Name + "Mutation",
			Fields: graphql.Fields{
				"create": &graphql.Field{
					Type:        modelType,
					Description: "Create",
					Args:        createArgs,
					Resolve: func(p graphql.ResolveParams) (interface{}, error) {
						return opts.Service.Create(p.Args)
					},
				},
				"update": &graphql.Field{
					Type:        modelType,
					Description: "Update",
					Args:        createArgs,
					Resolve: func(p graphql.ResolveParams) (interface{}, error) {
						id, ok := p.Args[pkCol].(int64)
						if ok {
							return opts.Service.Patch(id, p.Args)
						}
						return nil, nil
					},
				},
				"delete": &graphql.Field{
					Type:        graphql.String,
					Description: "Delete",
					Args: graphql.FieldConfigArgument{
						pkCol: &graphql.ArgumentConfig{
							Type: graphql.Int,
						},
					},
					Resolve: func(p graphql.ResolveParams) (interface{}, error) {
						id, ok := p.Args[pkCol].(int)
						if ok {
							return "", opts.Service.Remove(int64(id))
						}
						return nil, nil
					},
				},
			},
		},
	)
	return queryType, mutationType
}

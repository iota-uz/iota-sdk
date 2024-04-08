package service

import (
	"fmt"
	"github.com/doug-martin/goqu/v9"
	"github.com/doug-martin/goqu/v9/exp"
	"github.com/graphql-go/graphql"
	"github.com/graphql-go/graphql/language/ast"
	"github.com/iota-agency/iota-erp/pkg/utils"
)

type GraphQLAdapterOptions struct {
	Service Service
	Name    string
}

var sql2graphql = map[DataType]*graphql.Scalar{
	Boolean:          graphql.Boolean,
	Character:        graphql.String,
	CharacterVarying: graphql.String,
	Integer:          graphql.Int,
	BigSerial:        graphql.Int,
	Cidr:             graphql.String,
	Date:             graphql.String,
	DoublePrecision:  graphql.Float,
	Inet:             graphql.String,
	Json:             graphql.String,
	Jsonb:            graphql.String,
	Money:            graphql.Float,
	Numeric:          graphql.Float,
	Real:             graphql.Float,
	Text:             graphql.String,
	Time:             graphql.String,
	Timestamp:        graphql.String,
	Uuid:             graphql.String,
}

var StringOpToExpression = map[string]func(col interface{}) exp.SQLFunctionExpression{
	"min":   goqu.MIN,
	"max":   goqu.MAX,
	"avg":   goqu.AVG,
	"sum":   goqu.SUM,
	"count": goqu.COUNT,
}

var QueryToExpression = map[string]func(string, interface{}) exp.BooleanExpression{
	"gt": func(col string, val interface{}) exp.BooleanExpression {
		return goqu.C(col).Gt(val)
	},
	"gte": func(col string, val interface{}) exp.BooleanExpression {
		return goqu.C(col).Gte(val)
	},
	"lt": func(col string, val interface{}) exp.BooleanExpression {
		return goqu.C(col).Lt(val)
	},
	"lte": func(col string, val interface{}) exp.BooleanExpression {
		return goqu.C(col).Lte(val)
	},
	"in": func(col string, val interface{}) exp.BooleanExpression {
		return goqu.C(col).In(val)
	},
	"out": func(col string, val interface{}) exp.BooleanExpression {
		return goqu.C(col).NotIn(val)
	},
}

func modelToGQLType(model *Model, name string) *graphql.Object {
	fields := graphql.Fields{
		model.Pk.Name: &graphql.Field{
			Type: sql2graphql[model.Pk.Type],
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
			Name:   name,
			Fields: fields,
		},
	)
}

func modelToGQLTypeWithJoins(model *Model, name string) *graphql.Object {
	fields := graphql.Fields{
		model.Pk.Name: &graphql.Field{
			Type: sql2graphql[model.Pk.Type],
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
		if field.Association != nil {
			refModel := field.Association.To
			refName := fmt.Sprintf("%s%s", name, utils.Title(refModel.Table))
			fields[field.Association.As] = &graphql.Field{
				Type: modelToGQLTypeWithJoins(refModel, refName),
			}
		}
	}
	return graphql.NewObject(
		graphql.ObjectConfig{
			Name:   name,
			Fields: fields,
		},
	)
}

func (g *graphQLAdapter) paginatedQuery() *graphql.Object {
	return graphql.NewObject(graphql.ObjectConfig{
		Name: fmt.Sprintf("%sPaginated", utils.Title(g.name)),
		Fields: graphql.Fields{
			"total": &graphql.Field{
				Type: graphql.Int,
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					return g.service.Count(&CountQuery{})
				},
			},
			"data": &graphql.Field{
				Type: graphql.NewList(g.modelType),
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					attrs := getAttrs(p)
					query := goqu.From(g.model().Table).Select(attrs...)
					limit, ok := p.Info.VariableValues["limit"].(int)
					if ok {
						query.Limit(uint(limit))
					}
					offset, ok := p.Info.VariableValues["offset"].(int)
					if ok {
						query.Offset(uint(offset))
					}
					_sortBy, ok := p.Info.VariableValues["sortBy"].([]interface{})
					if ok {
						var sortBy []string
						for _, s := range _sortBy {
							sortBy = append(sortBy, s.(string))
						}
						query.Order(orderStringToExpression(sortBy)...)
					}
					data, err := g.service.ExecuteFind(query)
					if err != nil {
						return nil, err
					}
					return data, nil
				},
			},
		},
	})
}

func (g *graphQLAdapter) aggregateSubQuery() *graphql.Object {
	fields := graphql.Fields{}
	aggregationQuery := func(f *Field) *graphql.Object {
		queryFields := graphql.Fields{
			"groupBy": &graphql.Field{
				Type: graphql.NewList(graphql.String),
			},
			"count": &graphql.Field{
				Type: graphql.Int,
			},
		}
		if isTime(f.Type) || isNumeric(f.Type) {
			queryFields["min"] = &graphql.Field{
				Type: sql2graphql[f.Type],
			}
			queryFields["max"] = &graphql.Field{
				Type: sql2graphql[f.Type],
			}
		}
		if isNumeric(f.Type) {
			queryFields["avg"] = &graphql.Field{
				Type: sql2graphql[f.Type],
			}
			queryFields["sum"] = &graphql.Field{
				Type: sql2graphql[f.Type],
			}
		}
		return graphql.NewObject(
			graphql.ObjectConfig{
				Name:   fmt.Sprintf("%s%sAggregationQuery", utils.Title(g.name), utils.Title(f.Name)),
				Fields: queryFields,
			},
		)
	}
	for _, field := range g.model().Fields {
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
		if isTime(field.Type) || isNumeric(field.Type) {
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
			Name:   fmt.Sprintf("%sAggregate", utils.Title(g.name)),
			Fields: fields,
		},
	)
}

type graphQLAdapter struct {
	modelType *graphql.Object
	service   Service
	name      string
}

func (g *graphQLAdapter) model() *Model {
	return g.service.Model()
}

func (g *graphQLAdapter) pkName() string {
	return g.model().Pk.Name
}

func (g *graphQLAdapter) getQuery() *graphql.Field {
	return &graphql.Field{
		Type:        g.modelType,
		Description: "Get by id",
		Args: graphql.FieldConfigArgument{
			g.pkName(): &graphql.ArgumentConfig{
				Type: sql2graphql[g.model().Pk.Type],
			},
		},
		Resolve: func(p graphql.ResolveParams) (interface{}, error) {
			id, ok := p.Args[g.pkName()].(int)
			if !ok {
				return nil, nil
			}
			return g.service.Get(&GetQuery{
				Id:    int64(id),
				Attrs: getAttrs(p),
			})
		},
	}
}

func (g *graphQLAdapter) listQuery() *graphql.Field {
	modelType := modelToGQLTypeWithJoins(g.model(), fmt.Sprintf("%sJoinType", utils.Title(g.name)))
	model := g.model()
	// TODO: Add filtering & sorting
	return &graphql.Field{
		Type:        graphql.NewList(modelType),
		Description: "Get list",
		Resolve: func(p graphql.ResolveParams) (interface{}, error) {
			// TODO: Refactor && to whomever reads this, I'm deeply sorry, I was just trying to make it work
			data, err := g.service.ExecuteFind(ResolveToQuery(p, model))
			if err != nil {
				return nil, err
			}
			return data, nil
		},
	}
}

func (g *graphQLAdapter) listPaginatedQuery() *graphql.Field {
	return &graphql.Field{
		Type:        g.paginatedQuery(),
		Description: "Get paginated",
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
					g.pkName(),
				},
			},
		},
		Resolve: func(p graphql.ResolveParams) (interface{}, error) {
			return g.paginatedQuery(), nil
		},
	}
}

func (g *graphQLAdapter) aggregateQuery() *graphql.Field {
	return &graphql.Field{
		Type:        graphql.NewList(g.aggregateSubQuery()),
		Description: "Aggregate",
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
					g.pkName(),
				},
			},
		},
		Resolve: func(p graphql.ResolveParams) (interface{}, error) {
			root := p.Info.FieldASTs[0]
			aggregateQuery := &AggregateQuery{
				Query:       []goqu.Expression{},
				Expressions: []goqu.Expression{},
				GroupBy:     []string{},
				Sort:        []string{},
			}
			for _, _field := range root.SelectionSet.Selections {
				field := _field.(*ast.Field)
				for _, arg := range field.Arguments {
					c := QueryToExpression[arg.Name.Value]
					aggregateQuery.Query = append(aggregateQuery.Query, c(field.Name.Value, arg.Value.GetValue()))
				}
				for _, _op := range field.SelectionSet.Selections {
					op := _op.(*ast.Field)
					opName := op.Name.Value
					sqlOp := StringOpToExpression[opName]
					aggregateQuery.Expressions = append(
						aggregateQuery.Expressions,
						sqlOp(goqu.I(field.Name.Value)).As(goqu.C(fmt.Sprintf("%s.%s", field.Name.Value, opName))),
					)
				}
			}
			groupBy, ok := p.Args["groupBy"].([]interface{})
			if ok {
				for _, g := range groupBy {
					aggregateQuery.GroupBy = append(aggregateQuery.GroupBy, g.(string))
				}
			}
			sortBy, ok := p.Args["sortBy"].([]interface{})
			if ok {
				for _, s := range sortBy {
					aggregateQuery.Sort = append(aggregateQuery.Sort, s.(string))
				}
			}
			limit, ok := p.Args["limit"].(int)
			if ok {
				aggregateQuery.Limit = limit
			}
			offset, ok := p.Args["offset"].(int)
			if ok {
				aggregateQuery.Offset = offset
			}
			data, err := g.service.Aggregate(aggregateQuery)
			if err != nil {
				return nil, err
			}
			return data, nil
		},
	}
}

func (g *graphQLAdapter) QueryType() *graphql.Object {
	return graphql.NewObject(
		graphql.ObjectConfig{
			Name: utils.Title(g.name) + "Query",
			Fields: graphql.Fields{
				"get":           g.getQuery(),
				"list":          g.listQuery(),
				"listPaginated": g.listPaginatedQuery(),
				"aggregate":     g.aggregateQuery(),
			},
		},
	)
}

func (g *graphQLAdapter) MutationType() *graphql.Object {
	createArgs := graphql.FieldConfigArgument{}
	for _, field := range g.model().Fields {
		createArgs[field.Name] = &graphql.ArgumentConfig{
			Type: sql2graphql[field.Type],
		}
	}
	return graphql.NewObject(
		graphql.ObjectConfig{
			Name: utils.Title(g.name) + "Mutation",
			Fields: graphql.Fields{
				"create": &graphql.Field{
					Type:        g.modelType,
					Description: "Create",
					Args:        createArgs,
					Resolve: func(p graphql.ResolveParams) (interface{}, error) {
						return g.service.Create(p.Args)
					},
				},
				"update": &graphql.Field{
					Type:        g.modelType,
					Description: "Update",
					Args:        createArgs,
					Resolve: func(p graphql.ResolveParams) (interface{}, error) {
						id, ok := p.Args[g.pkName()].(int64)
						if ok {
							return g.service.Patch(id, p.Args)
						}
						return nil, nil
					},
				},
				"delete": &graphql.Field{
					Type:        graphql.String,
					Description: "Delete",
					Args: graphql.FieldConfigArgument{
						g.pkName(): &graphql.ArgumentConfig{
							Type: graphql.Int,
						},
					},
					Resolve: func(p graphql.ResolveParams) (interface{}, error) {
						id, ok := p.Args[g.pkName()].(int)
						if ok {
							return "", g.service.Remove(int64(id))
						}
						return nil, nil
					},
				},
			},
		},
	)
}

func (g *graphQLAdapter) ToGraphQL() (*graphql.Object, *graphql.Object) {
	return g.QueryType(), g.MutationType()
}

func GraphQLAdapter(opts *GraphQLAdapterOptions) (*graphql.Object, *graphql.Object) {
	return (&graphQLAdapter{
		service:   opts.Service,
		name:      opts.Name,
		modelType: modelToGQLType(opts.Service.Model(), fmt.Sprintf("%sType", utils.Title(opts.Name))),
	}).ToGraphQL()
}

package service

import (
	"fmt"
	"github.com/doug-martin/goqu/v9"
	"github.com/doug-martin/goqu/v9/exp"
	"github.com/graphql-go/graphql"
	"github.com/graphql-go/graphql/language/ast"
	"github.com/iota-agency/iota-erp/pkg/utils"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"strings"
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
		model.Pk.Name: &graphql.Field{
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
	strings := []DataType{
		Character,
		CharacterVarying,
		Text,
	}
	return utils.Includes(strings, kind)
}

func isTime(kind DataType) bool {
	times := []DataType{
		Date,
		Time,
		Timestamp,
	}
	return utils.Includes(times, kind)
}

func graphqlAggregateQuery(opts *GraphQLAdapterOptions) *graphql.Object {
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
		caser := cases.Title(language.English)
		return graphql.NewObject(
			graphql.ObjectConfig{
				Name:   fmt.Sprintf("%s%sAggregationQuery", caser.String(opts.Name), caser.String(f.Name)),
				Fields: queryFields,
			},
		)
	}
	for _, field := range opts.Service.Model().Fields {
		gqlType, ok := sql2graphql[field.Type]
		if !ok {
			panic(fmt.Sprintf("Type %s not found", field.Type))
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
			Name:   fmt.Sprintf("%sAggregate", opts.Name),
			Fields: fields,
		},
	)
}

func GraphQLAdapter(opts *GraphQLAdapterOptions) (*graphql.Object, *graphql.Object) {
	pkCol := opts.Service.Model().Pk.Name
	modelType := modelToGraphQLObject(opts.Name, opts.Service.Model())
	paginatedQ := paginatedQuery(opts.Service, modelType)
	aggregateType := graphqlAggregateQuery(opts)
	queryType := graphql.NewObject(
		graphql.ObjectConfig{
			Name: opts.Name,
			Fields: graphql.Fields{
				"get": &graphql.Field{
					Type:        modelType,
					Description: "Get by id",
					Args: graphql.FieldConfigArgument{
						pkCol: &graphql.ArgumentConfig{
							Type: sql2graphql[opts.Service.Model().Pk.Type],
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
					Type:        graphql.NewList(aggregateType),
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
								pkCol,
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
						data, err := opts.Service.Aggregate(aggregateQuery)
						if err != nil {
							return nil, err
						}
						var result []map[string]interface{}
						for _, row := range data {
							result = append(result, nestMap(row))
						}
						return result, nil
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

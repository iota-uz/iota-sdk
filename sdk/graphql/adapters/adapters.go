package adapters

import (
	"fmt"
	"github.com/doug-martin/goqu/v9"
	"github.com/doug-martin/goqu/v9/exp"
	"github.com/graphql-go/graphql"
	"github.com/iota-agency/iota-erp/pkg/utils"
	"github.com/iota-agency/iota-erp/sdk/db/dbutils"
	"github.com/iota-agency/iota-erp/sdk/graphql/resolvers"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
	"reflect"
	"sync"
)

var sql2graphql = map[schema.DataType]*graphql.Scalar{
	schema.Int:    graphql.Int,
	schema.String: graphql.String,
	schema.Float:  graphql.Float,
	schema.Time:   graphql.DateTime,
	schema.Bool:   graphql.Boolean,
	schema.Bytes:  graphql.String,
	schema.Uint:   graphql.Int,
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

func GqlTypeFromModel(model interface{}, name string) (*graphql.Object, error) {
	s, err := schema.Parse(model, &sync.Map{}, schema.NamingStrategy{})
	if err != nil {
		return nil, err
	}
	gqlFields := graphql.Fields{}
	for _, field := range s.Fields {
		if !field.Readable {
			continue
		}
		as := field.Tag.Get("gql")
		if as == "-" {
			continue
		}
		if field.DataType == "" {
			refName := fmt.Sprintf("%s%sJoin", name, as)
			obj, err := GqlTypeFromModel(reflect.New(field.FieldType).Elem().Interface(), refName)
			if err != nil {
				return nil, err
			}
			gqlFields[as] = &graphql.Field{
				Type: obj,
			}
		} else {
			t, ok := sql2graphql[field.DataType]
			if !ok {
				panic(fmt.Sprintf("Type %v not found for field %s", field.DataType, field.DBName))
			}
			gqlFields[field.DBName] = &graphql.Field{
				Type: t,
			}
		}
	}
	return graphql.NewObject(graphql.ObjectConfig{
		Name:   name,
		Fields: gqlFields,
	}), nil
}

func CreateArgsFromModel(model interface{}) (graphql.InputObjectConfigFieldMap, error) {
	s, err := schema.Parse(model, &sync.Map{}, schema.NamingStrategy{})
	if err != nil {
		return nil, nil
	}
	args := graphql.InputObjectConfigFieldMap{}
	for _, field := range s.Fields {
		if !field.Creatable {
			continue
		}
		if field.DataType == "" {
			as := field.Tag.Get("gql")
			obj, err := CreateArgsFromModel(reflect.New(field.FieldType).Elem().Interface())
			if err != nil {
				return nil, err
			}
			args[as] = &graphql.InputObjectFieldConfig{
				Type: graphql.NewInputObject(graphql.InputObjectConfig{
					Name:   fmt.Sprintf("%sCreateInput", utils.Title(as)),
					Fields: obj,
				}),
			}
		} else {
			args[field.DBName] = &graphql.InputObjectFieldConfig{
				Type: sql2graphql[field.DataType],
			}
		}
	}
	return args, nil
}

func UpdateArgsFromModel(model interface{}) (graphql.InputObjectConfigFieldMap, error) {
	s, err := schema.Parse(model, &sync.Map{}, schema.NamingStrategy{})
	if err != nil {
		return nil, nil
	}
	args := graphql.InputObjectConfigFieldMap{}
	for _, field := range s.Fields {
		if !field.Updatable {
			continue
		}
		if field.DataType == "" {
			as := field.Tag.Get("gql")
			obj, err := CreateArgsFromModel(reflect.New(field.FieldType).Elem().Interface())
			if err != nil {
				return nil, err
			}
			args[as] = &graphql.InputObjectFieldConfig{
				Type: graphql.NewInputObject(graphql.InputObjectConfig{
					Name:   fmt.Sprintf("%sUpdateInput", utils.Title(as)),
					Fields: obj,
				}),
			}
		} else {
			args[field.DBName] = &graphql.InputObjectFieldConfig{
				Type: sql2graphql[field.DataType],
			}
		}
	}
	return args, nil
}

func GetQuery(db *gorm.DB, model interface{}, modelType *graphql.Object, name string) *graphql.Field {
	pk, err := dbutils.GetModelPk(model)
	if err != nil {
		panic(err)
	}
	return &graphql.Field{
		Name:        name,
		Type:        modelType,
		Description: "Get by id",
		Args: graphql.FieldConfigArgument{
			pk.Name: &graphql.ArgumentConfig{
				Type: sql2graphql[pk.DataType],
			},
		},
		Resolve: resolvers.DefaultGetResolver(db, model),
	}
}

func ListPaginatedQuery(db *gorm.DB, model interface{}, modelType *graphql.Object, name string) *graphql.Field {
	pk, err := dbutils.GetModelPk(model)
	if err != nil {
		panic(err)
	}
	paginationType := graphql.NewObject(graphql.ObjectConfig{
		Name: name,
		Fields: graphql.Fields{
			"total": &graphql.Field{
				Type:    graphql.Int,
				Resolve: resolvers.DefaultCountResolver(db, model),
			},
			"data": &graphql.Field{
				Type:    graphql.NewList(modelType),
				Resolve: resolvers.DefaultPaginationResolver(db, model),
			},
		},
	})
	return &graphql.Field{
		Name:        name,
		Type:        paginationType,
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
					pk.Name,
				},
			},
		},
		Resolve: func(p graphql.ResolveParams) (interface{}, error) {
			return p.Args, nil
		},
	}
}

func DefaultQueries(db *gorm.DB, model interface{}, singular, plural string) []*graphql.Field {
	modelType, err := GqlTypeFromModel(model, utils.Title(singular))
	if err != nil {
		panic(err)
	}
	return []*graphql.Field{
		ListPaginatedQuery(db, model, modelType, plural),
		GetQuery(db, model, modelType, singular),
		AggregateQuery(db, model, fmt.Sprintf("%sAggregate", plural)),
	}
}

func DefaultMutations(db *gorm.DB, model interface{}, name string) []*graphql.Field {
	pk, err := dbutils.GetModelPk(model)
	if err != nil {
		panic(err)
	}
	modelType, err := GqlTypeFromModel(model, fmt.Sprintf("%sType", utils.Title(name)))
	if err != nil {
		panic(err)
	}
	createArgs, err := CreateArgsFromModel(model)
	if err != nil {
		panic(err)
	}

	updateArgs, err := UpdateArgsFromModel(model)
	if err != nil {
		panic(err)
	}
	return []*graphql.Field{
		{
			Name:        fmt.Sprintf("create%s", utils.Title(name)),
			Type:        modelType,
			Description: "Create a record",
			Args: graphql.FieldConfigArgument{
				"data": &graphql.ArgumentConfig{
					Type: graphql.NewInputObject(graphql.InputObjectConfig{
						Name:   fmt.Sprintf("Create%sInput", utils.Title(name)),
						Fields: createArgs,
					}),
				},
			},
			Resolve: resolvers.DefaultCreateResolver(db, model),
		},
		{
			Name:        fmt.Sprintf("update%s", utils.Title(name)),
			Type:        modelType,
			Description: "Update a record",
			Args: graphql.FieldConfigArgument{
				"id": &graphql.ArgumentConfig{
					Type: graphql.NewNonNull(sql2graphql[pk.DataType]),
				},
				"data": &graphql.ArgumentConfig{
					Type: graphql.NewInputObject(graphql.InputObjectConfig{
						Name:   fmt.Sprintf("Update%sInput", utils.Title(name)),
						Fields: updateArgs,
					}),
				},
			},
			Resolve: resolvers.DefaultUpdateResolver(db, model),
		},
		{
			Name:        fmt.Sprintf("delete%s", utils.Title(name)),
			Type:        graphql.String,
			Description: "Delete a record",
			Args: graphql.FieldConfigArgument{
				pk.Name: &graphql.ArgumentConfig{
					Type: sql2graphql[pk.DataType],
				},
			},
			Resolve: resolvers.DefaultDeleteResolver(db, model),
		},
	}
}

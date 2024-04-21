package adapters

import (
	"fmt"
	"github.com/doug-martin/goqu/v9"
	"github.com/doug-martin/goqu/v9/exp"
	"github.com/graphql-go/graphql"
	"github.com/iota-agency/iota-erp/models"
	"github.com/iota-agency/iota-erp/pkg/server/graphql/resolvers"
	"github.com/iota-agency/iota-erp/pkg/utils"
	"github.com/jmoiron/sqlx"
)

var sql2graphql = map[models.DataType]*graphql.Scalar{
	models.BigInt:           graphql.Int,
	models.Boolean:          graphql.Boolean,
	models.Character:        graphql.String,
	models.CharacterVarying: graphql.String,
	models.Integer:          graphql.Int,
	models.BigSerial:        graphql.Int,
	models.Cidr:             graphql.String,
	models.Date:             graphql.String,
	models.DoublePrecision:  graphql.Float,
	models.Inet:             graphql.String,
	models.Json:             graphql.String,
	models.Jsonb:            graphql.String,
	models.Money:            graphql.Float,
	models.Numeric:          graphql.Float,
	models.Real:             graphql.Float,
	models.Text:             graphql.String,
	models.Time:             graphql.String,
	models.Timestamp:        graphql.String,
	models.Uuid:             graphql.String,
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

func GqlTypeFromModel(model models.Model, name string) *graphql.Object {
	fields := graphql.Fields{}
	for _, field := range models.Fields(model) {
		if field.Association == nil {
			t, ok := sql2graphql[field.Type]
			if !ok {
				panic(fmt.Sprintf("Type %v not found", field.Type))
			}
			fields[field.Name] = &graphql.Field{
				Type: t,
			}
		} else {
			refModel := field.Association.To
			refName := fmt.Sprintf("%s%s", name, utils.Title(refModel.Table()))
			fields[field.Association.As] = &graphql.Field{
				Type: GqlTypeFromModel(refModel, refName),
			}
		}
	}
	return graphql.NewObject(graphql.ObjectConfig{
		Name:   name,
		Fields: fields,
	})
}

func GetQuery(db *sqlx.DB, model models.Model, modelType *graphql.Object, name string) *graphql.Field {
	pk := model.PkField()
	return &graphql.Field{
		Name:        name,
		Type:        modelType,
		Description: "Get by id",
		Args: graphql.FieldConfigArgument{
			pk.Name: &graphql.ArgumentConfig{
				Type: sql2graphql[pk.Type],
			},
		},
		Resolve: resolvers.DefaultGetResolver(db, model),
	}
}

func ListPaginatedQuery(db *sqlx.DB, model models.Model, modelType *graphql.Object, name string) *graphql.Field {
	pk := model.PkField()
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
			return paginationType, nil
		},
	}
}

func DefaultQueries(db *sqlx.DB, model models.Model, singular, plural string) []*graphql.Field {
	modelType := GqlTypeFromModel(model, utils.Title(singular))
	return []*graphql.Field{
		ListPaginatedQuery(db, model, modelType, plural),
		GetQuery(db, model, modelType, singular),
		AggregateQuery(db, model, fmt.Sprintf("%sAggregate", plural)),
	}
}

func DefaultMutations(db *sqlx.DB, model models.Model, name string) []*graphql.Field {
	pk := model.PkField()
	modelType := GqlTypeFromModel(model, fmt.Sprintf("%sType", utils.Title(name)))
	createArgs := graphql.InputObjectConfigFieldMap{}
	for _, field := range models.Fields(model) {
		if field.Name == pk.Name {
			continue
		}
		createArgs[field.Name] = &graphql.InputObjectFieldConfig{
			Type: sql2graphql[field.Type],
		}
	}
	return []*graphql.Field{
		{
			Name:        fmt.Sprintf("create%s", utils.Title(name)),
			Type:        modelType,
			Description: "Create a record",
			Args: graphql.FieldConfigArgument{
				"data": &graphql.ArgumentConfig{
					Type: graphql.NewInputObject(graphql.InputObjectConfig{
						Name: fmt.Sprintf("Create%sInput", utils.Title(name)), Fields: createArgs,
					}),
				},
			},
			Resolve: resolvers.DefaultCreateResolver(db, model.Table()),
		},
		{
			Name:        fmt.Sprintf("update%s", utils.Title(name)),
			Type:        modelType,
			Description: "Update a record",
			Args: graphql.FieldConfigArgument{
				"id": &graphql.ArgumentConfig{
					Type: graphql.NewNonNull(sql2graphql[pk.Type]),
				},
				"data": &graphql.ArgumentConfig{
					Type: graphql.NewInputObject(graphql.InputObjectConfig{
						Name: fmt.Sprintf("Update%sInput", utils.Title(name)), Fields: createArgs,
					}),
				},
			},
			Resolve: resolvers.DefaultUpdateResolver(db, model.Table(), pk.Name),
		},
		{
			Name:        fmt.Sprintf("delete%s", utils.Title(name)),
			Type:        graphql.String,
			Description: "Delete a record",
			Args: graphql.FieldConfigArgument{
				pk.Name: &graphql.ArgumentConfig{
					Type: sql2graphql[pk.Type],
				},
			},
			Resolve: resolvers.DefaultDeleteResolver(db, model.Table(), pk.Name),
		},
	}
}

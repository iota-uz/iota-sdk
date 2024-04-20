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
	return &graphql.Field{
		Name: name,
		Type: graphql.NewObject(graphql.ObjectConfig{
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
		}),
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
	createArgs := graphql.FieldConfigArgument{}
	for _, field := range models.Fields(model) {
		createArgs[field.Name] = &graphql.ArgumentConfig{
			Type: sql2graphql[field.Type],
		}
	}
	updateArgs := createArgs
	updateArgs[pk.Name] = &graphql.ArgumentConfig{
		Type: sql2graphql[model.PkField().Type],
	}
	return []*graphql.Field{
		{
			Name:        fmt.Sprintf("create%s", name),
			Type:        modelType,
			Description: fmt.Sprintf("Create %s", name),
			Args:        createArgs,
			Resolve:     resolvers.DefaultCreateResolver(db, model.Table()),
		},
		{
			Name:        fmt.Sprintf("update%s", name),
			Type:        modelType,
			Description: fmt.Sprintf("Update %s", name),
			Args:        updateArgs,
			Resolve:     resolvers.DefaultUpdateResolver(db, model.Table(), pk.Name),
		},
		{
			Name:        fmt.Sprintf("delete%s", name),
			Type:        graphql.String,
			Description: fmt.Sprintf("Delete %s", name),
			Args: graphql.FieldConfigArgument{
				pk.Name: &graphql.ArgumentConfig{
					Type: sql2graphql[pk.Type],
				},
			},
			Resolve: resolvers.DefaultDeleteResolver(db, model.Table(), pk.Name),
		},
	}
}

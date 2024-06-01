package resolvers

import (
	"github.com/graphql-go/graphql"
	"github.com/iota-agency/iota-erp/sdk/db/dbutils"
	adapters2 "github.com/iota-agency/iota-erp/sdk/graphql/old/adapters"
	"gorm.io/gorm"
)

func GetQuery(db *gorm.DB, model interface{}, modelType *graphql.Object, name string) *graphql.Field {
	pk, err := dbutils.GetModelPk(model)
	if err != nil {
		panic(err)
	}
	as := pk.Tag.Get("gql")
	return &graphql.Field{
		Name:        name,
		Type:        modelType,
		Description: "Get by id",
		Args: graphql.FieldConfigArgument{
			as: &graphql.ArgumentConfig{
				Type: adapters2.Sql2graphql[pk.DataType],
			},
		},
		Resolve: DefaultGetResolver(db, model),
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
				Resolve: DefaultCountResolver(db, model),
			},
			"data": &graphql.Field{
				Type:    graphql.NewList(modelType),
				Resolve: DefaultPaginationResolver(db, model),
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

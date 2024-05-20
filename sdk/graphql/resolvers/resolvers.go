package resolvers

import (
	"errors"
	"fmt"
	"github.com/graphql-go/graphql"
	"github.com/iota-agency/iota-erp/sdk/db/dbutils"
	"github.com/iota-agency/iota-erp/sdk/graphql/adapters"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
	"strings"
)

func ApplySort(query *gorm.DB, sortBy []interface{}, model interface{}) (*gorm.DB, error) {
	mapping, err := adapters.GetGormFields(model, func(f *schema.Field, gf *adapters.GqlFieldMeta) bool {
		return f.Readable && gf.Readable
	})
	if err != nil {
		return nil, err
	}
	for _, s := range sortBy {
		parts := strings.Split(s.(string), " ")
		if len(parts) > 2 {
			return nil, errors.New(fmt.Sprintf("invalid sort field %s", s))
		}
		if len(parts) == 2 && parts[1] != "asc" && parts[1] != "desc" {
			return nil, errors.New(fmt.Sprintf("invalid sort order %s", parts[1]))
		}
		field, ok := mapping[parts[0]]
		if !ok {
			return nil, errors.New(fmt.Sprintf("field %s not found", s))
		}
		query = query.Order(fmt.Sprintf("%s %s", field.DBName, parts[1]))
	}
	return query, nil
}

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
				Type: adapters.Sql2graphql[pk.DataType],
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

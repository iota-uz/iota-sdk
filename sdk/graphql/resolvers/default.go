package resolvers

import (
	"errors"
	"fmt"
	"github.com/graphql-go/graphql"
	"github.com/iota-agency/iota-erp/pkg/utils"
	"github.com/iota-agency/iota-erp/sdk/db/dbutils"
	"github.com/iota-agency/iota-erp/sdk/graphql/adapters"
	"gorm.io/gorm"
)

func DefaultCreateResolver(db *gorm.DB, model interface{}) graphql.FieldResolveFn {
	return func(p graphql.ResolveParams) (interface{}, error) {
		data := p.Args["data"].(map[string]interface{})
		if err := db.Model(model).Create(data).Error; err != nil {
			return nil, err
		}
		return data, nil
	}
}

func DefaultUpdateResolver(db *gorm.DB, model interface{}) graphql.FieldResolveFn {
	pk, err := dbutils.GetModelPk(model)
	if err != nil {
		panic(err)
	}
	return func(p graphql.ResolveParams) (interface{}, error) {
		id, ok := p.Args[pk.DBName].(int)
		if !ok {
			return nil, errors.New(fmt.Sprintf("Invalid %s", pk.Name))
		}
		data := p.Args["data"].(map[string]interface{})
		if err := db.Model(model).Where(pk.DBName, id).Updates(data).Error; err != nil {
			return nil, err
		}
		return data, nil
	}
}

func DefaultDeleteResolver(db *gorm.DB, model interface{}) graphql.FieldResolveFn {
	pk, err := dbutils.GetModelPk(model)
	if err != nil {
		panic(err)
	}
	return func(p graphql.ResolveParams) (interface{}, error) {
		id, ok := p.Args[pk.DBName].(int)
		if !ok {
			return nil, errors.New(fmt.Sprintf("Invalid %s", pk.DBName))
		}
		if err := db.Model(model).Where(pk.DBName, id).Delete(model).Error; err != nil {
			return nil, err
		}
		return id, nil
	}
}

func DefaultGetResolver(db *gorm.DB, model interface{}) graphql.FieldResolveFn {
	pk, err := dbutils.GetModelPk(model)
	if err != nil {
		panic(err)
	}
	return func(p graphql.ResolveParams) (interface{}, error) {
		id, err := adapters.CastId(p.Args, pk)
		if err != nil {
			return nil, err
		}
		var dest map[string]interface{}
		if err := db.Model(model).First(&dest, id).Error; err != nil {
			return nil, err
		}
		return NestMap(model, dest), nil
	}
}

func DefaultCountResolver(db *gorm.DB, model interface{}) graphql.FieldResolveFn {
	return func(p graphql.ResolveParams) (interface{}, error) {
		var count int64
		if err := db.Model(model).Count(&count).Error; err != nil {
			return nil, err
		}
		return count, nil
	}
}

func DefaultQueries(db *gorm.DB, model interface{}, singular, plural string) []*graphql.Field {
	modelType, err := adapters.ReadTypeFromModel(model, utils.Title(singular))
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
	modelType, err := adapters.ReadTypeFromModel(model, fmt.Sprintf("%sType", utils.Title(name)))
	if err != nil {
		panic(err)
	}
	createArgs, err := adapters.CreateArgsFromModel(model)
	if err != nil {
		panic(err)
	}

	updateArgs, err := adapters.UpdateArgsFromModel(model)
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
			Resolve: DefaultCreateResolver(db, model),
		},
		{
			Name:        fmt.Sprintf("update%s", utils.Title(name)),
			Type:        modelType,
			Description: "Update a record",
			Args: graphql.FieldConfigArgument{
				"id": &graphql.ArgumentConfig{
					Type: graphql.NewNonNull(adapters.Sql2graphql[pk.DataType]),
				},
				"data": &graphql.ArgumentConfig{
					Type: graphql.NewInputObject(graphql.InputObjectConfig{
						Name:   fmt.Sprintf("Update%sInput", utils.Title(name)),
						Fields: updateArgs,
					}),
				},
			},
			Resolve: DefaultUpdateResolver(db, model),
		},
		{
			Name:        fmt.Sprintf("delete%s", utils.Title(name)),
			Type:        graphql.String,
			Description: "Delete a record",
			Args: graphql.FieldConfigArgument{
				pk.Name: &graphql.ArgumentConfig{
					Type: adapters.Sql2graphql[pk.DataType],
				},
			},
			Resolve: DefaultDeleteResolver(db, model),
		},
	}
}

func DefaultPaginationResolver(db *gorm.DB, model interface{}) graphql.FieldResolveFn {
	return func(p graphql.ResolveParams) (interface{}, error) {
		query := db.Model(model)
		args := p.Source.(map[string]interface{})
		limit, ok := args["limit"].(int)
		if ok {
			query = query.Limit(limit)
		}
		offset, ok := args["offset"].(int)
		if ok {
			query = query.Offset(offset)
		}
		sortBy, ok := args["sortBy"].([]interface{})
		if ok {
			if q, err := ApplySort(query, sortBy, model); err == nil {
				query = q
			} else {
				return nil, err
			}
		}
		associations := GetAssociations(model, p.Info.FieldASTs[0].SelectionSet)
		for _, a := range associations {
			query = query.Joins(a)
		}
		var result []map[string]interface{}
		if err := query.Find(&result).Error; err != nil {
			return nil, err
		}
		for i, r := range result {
			result[i] = NestMap(model, r)
		}
		return result, nil
	}
}

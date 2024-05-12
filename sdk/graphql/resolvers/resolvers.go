package resolvers

import (
	"errors"
	"fmt"
	"github.com/graphql-go/graphql"
	"github.com/iota-agency/iota-erp/sdk/db/dbutils"
	"gorm.io/gorm"
)

func DefaultGetResolver(db *gorm.DB, model interface{}) graphql.FieldResolveFn {
	pk, err := dbutils.GetModelPk(model)
	if err != nil {
		panic(err)
	}
	return func(p graphql.ResolveParams) (interface{}, error) {
		id, ok := p.Args[pk.DBName].(int)
		if !ok {
			return nil, errors.New(fmt.Sprintf("Invalid %s", pk.DBName))
		}
		var dest map[string]interface{}
		if err := db.First(&dest, id).Error; err != nil {
			return nil, err
		}
		return dest, nil
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
			for _, s := range sortBy {
				query = query.Order(s.(string))
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

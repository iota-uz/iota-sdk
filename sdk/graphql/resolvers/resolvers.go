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
		var result []map[string]interface{}
		query := db.Model(model)
		limit, ok := p.Args["limit"].(int)
		if ok {
			query = query.Limit(limit)
		}
		offset, ok := p.Args["offset"].(int)
		if ok {
			query = query.Offset(offset)
		}
		if err := query.Find(&result).Error; err != nil {
			return nil, err
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

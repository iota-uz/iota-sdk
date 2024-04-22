package resolvers

import (
	"errors"
	"fmt"
	"github.com/doug-martin/goqu/v9"
	"github.com/graphql-go/graphql"
	"github.com/iota-agency/iota-erp/models"
	"github.com/iota-agency/iota-erp/pkg/server/graphql/dbutils"
	"github.com/jmoiron/sqlx"
)

func DefaultGetResolver(db *sqlx.DB, model models.Model) graphql.FieldResolveFn {
	pkName := model.PkField().Name
	return func(p graphql.ResolveParams) (interface{}, error) {
		id, ok := p.Args[pkName].(int)
		if !ok {
			return nil, errors.New(fmt.Sprintf("Invalid %s", pkName))
		}
		query := ResolveToQuery(p.Info.FieldASTs[0].SelectionSet, model).Where(goqu.Ex{
			fmt.Sprintf("%s.%s", model.Table(), pkName): id,
		})
		return dbutils.Get(db, query)
	}
}

func DefaultCountResolver(db *sqlx.DB, model models.Model) graphql.FieldResolveFn {
	return func(p graphql.ResolveParams) (interface{}, error) {
		query := goqu.From(model.Table())
		return dbutils.Count(db, query)
	}
}

func DefaultPaginationResolver(db *sqlx.DB, model models.Model) graphql.FieldResolveFn {
	return func(p graphql.ResolveParams) (interface{}, error) {
		query := ResolveToQuery(p.Info.FieldASTs[0].SelectionSet, model)
		limit, ok := p.Info.VariableValues["limit"].(int)
		if ok {
			query.Limit(uint(limit))
		}
		offset, ok := p.Info.VariableValues["offset"].(int)
		if ok {
			query.Offset(uint(offset))
		}
		query.Order(OrderedExpressionsFromResolveParams(p)...)
		return dbutils.Find(db, query)
	}
}

func DefaultCreateResolver(db *sqlx.DB, model models.Model) graphql.FieldResolveFn {
	return func(p graphql.ResolveParams) (interface{}, error) {
		data := p.Args["data"].(map[string]interface{})
		for _, field := range models.Refs(model) {
			if subData, ok := data[field.Association.As]; ok {
				delete(data, field.Association.As)
				subModel := field.Association.To
				subDataMap, ok := subData.(map[string]interface{})
				if !ok {
					return nil, errors.New(fmt.Sprintf("Invalid %s", field.Name))
				}
				q := goqu.Insert(subModel.Table()).Rows(subDataMap)
				if _, err := dbutils.Create(db, q); err != nil {
					return nil, err
				}
			}
		}
		query := goqu.Insert(model.Table()).Rows(data)
		return dbutils.Create(db, query)
	}
}

func DefaultUpdateResolver(db *sqlx.DB, model models.Model) graphql.FieldResolveFn {
	pk := model.PkField()
	return func(p graphql.ResolveParams) (interface{}, error) {
		id, ok := p.Args[pk.Name].(int)
		if !ok {
			return nil, errors.New(fmt.Sprintf("Invalid %s", pk.Name))
		}
		data := p.Args["data"].(map[string]interface{})
		for _, field := range models.Refs(model) {
			if subData, ok := data[field.Association.As]; ok {
				delete(data, field.Association.As)
				subModel := field.Association.To
				subDataMap, ok := subData.(map[string]interface{})
				if !ok {
					return nil, errors.New(fmt.Sprintf("Invalid %s", field.Name))
				}
				subDataMap[field.Association.Column] = id
				q := goqu.Insert(subModel.Table()).Rows(subDataMap)
				if _, err := dbutils.Create(db, q); err != nil {
					return nil, err
				}
			}
		}
		query := goqu.Update(model.Table()).Set(data).Where(goqu.Ex{
			pk.Name: id,
		})
		return dbutils.Patch(db, query)
	}
}

func DefaultDeleteResolver(db *sqlx.DB, tableName, pkName string) graphql.FieldResolveFn {
	return func(p graphql.ResolveParams) (interface{}, error) {
		id, ok := p.Args[pkName].(int)
		if !ok {
			return nil, errors.New(fmt.Sprintf("Invalid %s", pkName))
		}
		query := goqu.Delete(tableName).Where(goqu.Ex{
			pkName: id,
		})
		return true, dbutils.Delete(db, query)
	}
}

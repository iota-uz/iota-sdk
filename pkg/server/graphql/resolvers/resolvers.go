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
		query := goqu.From(model.Table()).Select(GetAttrs(p.Info.FieldASTs[0].SelectionSet)...).Where(goqu.Ex{
			pkName: id,
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

func DefaultCreateResolver(db *sqlx.DB, tableName string) graphql.FieldResolveFn {
	return func(p graphql.ResolveParams) (interface{}, error) {
		query := goqu.Insert(tableName).Rows(p.Args)
		return dbutils.Create(db, query)
	}
}

func DefaultUpdateResolver(db *sqlx.DB, tableName, pkName string) graphql.FieldResolveFn {
	return func(p graphql.ResolveParams) (interface{}, error) {
		id, ok := p.Args[pkName].(int)
		if !ok {
			return nil, errors.New(fmt.Sprintf("Invalid %s", pkName))
		}
		query := goqu.Update(tableName).Set(p.Args).Where(goqu.Ex{
			pkName: id,
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

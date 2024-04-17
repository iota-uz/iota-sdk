package resolvers

import (
	"errors"
	"github.com/doug-martin/goqu/v9"
	"github.com/graphql-go/graphql"
	"github.com/iota-agency/iota-erp/pkg/server/graphql/service"
	"github.com/jmoiron/sqlx"
)

func DefaultCreateResolver(db *sqlx.DB) graphql.ResolveTypeFn {
	return func(p graphql.ResolveParams) (interface{}, error) {
		id, ok := p.Args[g.pkName()].(int)
		if !ok {
			return nil, errors.New("id is required")
		}
		query := goqu.Update(g.model.Table).Set(p.Args).Where(goqu.Ex{
			g.pkName(): id,
		})
		return service.Patch(db, query)
	}
}

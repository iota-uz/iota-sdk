package expenses

import (
	"github.com/graphql-go/graphql"
	"github.com/iota-agency/iota-erp/models"
	"github.com/iota-agency/iota-erp/pkg/server/graphql/adapters"
	"github.com/jmoiron/sqlx"
)

func Queries(db *sqlx.DB) []*graphql.Field {
	return adapters.DefaultQueries(db, &models.Expense{}, "expense", "expenses")
}

func Mutations(db *sqlx.DB) []*graphql.Field {
	return adapters.DefaultMutations(db, &models.Expense{}, "expense")
}

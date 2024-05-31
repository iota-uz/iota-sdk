package expenses

import (
	"github.com/graphql-go/graphql"
	"github.com/iota-agency/iota-erp/models"
	"github.com/iota-agency/iota-erp/sdk/graphql/old/resolvers"
	"gorm.io/gorm"
)

func Queries(db *gorm.DB) []*graphql.Field {
	return resolvers.DefaultQueries(db, &models.Expense{}, "expense", "expenses")
}

func Mutations(db *gorm.DB) []*graphql.Field {
	return resolvers.DefaultMutations(db, &models.Expense{}, "expense")
}

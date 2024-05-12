package positions

import (
	"github.com/graphql-go/graphql"
	"github.com/iota-agency/iota-erp/models"
	"github.com/iota-agency/iota-erp/sdk/graphql/adapters"
	"gorm.io/gorm"
)

func Queries(db *gorm.DB) []*graphql.Field {
	return adapters.DefaultQueries(db, &models.Position{}, "position", "positions")
}

func Mutations(db *gorm.DB) []*graphql.Field {
	return adapters.DefaultMutations(db, &models.Position{}, "position")
}

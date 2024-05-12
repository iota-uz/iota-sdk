package routes

import (
	"github.com/graphql-go/graphql"
	"gorm.io/gorm"
)

type GraphQLConstructor func(db *gorm.DB) []*graphql.Field

package routes

import (
	"github.com/graphql-go/graphql"
	"github.com/jmoiron/sqlx"
)

type GraphQLConstructor func(db *sqlx.DB) (*graphql.Object, *graphql.Object)

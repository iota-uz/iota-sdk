package graph

//go:generate bash ../scripts/merge_graphql_schemas.sh  // Run our script first (adjust path if needed)
//go:generate go run github.com/99designs/gqlgen generate // Run gqlgen generate AFTER merging

package users

import (
	"github.com/graphql-go/graphql"
	"github.com/iota-agency/iota-erp/models"
	"github.com/iota-agency/iota-erp/pkg/server/graphql/adapters"
	"github.com/iota-agency/iota-erp/pkg/server/graphql/resolvers"
	"github.com/jmoiron/sqlx"
)

func CreateUser(db *sqlx.DB) graphql.FieldResolveFn {
	return func(p graphql.ResolveParams) (interface{}, error) {
		data := p.Args["data"].(map[string]interface{})
		user := &models.User{
			FirstName: data["first_name"].(string),
			LastName:  data["last_name"].(string),
			Email:     data["email"].(string),
			Password:  data["password"].(string),
		}
		if errs := user.Validate(); len(errs) != 0 {
			return nil, errs[0]
		}
		if err := user.SetPassword(user.Password); err != nil {
			return nil, err
		}
		if err := models.Insert(db, user); err != nil {
			return nil, err
		}
		return user, nil
	}
}

func UpdateUser(db *sqlx.DB) graphql.FieldResolveFn {
	return func(p graphql.ResolveParams) (interface{}, error) {
		data := p.Args["data"].(map[string]interface{})
		user := &models.User{
			Id:        int64(data["id"].(int)),
			FirstName: data["first_name"].(string),
			LastName:  data["last_name"].(string),
			Email:     data["email"].(string),
			Password:  data["password"].(string),
		}
		if errs := user.Validate(); len(errs) != 0 {
			return nil, errs[0]
		}
		if err := user.SetPassword(user.Password); err != nil {
			return nil, err
		}
		if err := models.Update(db, user); err != nil {
			return nil, err
		}
		return user, nil
	}
}

func Queries(db *sqlx.DB) []*graphql.Field {
	userType := adapters.GqlTypeFromModel(&models.User{}, "User")
	return []*graphql.Field{
		adapters.AggregateQuery(db, &models.User{}, "users"),
		adapters.ListPaginatedQuery(db, &models.User{}, userType, "users"),
		adapters.GetQuery(db, &models.User{}, userType, "user"),
	}
}

func Mutations(db *sqlx.DB) []*graphql.Field {
	userType := graphql.NewObject(graphql.ObjectConfig{
		Name: "UserType",
		Fields: graphql.Fields{
			"id": &graphql.Field{
				Type: graphql.Int,
			},
			"email": &graphql.Field{
				Type: graphql.String,
			},
			"first_name": &graphql.Field{
				Type: graphql.String,
			},
			"last_name": &graphql.Field{
				Type: graphql.String,
			},
			"avatar_id": &graphql.Field{
				Type: graphql.Int,
			},
		},
	})
	createArgs := adapters.CreateArgsFromModel(&models.User{})

	createField := &graphql.Field{
		Name:        "createUser",
		Description: "Create a new user",
		Type:        userType,
		Args: graphql.FieldConfigArgument{
			"data": &graphql.ArgumentConfig{
				Type: graphql.NewNonNull(graphql.NewInputObject(graphql.InputObjectConfig{
					Name:   "CreateUserInput",
					Fields: createArgs,
				})),
			},
		},
		Resolve: CreateUser(db),
	}
	updateField := &graphql.Field{
		Name:        "updateUser",
		Description: "Update a user",
		Type:        userType,
		Args: graphql.FieldConfigArgument{
			"id": &graphql.ArgumentConfig{
				Type: graphql.NewNonNull(graphql.Int),
			},
			"data": &graphql.ArgumentConfig{
				Type: graphql.NewNonNull(graphql.NewInputObject(graphql.InputObjectConfig{
					Name:   "UpdateUserInput",
					Fields: createArgs,
				})),
			},
		},
		Resolve: UpdateUser(db),
	}
	deleteField := &graphql.Field{
		Name:        "deleteUser",
		Description: "Delete a user",
		Type:        graphql.Boolean,
		Args: graphql.FieldConfigArgument{
			"id": &graphql.ArgumentConfig{
				Type: graphql.NewNonNull(graphql.Int),
			},
		},
		Resolve: resolvers.DefaultDeleteResolver(db, "users", "id"),
	}
	return []*graphql.Field{createField, updateField, deleteField}
}

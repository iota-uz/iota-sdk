package users

import (
	"github.com/graphql-go/graphql"
	"github.com/iota-agency/iota-erp/models"
	"github.com/iota-agency/iota-erp/sdk/graphql/old/adapters"
	resolvers2 "github.com/iota-agency/iota-erp/sdk/graphql/old/resolvers"
	"gorm.io/gorm"
)

func CreateUser(db *gorm.DB) graphql.FieldResolveFn {
	return func(p graphql.ResolveParams) (interface{}, error) {
		data := p.Args["data"].(map[string]interface{})
		user := &models.User{
			FirstName: data["firstName"].(string),
			LastName:  data["lastName"].(string),
			Email:     data["email"].(string),
			Password:  data["password"].(string),
		}
		if err := user.SetPassword(user.Password); err != nil {
			return nil, err
		}
		if err := db.Create(user).Error; err != nil {
			return nil, err
		}
		return user, nil
	}
}

func UpdateUser(db *gorm.DB) graphql.FieldResolveFn {
	return func(p graphql.ResolveParams) (interface{}, error) {
		data := p.Args["data"].(map[string]interface{})
		user := &models.User{
			Id:        int64(data["id"].(int)),
			FirstName: data["firstName"].(string),
			LastName:  data["lastName"].(string),
			Email:     data["email"].(string),
			Password:  data["password"].(string),
		}
		if err := user.SetPassword(user.Password); err != nil {
			return nil, err
		}
		if err := db.Save(user).Error; err != nil {
			return nil, err
		}
		return user, nil
	}
}

func Queries(db *gorm.DB) []*graphql.Field {
	userType, err := adapters.ReadTypeFromModel(&models.User{}, "User")
	if err != nil {
		panic(err)
	}
	return []*graphql.Field{
		resolvers2.AggregateQuery(db, &models.User{}, "users"),
		resolvers2.ListPaginatedQuery(db, &models.User{}, userType, "users"),
		resolvers2.GetQuery(db, &models.User{}, userType, "user"),
	}
}

func Mutations(db *gorm.DB) []*graphql.Field {
	userType, err := adapters.ReadTypeFromModel(&models.User{}, "UserResponse")
	if err != nil {
		panic(err)
	}
	createArgs, err := adapters.CreateArgsFromModel(&models.User{})
	if err != nil {
		panic(err)
	}
	updateArgs, err := adapters.UpdateArgsFromModel(&models.User{})
	if err != nil {
		panic(err)
	}
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
					Fields: updateArgs,
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
		Resolve: resolvers2.DefaultDeleteResolver(db, &models.User{}),
	}
	return []*graphql.Field{createField, updateField, deleteField}
}

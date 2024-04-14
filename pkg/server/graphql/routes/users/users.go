package users

import (
	"github.com/graphql-go/graphql"
	"github.com/iota-agency/iota-erp/models"
	"github.com/jmoiron/sqlx"
)

//type Query {
//    employees: [Employee]!
//    employee(id: ID!): Employee
//  }
//type Mutation {
//    createEmployee(input: EmployeeInput!): Employee!
//    updateEmployee(id: ID!, input: EmployeeInput!): Employee!
//    deleteEmployee(id: ID!): Employee!
//  }

func CreateUser(db *sqlx.DB) graphql.FieldResolveFn {
	return func(p graphql.ResolveParams) (interface{}, error) {
		user := &models.User{
			FirstName: p.Args["firstName"].(string),
			LastName:  p.Args["lastName"].(string),
			Email:     p.Args["email"].(string),
			Password:  p.Args["password"].(string),
		}
		if errs := user.Validate(); len(errs) != 0 {
			return nil, errs[0]
		}
		if err := user.SetPassword(user.Password); err != nil {
			return nil, err
		}
		if err := user.Save(db); err != nil {
			return nil, err
		}
		return user, nil
	}
}

func GraphQL(db *sqlx.DB) (*graphql.Object, *graphql.Object) {
	queryType := graphql.NewObject(graphql.ObjectConfig{
		Name:        "UsersList",
		Description: "A",
		Fields: graphql.Fields{
			"ip": &graphql.Field{
				Name: "ip",
				Type: graphql.String,
			},
		},
	})
	mutationType := graphql.NewObject(graphql.ObjectConfig{
		Name: "Users",
		Fields: graphql.Fields{
			"create": &graphql.Field{
				Args: graphql.FieldConfigArgument{
					"firstName": &graphql.ArgumentConfig{
						Type: graphql.NewNonNull(graphql.String),
					},
					"lastName": &graphql.ArgumentConfig{
						Type: graphql.NewNonNull(graphql.String),
					},
					"email": &graphql.ArgumentConfig{
						Type: graphql.NewNonNull(graphql.String),
					},
					"password": &graphql.ArgumentConfig{
						Type: graphql.NewNonNull(graphql.String),
					},
				},
				Type: graphql.NewObject(graphql.ObjectConfig{
					Name: "User",
					Fields: graphql.Fields{
						"token": &graphql.Field{
							Type: graphql.String,
						},
					},
				}),
				Resolve: CreateUser(db),
			},
		},
	})
	return queryType, mutationType
}

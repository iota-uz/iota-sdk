## Code

If the size of the array is known, use `make` to create the array with the correct size

```go
package main

// Bad
var employees []string
for i := 0; i < 10; i++ {
    employees = append(employees, "employee")
}

// Good
employees := make([]string, 10)

for i := 0; i < 10; i++ {
    employees[i] = "employee"
}
```

## Project structure
We follow the [Standard Go Project Layout](https://github.com/golang-standards/project-layout) and DDD principles. 
Below is an example of a project structure
```
project-root/
├── cmd/
│   └── server/
│       └── main.go
├── internal/
│   ├── app/
│   │   └── application.go
│   ├── domain/
│   │   ├── user/
│   │   │   ├── user.go
│   │   │   ├── user_repository.go
│   │   │   └── user_service.go
│   ├── infrastructure/
│   │   ├── persistence/
│   │   │   └── user_repository.go
│   │   └── event/
│   │       └── event_publisher.go
│   ├── interfaces/
│   │   └── graph/
│   │       ├── users.graphqls
│   │       ├── users.resolvers.go
│   │       └── gqlmodels/
│   │           └── models_gen.go
│   └── presentation/
│       ├── templates/
│       │   ├── pages/
│       │   │   ├── home/
│       │   │   │   └── index.templ
│       │   │   ├── user/
│       │   │   │   ├── profile.templ
│       │   │   │   └── list.templ
│       │   │   ├── admin/
│       │   │   │   └── dashboard.templ
│       │   ├── layouts/
│       │   │   ├── main.templ
│       │   │   └── admin.templ
│       │   ├── components/
│       │   │   ├── navbar.templ
│       │   │   ├── footer.templ
│       │   │   └── sidebar.templ
│       ├── controllers/
│       │   └── user_controller.go
│       └── static/
│           ├── css/
│           │   └── styles.css
│           └── js/
│               └── scripts.js
└── go.mod
```


## Types

Use `int64` for `count` && `ID` fields

```go
package models

type Employee struct {
    ID        int64
    Email     string
    FirstName string
    LastName  string
    PositionID int64
    Salary    float64
}
```

## Naming

### GraphQL
For queries and mutations, we use camelCase. Below is an example for the `users` service

```graphql
type User {
    id: ID!
    firstName: String!
    lastName: String!
    email: String!
    createdAt: Time!
    updatedAt: Time!
}

type PaginatedUsers {
    data: [User!]!
    total: Int64!
}

input CreateUser {
    firstName: String!
    lastName: String!
    email: String!
}

input UpdateUser {
    firstName: String
    lastName: String
    email: String
}

extend type Query {
    user(id: ID!): User
    users(offset: Int!, limit: Int!, sortBy: [String!]): PaginatedUsers!
}

extend type Mutation {
    createUser(input: CreateUser!): User!
    updateUser(id: ID!, input: UpdateUser!): User!
    deleteUser(id: ID!): User!
}

extend type Subscription {
    userCreated: User!
    userUpdated: User!
    userDeleted: User!
}
```

### Code style
We use the [standard Go code style](https://go.dev/doc/effective_go) with the following exceptions:
- We use `PascalCase` for struct fields
- We use `PascalCase` for file names

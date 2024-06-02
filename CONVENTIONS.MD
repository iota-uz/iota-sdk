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
│   │   ├── application.go
│   │   ├── service/
│   │   │   └── user_service.go
│   ├── domain/
│   │   ├── user/
│   │   │   ├── user.go
│   │   │   ├── user_repository.go
│   │   │   └── user_service.go
│   ├── infrastructure/
│   │   ├── persistence/
│   │   │   └── user_repository.go
│   │   ├── event/
│   │   │   └── event_publisher.go
│   └── interfaces/
│       └── api/
│           └── user_handler.go
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
type Query {
    employees: EmployeesPaginated!
    employee(id: ID!): Employee!
    usersAggregate(filter: EmployeeFilter): EmployeesAggregate!
}

type Mutation {
    createEmployee(input: CreateEmployeeInput!): EmployeeResponse!
    updateEmployee(id: ID!, input: UpdateEmployeeInput!): EmployeeResponse!
    deleteEmployee(id: ID!): EmployeeResponse!
}

type Employee {
    id: Int!
    email: String!
    first_name: String!
    last_name: String!
    position_id: Int
    salary: Float!
}

type EmployeesPaginated {
    total: Int!
    data: [Employee]!
}

input CreateEmployeeInput {
    email: String!
    first_name: String!
    last_name: String!
    position_id: Int
}

type UpdateEmployeeInput {
    email: String
    first_name: String
    last_name: String
    position_id: Int
}

type EmployeesAggregate {
    first_name {
        count: Int!
    }
    last_name {
        count: Int!
    }
    position_id {
        count: Int!
    }
    salary {
        sum: Float!
        avg: Float!
        max: Float!
        min: Float!
        count: Int!
    }
}
```


#### CRUD operations
```graphql
type Query {
    employees: EmployeesPaginated!
    employee(id: ID!): Employee!
    usersAggregate(filter: EmployeeFilter): EmployeesAggregate!
}

type Mutation {
    createEmployee(input: CreateEmployeeInput!): EmployeeResponse!
    updateEmployee(id: ID!, input: UpdateEmployeeInput!): EmployeeResponse!
    deleteEmployee(id: ID!): EmployeeResponse!
}
```

### Code style
We use the [standard Go code style](https://go.dev/doc/effective_go) with the following exceptions:
- We use `PascalCase` for struct fields
- We use `PascalCase` for file names
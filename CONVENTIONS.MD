## Types
Use `int64` for count && id fields

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
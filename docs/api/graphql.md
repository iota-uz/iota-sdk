---
layout: default
title: GraphQL API
parent: API Reference
nav_order: 1
description: "GraphQL API endpoint documentation"
---

# GraphQL API

The IOTA SDK GraphQL API provides a type-safe, flexible interface for querying and mutating data.

## Endpoint

```
POST /api/graphql
GET /api/graphql (for introspection queries)
WebSocket /api/graphql (for subscriptions)
```

## Authentication

### Session-Based (Default)

Automatic with HTTP cookies set after login:

```bash
# 1. Login to obtain session cookie
curl -X POST http://localhost:8080/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"user@example.com","password":"secret"}'

# 2. Use session cookie for GraphQL queries
curl -X POST http://localhost:8080/api/graphql \
  -H "Content-Type: application/json" \
  -b "session=abc123" \
  -d '{"query":"{users{data{id firstName}}}"}'
```

### Bearer Token

For API clients and integrations:

```bash
curl -X POST http://localhost:8080/api/graphql \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer your-token-here" \
  -d '{"query":"{users{data{id firstName}}}"}'
```

## Request Format

### Standard Query

```json
{
  "query": "query { users(limit: 10) { data { id firstName email } } }",
  "variables": {},
  "operationName": null
}
```

### With Variables

```json
{
  "query": "query GetUser($id: ID!) { user(id: $id) { id firstName email } }",
  "variables": {
    "id": "550e8400-e29b-41d4-a716-446655440000"
  }
}
```

## Response Format

### Success Response

```json
{
  "data": {
    "users": {
      "data": [
        {
          "id": "550e8400-e29b-41d4-a716-446655440000",
          "firstName": "John",
          "lastName": "Doe",
          "email": "john@example.com"
        }
      ],
      "total": 1
    }
  }
}
```

### Error Response

```json
{
  "errors": [
    {
      "message": "User not found",
      "locations": [
        {
          "line": 2,
          "column": 3
        }
      ],
      "extensions": {
        "code": "NOT_FOUND"
      }
    }
  ]
}
```

## Core Types

### User

```graphql
type User {
  id: ID!
  firstName: String!
  lastName: String!
  email: String!
  uiLanguage: String!
  updatedAt: Time!
  createdAt: Time!
}
```

### Session

```graphql
type Session {
  token: String!
  userId: ID!
  ip: String!
  userAgent: String!
  expiresAt: Time!
  createdAt: Time!
}
```

### PaginatedResponse

```graphql
type PaginatedUsers {
  data: [User!]!
  total: Int64!
}
```

## Query Examples

### Get Current User

```graphql
query {
  me {
    id
    firstName
    lastName
    email
    uiLanguage
  }
}
```

**Response:**
```json
{
  "data": {
    "me": {
      "id": "550e8400-e29b-41d4-a716-446655440000",
      "firstName": "John",
      "lastName": "Doe",
      "email": "john@example.com",
      "uiLanguage": "en"
    }
  }
}
```

### List Users

```graphql
query {
  users(offset: 0, limit: 10, ascending: true) {
    data {
      id
      firstName
      lastName
      email
      createdAt
    }
    total
  }
}
```

**Response:**
```json
{
  "data": {
    "users": {
      "data": [
        {
          "id": "550e8400-e29b-41d4-a716-446655440000",
          "firstName": "John",
          "lastName": "Doe",
          "email": "john@example.com",
          "createdAt": "2024-01-15T10:30:00Z"
        }
      ],
      "total": 1
    }
  }
}
```

### Get User by ID

```graphql
query GetUser($id: ID!) {
  user(id: $id) {
    id
    firstName
    lastName
    email
    createdAt
    updatedAt
  }
}
```

**Variables:**
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000"
}
```

## Mutation Examples

### Authenticate

```graphql
mutation {
  authenticate(email: "user@example.com", password: "secret") {
    token
    userId
    expiresAt
  }
}
```

**Response:**
```json
{
  "data": {
    "authenticate": {
      "token": "abc123token",
      "userId": "550e8400-e29b-41d4-a716-446655440000",
      "expiresAt": "2024-01-16T10:30:00Z"
    }
  }
}
```

### Google Authentication

```graphql
mutation {
  googleAuthenticate
}
```

This initiates OAuth flow. Redirect to the returned URL, then use session cookies.

### Delete Session

```graphql
mutation {
  deleteSession(token: "abc123token")
}
```

## Filtering

### Pagination Parameters

```graphql
users(
  offset: 0,        # Starting position
  limit: 10,        # Items per page
  sortBy: ["firstName"],  # Sort fields
  ascending: true   # Sort direction
) {
  data { id firstName }
  total
}
```

### Filter Operators

```graphql
# Not yet fully implemented, but planned for:
users(filters: [
  {field: "status", operator: "eq", value: "active"},
  {field: "createdAt", operator: "gte", value: "2024-01-01"}
]) {
  data { id firstName }
  total
}
```

## Subscriptions

Subscribe to real-time events:

```graphql
subscription {
  sessionDeleted
}
```

**WebSocket URL:**
```
wss://localhost:8080/api/graphql
```

**Connection:**
```json
{
  "type": "connection_init",
  "payload": {
    "Authorization": "Bearer token"
  }
}
```

## Common Patterns

### Paginate Through All Results

```graphql
query GetAllUsers {
  page1: users(offset: 0, limit: 100) {
    data { id firstName }
    total
  }
}
```

Then fetch subsequent pages with offset += limit.

### Search Users

```graphql
# Once full-text search is implemented
query {
  users(filters: [
    {field: "email", operator: "like", value: "example"}
  ]) {
    data { id firstName email }
    total
  }
}
```

### Sort Results

```graphql
query {
  users(
    limit: 20,
    sortBy: ["firstName", "lastName"],
    ascending: true
  ) {
    data { id firstName lastName }
    total
  }
}
```

## Error Handling

### Authentication Error

```json
{
  "errors": [
    {
      "message": "Unauthorized",
      "extensions": {
        "code": "UNAUTHENTICATED"
      }
    }
  ]
}
```

### Validation Error

```json
{
  "errors": [
    {
      "message": "validation error: email is required",
      "extensions": {
        "code": "VALIDATION_ERROR"
      }
    }
  ]
}
```

### Permission Error

```json
{
  "errors": [
    {
      "message": "Forbidden",
      "extensions": {
        "code": "FORBIDDEN"
      }
    }
  ]
}
```

## Rate Limiting

GraphQL requests are subject to rate limiting:

```
X-RateLimit-Limit: 1000
X-RateLimit-Remaining: 998
X-RateLimit-Reset: 1640000000
```

## HTTP Status Codes

| Status | Meaning |
|--------|---------|
| 200 | Request processed (check errors in response) |
| 400 | Invalid request format |
| 401 | Authentication required |
| 403 | Forbidden |
| 429 | Rate limited |
| 500 | Server error |
| 503 | Service unavailable |

## Introspection

Query the schema:

```graphql
{
  __schema {
    types {
      name
      kind
      fields {
        name
        type { name kind }
      }
    }
  }
}
```

## Caching

### Cache Headers

Responses include cache headers:

```
Cache-Control: max-age=300, public
ETag: "abc123"
```

### Client-Side Caching

Implement in client libraries using response `__typename`:

```javascript
// Apollo Client example
const cache = new InMemoryCache({
  typePolicies: {
    User: {
      keyFields: ["id"]
    }
  }
});
```

## Batch Queries

Send multiple queries in one request:

```json
[
  {
    "query": "query { users(limit: 10) { data { id } } }"
  },
  {
    "query": "query { me { id firstName } }"
  }
]
```

**Response:**
```json
[
  {
    "data": {
      "users": { "data": [...], "total": ... }
    }
  },
  {
    "data": {
      "me": { "id": "...", "firstName": "..." }
    }
  }
]
```

## Testing with cURL

### Simple Query

```bash
curl -X POST http://localhost:8080/api/graphql \
  -H "Content-Type: application/json" \
  -d '{
    "query": "{ users(limit: 10) { data { id firstName } total } }"
  }'
```

### Query with Variables

```bash
curl -X POST http://localhost:8080/api/graphql \
  -H "Content-Type: application/json" \
  -d '{
    "query": "query GetUser($id: ID!) { user(id: $id) { id firstName } }",
    "variables": {
      "id": "550e8400-e29b-41d4-a716-446655440000"
    }
  }'
```

### With Authentication

```bash
curl -X POST http://localhost:8080/api/graphql \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -d '{"query":"{ me { id firstName } }"}'
```

## GraphQL Playground

Access interactive GraphQL IDE in development:

```
http://localhost:8080/graphql
```

Features:
- Query editor with syntax highlighting
- Schema documentation
- Query history
- Variables panel
- Response viewer

## Best Practices

1. **Always paginate**: Use limit/offset for large datasets
   ```graphql
   users(offset: 0, limit: 100) { data { id } total }
   ```

2. **Request only needed fields**: Minimize data transfer
   ```graphql
   # Good
   users { data { id firstName } }

   # Bad
   users { data { ... all fields ... } }
   ```

3. **Use fragments for reusable selections**:
   ```graphql
   fragment UserFields on User {
     id
     firstName
     lastName
     email
   }

   query {
     users { data { ...UserFields } }
   }
   ```

4. **Handle errors gracefully**:
   ```javascript
   if (response.errors) {
     console.error("GraphQL error:", response.errors[0].message);
   }
   ```

5. **Implement exponential backoff** for retries:
   ```javascript
   let delay = 100;
   while (retries < maxRetries) {
     try {
       return await query();
     } catch (e) {
       await sleep(delay);
       delay *= 2;
     }
   }
   ```

## Limitations

- Query timeout: 30 seconds
- Maximum query depth: 10 levels
- Maximum query complexity: 1000 points
- File upload max size: 100MB

## Modules Schema

Each module registers its own GraphQL types and queries/mutations. See module documentation for specific schemas.

### Available Modules

- **Core**: Users, Roles, Groups, Sessions
- **Finance**: Payments, Expenses, Transactions
- **CRM**: Clients, Chats, Messages
- **Warehouse**: Products, Inventory, Orders
- **Projects**: Projects, Stages
- **HRM**: Employees

---

For more information, see the [API Reference](./index.md) or [IOTA SDK Documentation](../index.md).

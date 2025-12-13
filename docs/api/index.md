---
layout: default
title: API Reference
nav_order: 15
has_children: true
description: "API reference for IOTA SDK endpoints"
---

# API Reference

The IOTA SDK provides comprehensive REST and GraphQL APIs for building client applications and integrations.

## Overview

The IOTA SDK API includes:

- **GraphQL API**: Modern, type-safe queries and mutations
- **REST Endpoints**: Traditional HTTP endpoints for common operations
- **WebSocket Subscriptions**: Real-time data updates
- **File Upload**: Multipart file upload support
- **Pagination**: Efficient data retrieval with limit/offset
- **Filtering**: Advanced query filtering and searching
- **Authentication**: Cookie-based and token-based auth

## API Versions

| Version | Status | Support |
|---------|--------|---------|
| v1 | Current | Active |

## Base URL

```
http://localhost:8080/api/graphql
```

## Authentication

### Cookie-Based (Default)

Automatic with session cookies. Obtained via login endpoint:

```
POST /auth/login
```

### Bearer Token (Optional)

For API clients:

```
Authorization: Bearer <token>
```

### User Context

All requests are scoped to:
- **Tenant**: Current user's tenant
- **Organization**: Current user's organization
- **User**: Authenticated user

## Response Format

### Success Response

```json
{
    "data": {
        "users": [
            {
                "id": "uuid",
                "firstName": "John",
                "lastName": "Doe",
                "email": "john@example.com"
            }
        ]
    }
}
```

### Error Response

```json
{
    "errors": [
        {
            "message": "User not found",
            "extensions": {
                "code": "NOT_FOUND",
                "timestamp": "2024-01-15T10:30:00Z"
            }
        }
    ]
}
```

## Rate Limiting

All API endpoints are subject to rate limiting:

```
X-RateLimit-Limit: 1000
X-RateLimit-Remaining: 998
X-RateLimit-Reset: 1640000000
```

## HTTP Methods

### Supported Methods

- **GET** - Retrieve data (queries)
- **POST** - Create data, execute mutations
- **PUT** - Update data
- **DELETE** - Delete data
- **PATCH** - Partial updates

## Data Types

### Common Types

| Type | Format | Example |
|------|--------|---------|
| ID | UUID v4 | `550e8400-e29b-41d4-a716-446655440000` |
| String | UTF-8 | `"John Doe"` |
| Integer | 64-bit | `12345` |
| Decimal | String | `"150.50"` |
| Boolean | true/false | `true` |
| DateTime | ISO 8601 | `2024-01-15T10:30:00Z` |
| Date | ISO 8601 | `2024-01-15` |
| Time | HH:MM:SS | `14:30:00` |

## Pagination

All list endpoints support pagination:

```graphql
query {
    users(offset: 0, limit: 10, sortBy: ["firstName"], ascending: true) {
        data {
            id
            firstName
            lastName
        }
        total
    }
}
```

Parameters:
- `offset`: Starting position (default: 0)
- `limit`: Number of items per page (default: 20, max: 100)
- `sortBy`: Field names to sort by (array)
- `ascending`: Sort direction (default: true)

## Filtering

Filter entities using field conditions:

```graphql
query {
    users(filters: [
        {field: "status", operator: "eq", value: "active"},
        {field: "createdAt", operator: "gte", value: "2024-01-01"}
    ]) {
        data {
            id
            firstName
        }
        total
    }
}
```

Supported operators:
- `eq` - Equal
- `neq` - Not equal
- `gt` - Greater than
- `gte` - Greater than or equal
- `lt` - Less than
- `lte` - Less than or equal
- `like` - Contains (text search)
- `in` - In array
- `nin` - Not in array

## Error Codes

| Code | Status | Description |
|------|--------|-------------|
| INVALID_REQUEST | 400 | Invalid request parameters |
| UNAUTHORIZED | 401 | Authentication required |
| FORBIDDEN | 403 | Insufficient permissions |
| NOT_FOUND | 404 | Resource not found |
| CONFLICT | 409 | Resource already exists |
| VALIDATION_ERROR | 422 | Input validation failed |
| RATE_LIMITED | 429 | Rate limit exceeded |
| SERVER_ERROR | 500 | Internal server error |

## CORS

CORS is enabled for authenticated requests:

```
Access-Control-Allow-Origin: *
Access-Control-Allow-Methods: GET, POST, PUT, DELETE
Access-Control-Allow-Headers: Content-Type, Authorization
```

## Modules and Endpoints

The IOTA SDK is organized into modules. Each module provides specific endpoints:

### Core Module
- **Authentication**: `/auth/login`, `/auth/logout`
- **Users**: `/api/users`, `/api/users/{id}`
- **Roles**: `/api/roles`, `/api/roles/{id}`
- **Groups**: `/api/groups`, `/api/groups/{id}`

### Finance Module
- **Payments**: `/api/finance/payments`
- **Expenses**: `/api/finance/expenses`
- **Transactions**: `/api/finance/transactions`
- **Accounts**: `/api/finance/accounts`

### CRM Module
- **Clients**: `/api/crm/clients`
- **Chats**: `/api/crm/chats`
- **Message Templates**: `/api/crm/message-templates`

### Warehouse Module
- **Products**: `/api/warehouse/products`
- **Inventory**: `/api/warehouse/inventory`
- **Orders**: `/api/warehouse/orders`

### Projects Module
- **Projects**: `/api/projects`
- **Stages**: `/api/projects/stages`

### HRM Module
- **Employees**: `/api/hrm/employees`

## SDK Features

### Subscriptions (Real-time)

WebSocket-based subscriptions for real-time updates:

```graphql
subscription {
    userCreated {
        id
        firstName
        email
    }
}
```

Available subscriptions:
- `userCreated` - New user created
- `userUpdated` - User profile updated
- `paymentProcessed` - Payment completed
- `orderCreated` - New order
- `orderStatusChanged` - Order status update

### File Upload

Upload files through GraphQL multipart:

```graphql
mutation {
    uploadFile(file: File!) {
        id
        url
        fileName
        contentType
        size
    }
}
```

Maximum file size: 100MB (configurable)

## Quick Links

- **[GraphQL API](./graphql.md)** - GraphQL endpoint documentation
- **[Authentication](./graphql.md#authentication)** - Auth patterns
- **[Error Handling](./graphql.md#error-handling)** - Error responses
- **[Examples](./graphql.md#examples)** - Common queries and mutations

## Testing the API

### Using cURL

```bash
# GraphQL query
curl -X POST http://localhost:8080/api/graphql \
  -H "Content-Type: application/json" \
  -d '{"query":"{users(limit:10){data{id firstName}}}"}'

# With authentication
curl -X POST http://localhost:8080/api/graphql \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer token123" \
  -d '{"query":"{users{data{id}}}"}'
```

### Using GraphQL Playground

Visit: `http://localhost:8080/graphql` (in development mode)

### Using REST

```bash
# GET request
curl http://localhost:8080/api/users

# POST request
curl -X POST http://localhost:8080/api/users \
  -H "Content-Type: application/json" \
  -d '{"firstName":"John","lastName":"Doe"}'
```

## SDK Clients

Official client libraries:

- **JavaScript/TypeScript**: `@iota-sdk/client`
- **Python**: `iota-sdk` (PyPI)
- **Go**: `github.com/iota-uz/iota-sdk`

## Support

- **Documentation**: [IOTA SDK Docs](https://iota-sdk.uz/docs)
- **GitHub Issues**: [Report Issues](https://github.com/iota-uz/iota-sdk/issues)
- **Discussions**: [GitHub Discussions](https://github.com/iota-uz/iota-sdk/discussions)

---

For detailed API documentation, see the [GraphQL API Guide](./graphql.md).

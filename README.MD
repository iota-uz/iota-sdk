## About
IOTA ERP - is a set of building blocks for ERP development, which will later turn into a complete SDK

## Overview
* [Architecture](https://app.excalidraw.com/s/3x4l1qRpK2w/ADYN81ksZsd)
* [Database schema](https://dbdiagram.io/d/ERP-SDK-65fd8cb4ae072629ceb7f50e)

## Features
* [x] GraphQL API
* [x] Database migrations
* [x] Hot reload
* [x] Query filters (partial)
* [x] Query sorting
* [x] Query pagination
* [x] Query relations (partial)
* [x] Query fields selection
* [ ] Caching
* [x] Authorization
* [x] Authentication
* [ ] Subscriptions
* [ ] DataLoader
* [ ] File uploads

## Prerequisites
* [SQL-Migrate](https://github.com/rubenv/sql-migrate)
* [Go](https://golang.org/doc/install)
* [Docker](https://docs.docker.com/get-docker/)
* [Air](https://github.com/cosmtrek/air#Installation)

## Project structure
We follow the [Standard Go Project Layout](https://github.com/golang-standards/project-layout)

## Development

Clone the repository
```shell
git clone https://github.com/iota-agency/iota-erp.git
```

Install dependencies
```shell
go get -u -d ./...
```

Run postgres
```shell
docker compose -f docker-compose.dev.yml up -d
```

Run migrations
```shell
go run cmd/migrate/migrate.go up
```

Run in watch mode (hot reload)
```shell
air
```

Now open postman and explore the GraphQL schema at http://localhost:3200/query

## Examples
Get all employees and their positions
```graphql
query Users {
    users(limit: 10, offset: 0) {
        total
        data {
            id
            email
            firstName
            lastName
            avatar {
                id
                size
            }
        }
    }
}
```

## Reference commands

Regenerate from GraphQL schema
```shell
go generate ./...
```

Making a migration
```shell
go run cmd/makemigration/main.go
```

Running tests
```shell
go test -v ./...
```

Running the server
```shell
go run cmd/server/main.go
```

## Conventions
* [Naming](CONVENTIONS.MD#naming)
* [Code style](CONVENTIONS.MD#code-style)
## About
IOTA ERP - is a set of building blocks for ERP development, which will later turn into a complete SDK

## Overview

[Architecture](https://app.excalidraw.com/s/3x4l1qRpK2w/ADYN81ksZsd)


## Development

Install dependencies

```shell
go get -u -d ./...
```

Running in dev mode
```shell
air
```

Run migrations
```shell
go run cmd/migrate/migrate.go up
```

Make migrations
```shell
go run cmd/makemigration/main.go
```

Run server
```shell
go run cmd/server/houston.go
```

Running tests
```shell
go test -v ./...
```

## TODO

- [ ] Add tests
- [ ] Add more documentation
- [ ] Add JOIN support for queries
- [ ] Add more query filters (like `IN`, `NOT IN`, `LIKE`, `NOT LIKE`, `BETWEEN`, `NOT BETWEEN`, `IS NULL`, `IS NOT NULL`)

## About


## Development

Install dependencies

```shell
go get -u -d ./...
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


## IMPORTANT

Do not run

```shell
golangci-lint run --fix
```

It will break the code.

When facing an error like this:

```
WARN [runner] Can't run linter goanalysis_metalinter: buildssa: failed to load package : could not load export data: no
export data for "github.com/iota-uz/iota-sdk/pkg/domain/entities/expense_category"
```

Try running:

```shell
go mod tidy
```

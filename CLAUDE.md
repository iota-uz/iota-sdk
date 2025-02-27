# CLAUDE.md - IOTA SDK Guide

## Overview
The project follows DDD principles 

## Build/Lint/Test Commands
- After changes to css or .templ files: `make generate && make css`
- Run all tests: `make test` or `go test -v ./...` 
- Run single test: `go test -v ./path/to/package -run TestName`
- Run specific subtest: `go test -v ./path/to/package -run TestName/SubtestName`
- Lint code: `make lint` or `golangci-lint run ./...`
- JSON linting: `make build-iota-linter && make run-iota-linter`
- Apply migrations: `make migrate up`

## Code Style Guidelines
- Use Go v1.23.2 and follow standard Go idioms
- File organization: group related functionality in modules/ or pkg/ directories
- Naming: use camelCase for variables, PascalCase for exported functions/types
- Testing: table-driven tests with descriptive names (TestFunctionName_Scenario)
- Error handling: use pkg/serrors for standard error types
- Type safety: use strong typing and avoid interface{} where possible
- Follow existing patterns for database operations with jmoiron/sqlx
- For UI components, follow the existing templ/htmx patterns


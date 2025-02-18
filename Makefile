# Variables
TAILWIND_INPUT := modules/core/presentation/assets/css/main.css
TAILWIND_OUTPUT := modules/core/presentation/assets/css/main.min.css

# Install dependencies
deps:
	go get ./...

# Seed database
seed:
	go run cmd/seed/main.go

generate:
	go generate ./... && templ generate

# Run tests
test:
	go test -v ./... -coverprofile=./coverage/coverage.out

# Run tests with file watching
test-watch:
	gow test -v ./...

# Generate dependency graph
graph:
	goda graph ./modules/... | dot -Tpng -o dependencies.png

# Run tests inside docker
test-docker:
	docker compose -f compose.testing.yml up --build erp_local

coverage-score:
	go tool cover -func ./coverage/coverage.out | grep "total:" | awk '{print ((int($$3) > 80) != 1) }'

report:
	go tool cover -html=coverage.out -o ./coverage/cover.html

# Run PostgreSQL
localdb:
	docker compose -f compose.dev.yml up

clear-localdb:
	rm -rf postgres-data/

# Apply database migrations (up)
migrate-up:
	go run cmd/migrate/main.go up

# Downgrade database migrations (down)
migrate-down:
	go run cmd/migrate/main.go down

# Compile TailwindCSS (with watch)
css-watch:
	tailwindcss -c tailwind.config.js -i $(TAILWIND_INPUT) -o $(TAILWIND_OUTPUT) --minify --watch

# Compile TailwindCSS (without watch)
css:
	tailwindcss -c tailwind.config.js -i $(TAILWIND_INPUT) -o $(TAILWIND_OUTPUT) --minify

# Run linter
lint:
	golangci-lint run ./...

# Release - assume Alpine Linux as target
release:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o /build/run_server cmd/server/main.go

# Release - assume local OS as target
release-local:
	go build -ldflags="-s -w" -o run_server cmd/server/main.go

# Clean build artifacts
clean:
	rm -rf $(TAILWIND_OUTPUT)

# Full setup
setup: deps localdb migrate-up css lint

## Linter targets
#.PHONY: build-iota-linter run-iota-linter clean-iota-linter

# Build the JSON linter
build-iota-linter:
	go build -o bin/iotalinter tools/iotalinter.go

# Run the JSON linter
run-iota-linter:
	./bin/iotalinter ./...

# Clean built binaries
clean-iota-linter:
	rm -f bin/iotalinter

# Migration management targets
collect-migrations:
	go run cmd/migrate/main.go collect

build-docker-base:
	docker buildx build --push --platform linux/amd64,linux/arm64 -t iotauz/sdk:base-$v --target base .

build-docker-prod:
	docker buildx build --push --platform linux/amd64,linux/arm64 -t iotauz/sdk:$v --target production .

.PHONY: default deps test test-watch localdb migrate-up migrate-down dev css-watch css lint release release-local clean setup build-iota-linter run-iota-linter clean-iota-linter collect-migrations
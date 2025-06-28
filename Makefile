# Variables
TAILWIND_INPUT := modules/core/presentation/assets/css/main.css
TAILWIND_OUTPUT := modules/core/presentation/assets/css/main.min.css

# Install dependencies
deps:
	go get ./...

fmt:
	go fmt ./... && templ fmt . && go mod tidy

# Seed database
seed:
	go run cmd/seed/main.go

# Generate code documentation
docs:
	go run cmd/document/main.go -dir . -out docs/LLMS.md -recursive -exclude "vendor,node_modules,tmp,e2e,py-embed,cmd"

generate:
	go generate ./... && templ generate

# Run tests
test:
	go test -v ./... -p 8 -coverprofile=./coverage/coverage.out

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

reset-localdb:
	docker compose -f compose.dev.yml down
	make clear-localdb
	make localdb

migrate:
	go run cmd/migrate/main.go $(filter-out $@,$(MAKECMDGOALS))

# Compile TailwindCSS (with watch)
css-watch:
	tailwindcss -c tailwind.config.js -i $(TAILWIND_INPUT) -o $(TAILWIND_OUTPUT) --minify --watch

# Compile TailwindCSS (without watch)
css:
	tailwindcss -c tailwind.config.js -i $(TAILWIND_INPUT) -o $(TAILWIND_OUTPUT) --minify

# Run linter
lint:
	make fmt
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

check-tr:
	go run cmd/command/main.go

build-docker-base:
	docker buildx build --push --platform linux/amd64,linux/arm64 -t iotauz/sdk:base-$v --target base .

build-docker-prod:
	docker buildx build --push --platform linux/amd64,linux/arm64 -t iotauz/sdk:$v --target production .

tunnel:
	cloudflared tunnel --url http://localhost:3200 --loglevel debug

# Prevents make from treating the argument as an undefined target
%:
	@:

.PHONY: default deps test test-watch localdb clear-localdb reset-localdb migrate-up migrate-down dev css-watch css lint release release-local clean setup run-iota-linter clean-iota-linter collect-migrations docs seed

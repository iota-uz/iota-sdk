# Variables
TAILWIND_INPUT := pkg/presentation/assets/css/main.css
TAILWIND_OUTPUT := pkg/presentation/assets/css/main.min.css

# Default target
default: dev

# Install dependencies
deps:
	go get -u ./...

# Seed database
seed:
	go run cmd/seed/main.go

# Run tests
test:
	go test -v ./...

# Run tests with file watching
test-watch:
	gow test -v ./...

# Run tests inside docker
test-docker:
	docker compose -f docker-compose.testing.yml up --build erp_local

# Run PostgreSQL
localdb:
	docker compose -f docker-compose.dev.yml up

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

# Generate templ files
templ:
	templ generate

# Run linter
lint:
	golangci-lint run ./...

# Clean build artifacts
clean:
	rm -rf $(TAILWIND_OUTPUT)

# Full setup
setup: deps localdb migrate-up css lint

# Release pipeline
release:
	git checkout build
	templ generate
	git merge main
	git add -f '*_templ.go'
	git commit -m"add build files"
	git push
	git checkout main

.PHONY: default deps test test-watch localdb migrate-up migrate-down dev css-watch css templ lint clean setup release

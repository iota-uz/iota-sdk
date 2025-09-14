# Variables
TAILWIND_INPUT := modules/core/presentation/assets/css/main.css
TAILWIND_OUTPUT := modules/core/presentation/assets/css/main.min.css

# Install dependencies
deps:
	go get ./...



# Generate code documentation
docs:
	go run cmd/command/main.go doc --dir . --out docs/LLMS.md --recursive --exclude "vendor,node_modules,tmp,e2e,cmd"

# Template generation with optional subcommands (generate, watch)
generate:
	@if [ "$(word 2,$(MAKECMDGOALS))" = "watch" ]; then \
		templ generate --watch; \
	else \
		go generate ./... && templ generate; \
	fi


# Docker compose management with subcommands (up, down, restart, logs)
compose:
	@if [ "$(word 2,$(MAKECMDGOALS))" = "up" ]; then \
		docker compose -f compose.dev.yml up; \
	elif [ "$(word 2,$(MAKECMDGOALS))" = "down" ]; then \
		docker compose -f compose.dev.yml down; \
	elif [ "$(word 2,$(MAKECMDGOALS))" = "restart" ]; then \
		docker compose -f compose.dev.yml down && docker compose -f compose.dev.yml up; \
	elif [ "$(word 2,$(MAKECMDGOALS))" = "logs" ]; then \
		docker compose -f compose.dev.yml logs -f; \
	else \
		echo "Usage: make compose [up|down|restart|logs]"; \
		echo "  up      - Start all development services"; \
		echo "  down    - Stop all development services"; \
		echo "  restart - Stop and start all services"; \
		echo "  logs    - Follow logs from all services"; \
	fi

# Database management with subcommands (local, stop, clean, reset, seed, migrate)
db:
	@if [ "$(word 2,$(MAKECMDGOALS))" = "local" ]; then \
		docker compose -f compose.dev.yml up db; \
	elif [ "$(word 2,$(MAKECMDGOALS))" = "stop" ]; then \
		docker compose -f compose.dev.yml down db; \
	elif [ "$(word 2,$(MAKECMDGOALS))" = "clean" ]; then \
		docker volume rm sdk-data || true; \
	elif [ "$(word 2,$(MAKECMDGOALS))" = "reset" ]; then \
		docker compose -f compose.dev.yml down db && docker volume rm sdk-data || true && docker compose -f compose.dev.yml up db; \
	elif [ "$(word 2,$(MAKECMDGOALS))" = "seed" ]; then \
		go run cmd/command/main.go seed; \
	elif [ "$(word 2,$(MAKECMDGOALS))" = "migrate" ]; then \
		go run cmd/command/main.go migrate $(word 3,$(MAKECMDGOALS)); \
	else \
		echo "Usage: make db [local|stop|clean|reset|seed|migrate]"; \
		echo "  local   - Start local PostgreSQL database"; \
		echo "  stop    - Stop database container"; \
		echo "  clean   - Remove postgres-data directory"; \
		echo "  reset   - Stop, clean, and restart local database"; \
		echo "  seed    - Seed database with test data"; \
		echo "  migrate - Run database migrations (up/down/redo/collect)"; \
	fi

# Run tests with optional subcommands (test, watch, coverage, verbose, package, docker)
test:
	@if [ "$(word 1,$(MAKECMDGOALS))" != "test" ]; then \
		exit 0; \
	fi
	@if [ "$(word 2,$(MAKECMDGOALS))" = "watch" ]; then \
		gow test -v ./...; \
	elif [ "$(word 2,$(MAKECMDGOALS))" = "coverage" ]; then \
		go test -v ./... -coverprofile=./coverage/coverage.out; \
	elif [ "$(word 2,$(MAKECMDGOALS))" = "verbose" ]; then \
		go test -v ./...; \
	elif [ "$(word 2,$(MAKECMDGOALS))" = "package" ]; then \
		go test -v $(word 3,$(MAKECMDGOALS)); \
	elif [ "$(word 2,$(MAKECMDGOALS))" = "docker" ]; then \
		docker compose -f compose.testing.yml up --build erp_local; \
	elif [ "$(word 2,$(MAKECMDGOALS))" = "score" ]; then \
		go tool cover -func ./coverage/coverage.out | grep "total:" | awk '{print ((int($$3) > 80) != 1) }'; \
	elif [ "$(word 2,$(MAKECMDGOALS))" = "report" ]; then \
		go tool cover -html=coverage.out -o ./coverage/cover.html; \
	else \
		go test ./...; \
	fi

# Compile TailwindCSS with optional subcommands (css, watch, dev, clean)
css:
	@if [ "$(word 2,$(MAKECMDGOALS))" = "watch" ]; then \
		tailwindcss -c tailwind.config.js -i $(TAILWIND_INPUT) -o $(TAILWIND_OUTPUT) --minify --watch; \
	elif [ "$(word 2,$(MAKECMDGOALS))" = "dev" ]; then \
		tailwindcss -c tailwind.config.js -i $(TAILWIND_INPUT) -o $(TAILWIND_OUTPUT); \
	elif [ "$(word 2,$(MAKECMDGOALS))" = "clean" ]; then \
		rm -rf $(TAILWIND_OUTPUT); \
	else \
		tailwindcss -c tailwind.config.js -i $(TAILWIND_INPUT) -o $(TAILWIND_OUTPUT) --minify; \
	fi

# E2E testing management with subcommands (setup, test, reset, seed, run, clean)
e2e:
	@if [ "$(word 2,$(MAKECMDGOALS))" = "test" ]; then \
		go run cmd/command/main.go e2e test; \
	elif [ "$(word 2,$(MAKECMDGOALS))" = "reset" ]; then \
		go run cmd/command/main.go e2e reset; \
	elif [ "$(word 2,$(MAKECMDGOALS))" = "seed" ]; then \
		go run cmd/command/main.go e2e seed; \
	elif [ "$(word 2,$(MAKECMDGOALS))" = "migrate" ]; then \
		go run cmd/command/main.go e2e migrate; \
	elif [ "$(word 2,$(MAKECMDGOALS))" = "run" ]; then \
		cd e2e && npm run cy:open; \
	elif [ "$(word 2,$(MAKECMDGOALS))" = "clean" ]; then \
		go run cmd/command/main.go e2e drop; \
	elif [ "$(word 2,$(MAKECMDGOALS))" = "dev" ]; then \
		PORT=3201 ORIGIN='http://localhost:3201' DB_NAME=iota_erp_e2e air -c .air.e2e.toml; \
	else \
		echo "Usage: make e2e [test|reset|seed|migrate|run|dev|clean]"; \
		echo "  test         - Set up database and run all e2e tests"; \
		echo "  reset        - Drop and recreate e2e database with fresh data"; \
		echo "  seed         - Seed e2e database with test data"; \
		echo "  migrate      - Run migrations on e2e database"; \
		echo "  run          - Open Cypress interactive mode"; \
		echo "  dev          - Start e2e development server with hot reload on port 3201"; \
		echo "  clean        - Drop e2e database"; \
	fi

# Build and release management
build:
	@if [ "$(word 2,$(MAKECMDGOALS))" = "local" ]; then \
		go build -ldflags="-s -w" -o run_server cmd/server/main.go; \
	elif [ "$(word 2,$(MAKECMDGOALS))" = "linux" ]; then \
		CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o /build/run_server cmd/server/main.go; \
	elif [ "$(word 2,$(MAKECMDGOALS))" = "docker-base" ]; then \
		docker buildx build --push --platform linux/amd64,linux/arm64 -t iotauz/sdk:base-$v --target base .; \
	elif [ "$(word 2,$(MAKECMDGOALS))" = "docker-prod" ]; then \
		docker buildx build --push --platform linux/amd64,linux/arm64 -t iotauz/sdk:$v --target production .; \
	else \
		echo "Usage: make build [local|linux|docker-base|docker-prod]"; \
		echo "  local       - Build for local OS"; \
		echo "  linux       - Build for Alpine Linux (production)"; \
		echo "  docker-base - Build and push base Docker image"; \
		echo "  docker-prod - Build and push production Docker image"; \
	fi

# Dependency graph generation
graph:
	goda graph ./modules/... | dot -Tpng -o dependencies.png

# Code quality checks with subcommands (fmt, lint, tr)
check:
	@if [ "$(word 2,$(MAKECMDGOALS))" = "fmt" ]; then \
		goimports -w . && templ fmt . && go mod tidy && templ generate; \
	elif [ "$(word 2,$(MAKECMDGOALS))" = "lint" ]; then \
		golangci-lint run ./...; \
	elif [ "$(word 2,$(MAKECMDGOALS))" = "tr" ]; then \
		go run cmd/command/main.go check_tr_keys; \
	else \
		echo "Usage: make check [fmt|lint|tr]"; \
		echo "  fmt  - Format Go code, templates, and tidy modules"; \
		echo "  lint - Run golangci-lint (checks for unused variables/functions)"; \
		echo "  tr   - Check translations for completeness"; \
	fi


# Cloudflared tunnel
tunnel:
	cloudflared tunnel --url http://localhost:3200 --loglevel debug

# Clean build artifacts
clean:
	rm -rf $(TAILWIND_OUTPUT)


# Full setup
setup: deps css
	make check lint

# Prevents make from treating the argument as an undefined target
%:
	@:

.PHONY: deps db test css compose setup e2e build graph docs tunnel clean generate check \
        up down restart logs local stop reset seed migrate watch coverage verbose package docker score report \
        dev fmt lint tr linux docker-base docker-prod run server
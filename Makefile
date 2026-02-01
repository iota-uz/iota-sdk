# Variables
TAILWIND_INPUT := modules/core/presentation/assets/css/main.css
TAILWIND_OUTPUT := modules/core/presentation/assets/css/main.min.css

# Install dependencies
deps:
	go get ./...

# Documentation management with subcommands (serve, install)
docs:
	@if [ "$(word 2,$(MAKECMDGOALS))" = "dev" ]; then \
		cd docs && pnpm dev; \
	elif [ "$(word 2,$(MAKECMDGOALS))" = "build" ]; then \
		cd docs && pnpm build; \
	elif [ "$(word 2,$(MAKECMDGOALS))" = "install" ]; then \
		cd docs && pnpm install; \
	else \
		echo "Usage: make docs [dev|build|install]"; \
		echo "  dev     - Start local Nextra dev server on port 4000"; \
		echo "  build   - Build static site to docs/out/"; \
		echo "  install - Install Node.js/pnpm dependencies"; \
	fi

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
		docker volume rm iota-sdk-data || true; \
	elif [ "$(word 2,$(MAKECMDGOALS))" = "reset" ]; then \
		docker compose -f compose.dev.yml down db && docker volume rm iota-sdk-data || true && docker compose -f compose.dev.yml up db; \
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
		gow test -tags dev -v ./...; \
	elif [ "$(word 2,$(MAKECMDGOALS))" = "coverage" ]; then \
		go test -tags dev -v ./... -coverprofile=./coverage/coverage.out; \
	elif [ "$(word 2,$(MAKECMDGOALS))" = "verbose" ]; then \
		go test -tags dev -v ./...; \
	elif [ "$(word 2,$(MAKECMDGOALS))" = "docker" ]; then \
		docker compose -f compose.testing.yml up --build erp_local; \
	elif [ "$(word 2,$(MAKECMDGOALS))" = "score" ]; then \
		go tool cover -func coverage.out | grep "total:" | awk '{print ((int($$3) > 80) != 1) }'; \
	elif [ "$(word 2,$(MAKECMDGOALS))" = "report" ]; then \
		go tool cover -html=coverage.out -o ./coverage/cover.html; \
	else \
		go test -tags dev ./...; \
	fi

# Compile TailwindCSS with optional subcommands (css, watch, dev, clean)
css:
	@if [ "$(word 2,$(MAKECMDGOALS))" = "watch" ]; then \
		tailwindcss -i $(TAILWIND_INPUT) -o $(TAILWIND_OUTPUT) --minify --watch; \
	elif [ "$(word 2,$(MAKECMDGOALS))" = "dev" ]; then \
		tailwindcss -i $(TAILWIND_INPUT) -o $(TAILWIND_OUTPUT); \
	elif [ "$(word 2,$(MAKECMDGOALS))" = "clean" ]; then \
		rm -rf $(TAILWIND_OUTPUT); \
	else \
		tailwindcss -i $(TAILWIND_INPUT) -o $(TAILWIND_OUTPUT) --minify; \
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
		cd e2e && npx playwright test --ui; \
	elif [ "$(word 2,$(MAKECMDGOALS))" = "ci" ]; then \
		cd e2e && npx playwright test --workers=1 --reporter=list; \
	elif [ "$(word 2,$(MAKECMDGOALS))" = "clean" ]; then \
		go run cmd/command/main.go e2e drop; \
	elif [ "$(word 2,$(MAKECMDGOALS))" = "dev" ]; then \
		PORT=3201 ORIGIN='http://localhost:3201' DB_NAME=iota_erp_e2e ENABLE_TEST_ENDPOINTS=true air; \
	else \
		echo "Usage: make e2e [test|reset|seed|migrate|run|ci|dev|clean]"; \
		echo "  test         - Set up database and run all e2e tests"; \
		echo "  reset        - Drop and recreate e2e database with fresh data"; \
		echo "  seed         - Seed e2e database with test data"; \
		echo "  migrate      - Run migrations on e2e database"; \
		echo "  run          - Open Playwright interactive mode (UI mode)"; \
		echo "  ci           - Run Playwright tests in CI mode (headless, no UI, serial execution)"; \
		echo "  dev          - Start e2e development server with hot reload on port 3201"; \
		echo "  clean        - Drop e2e database"; \
	fi

# Build and release management
build:
	@if [ "$(word 2,$(MAKECMDGOALS))" = "dev" ]; then \
		go build -tags dev -ldflags="-s -w" -o run_server cmd/server/main.go; \
	elif [ "$(word 2,$(MAKECMDGOALS))" = "local" ]; then \
		go build -ldflags="-s -w" -o run_server cmd/server/main.go; \
	elif [ "$(word 2,$(MAKECMDGOALS))" = "prod" ]; then \
		go build -ldflags="-s -w" -o run_server cmd/server/main.go; \
	elif [ "$(word 2,$(MAKECMDGOALS))" = "linux" ]; then \
		CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o /build/run_server cmd/server/main.go; \
	elif [ "$(word 2,$(MAKECMDGOALS))" = "docker-base" ]; then \
		docker buildx build --push --platform linux/amd64,linux/arm64 -t iotauz/sdk:base-$v --target base .; \
	elif [ "$(word 2,$(MAKECMDGOALS))" = "docker-prod" ]; then \
		docker buildx build --push --platform linux/amd64,linux/arm64 -t iotauz/sdk:$v --target production .; \
	else \
		echo "Usage: make build [dev|local|prod|linux|docker-base|docker-prod]"; \
		echo "  dev         - Build with dev tags for development"; \
		echo "  local       - Build for local OS (production mode)"; \
		echo "  prod        - Build for production (explicit)"; \
		echo "  linux       - Build for Alpine Linux (production)"; \
		echo "  docker-base - Build and push base Docker image"; \
		echo "  docker-prod - Build and push production Docker image"; \
	fi

# Super Admin server management with subcommands (default, dev, seed)
superadmin:
	@if [ "$(word 2,$(MAKECMDGOALS))" = "dev" ]; then \
		PORT=4000 DOMAIN='localhost:4000' ORIGIN='http://localhost:4000' air -c .air.superadmin.toml; \
	elif [ "$(word 2,$(MAKECMDGOALS))" = "seed" ]; then \
		LOG_LEVEL=info go run cmd/command/main.go seed_superadmin; \
	else \
		PORT=4000 DOMAIN='localhost:4000' go run cmd/superadmin/main.go; \
	fi

# Dependency graph generation
graph:
	goda graph ./modules/... | dot -Tpng -o dependencies.png

# Auto-fix code formatting and imports
fix:
	@if [ "$(word 2,$(MAKECMDGOALS))" = "fmt" ]; then \
		go fmt ./... && templ fmt .; \
	elif [ "$(word 2,$(MAKECMDGOALS))" = "imports" ]; then \
		find . -name '*.go' -not -name '*_templ.go' -exec goimports -w {} +; \
	else \
		echo "Usage: make fix [fmt|imports]"; \
		echo "  fmt     - Format Go code and templates"; \
		echo "  imports - Organize and format Go imports"; \
	fi

# Code quality checks with subcommands (lint, tr)
check:
	@if [ "$(word 2,$(MAKECMDGOALS))" = "lint" ]; then \
		golangci-lint run --build-tags dev ./...; \
	elif [ "$(word 2,$(MAKECMDGOALS))" = "tr" ]; then \
		go run cmd/command/main.go check_tr_keys; \
	else \
		echo "Usage: make check [lint|tr]"; \
		echo "  lint - Run golangci-lint (checks for unused variables/functions)"; \
		echo "  tr   - Check translations for completeness"; \
	fi

# sdk-tools CLI management
.PHONY: sdk-tools
sdk-tools:
	@if [ "$(word 2,$(MAKECMDGOALS))" = "install" ]; then \
		echo "Installing sdk-tools..."; \
		cd .claude/tools && GOWORK=off go install .; \
		echo "✓ Installed sdk-tools to $$(go env GOPATH)/bin/sdk-tools"; \
		echo "Make sure $$(go env GOPATH)/bin is in your PATH"; \
	elif [ "$(word 2,$(MAKECMDGOALS))" = "test" ]; then \
		echo "Running sdk-tools tests..."; \
		cd .claude/tools && GOWORK=off go test ./... -v; \
	elif [ "$(word 2,$(MAKECMDGOALS))" = "help" ]; then \
		sdk-tools --help; \
	else \
		echo "Usage: make sdk-tools [install|test|help]"; \
		echo "  install - Install sdk-tools globally to \$$GOPATH/bin"; \
		echo "  test    - Run tests"; \
		echo "  help    - Show sdk-tools help"; \
	fi

# Cloudflared tunnel
tunnel:
	cloudflared tunnel --url http://localhost:3200 --loglevel debug

# Clean build artifacts
clean:
	rm -rf $(TAILWIND_OUTPUT)

# Development watch mode - run templ and tailwind in watch mode concurrently
dev:
	@if [ "$(word 2,$(MAKECMDGOALS))" = "watch" ]; then \
		echo "Starting development watch mode (templ + tailwind)..."; \
		trap 'kill %1 %2 2>/dev/null; exit' INT TERM; \
		templ generate --watch & \
		tailwindcss -i $(TAILWIND_INPUT) -o $(TAILWIND_OUTPUT) --watch & \
		wait; \
	else \
		echo "Usage: make dev [watch]"; \
		echo "  watch - Run templ and tailwind in watch mode concurrently"; \
	fi

# Full setup
setup: deps css
	make fix fmt
	make fix imports
	make check lint

# Prevents make from treating the argument as an undefined target
watch coverage verbose docker score report prod linux docker-base docker-prod up down restart logs local stop reset seed migrate install help imports serve:
	@:

.PHONY: deps db test css compose setup e2e build graph docs tunnel clean generate check fix superadmin \
        down restart logs local stop reset watch coverage verbose docker score report \
        dev fmt lint tr linux docker-base docker-prod run server sdk-tools install help
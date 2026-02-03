set shell := ["bash", "-eu", "-o", "pipefail", "-c"]

TAILWIND_INPUT := "modules/core/presentation/assets/css/main.css"
TAILWIND_OUTPUT := "modules/core/presentation/assets/css/main.min.css"

default:
  @just --list

# Install dependencies
deps:
  go get ./...

install-tools:
  bash scripts/install-dev-tools.sh

# Documentation management (dev, build, install)
docs cmd="help":
  case "{{cmd}}" in \
    dev) (cd docs && pnpm dev) ;; \
    build) (cd docs && pnpm build) ;; \
    install) (cd docs && pnpm install) ;; \
    *) \
      echo "Usage: just docs [dev|build|install]" ; \
      echo "  dev     - Start local Nextra dev server on port 4000" ; \
      echo "  build   - Build static site to docs/out/" ; \
      echo "  install - Install Node.js/pnpm dependencies" ; \
      exit 2 ;; \
  esac

# Template generation (generate, watch)
generate cmd="":
  if [ "{{cmd}}" = "watch" ]; then \
    templ generate --watch ; \
  else \
    go generate ./... && templ generate ; \
  fi

# Docker compose management (up, down, restart, logs)
compose cmd="help":
  case "{{cmd}}" in \
    up) docker compose -f compose.dev.yml up ;; \
    down) docker compose -f compose.dev.yml down ;; \
    restart) docker compose -f compose.dev.yml down && docker compose -f compose.dev.yml up ;; \
    logs) docker compose -f compose.dev.yml logs -f ;; \
    *) \
      echo "Usage: just compose [up|down|restart|logs]" ; \
      echo "  up      - Start all development services" ; \
      echo "  down    - Stop all development services" ; \
      echo "  restart - Stop and start all services" ; \
      echo "  logs    - Follow logs from all services" ; \
      exit 2 ;; \
  esac

# Database management (local, stop, clean, reset, seed, migrate)
db cmd="help" direction="":
  case "{{cmd}}" in \
    local) docker compose -f compose.dev.yml up db ;; \
    stop) docker compose -f compose.dev.yml down db ;; \
    clean) docker volume rm iota-sdk-data || true ;; \
    reset) docker compose -f compose.dev.yml down db && docker volume rm iota-sdk-data || true && docker compose -f compose.dev.yml up db ;; \
    seed) go run cmd/command/main.go seed ;; \
    migrate) go run cmd/command/main.go migrate {{direction}} ;; \
    *) \
      echo "Usage: just db [local|stop|clean|reset|seed|migrate] [up|down|redo|collect]" ; \
      echo "  local   - Start local PostgreSQL database" ; \
      echo "  stop    - Stop database container" ; \
      echo "  clean   - Remove iota-sdk-data docker volume" ; \
      echo "  reset   - Stop, clean, and restart local database" ; \
      echo "  seed    - Seed database with test data" ; \
      echo "  migrate - Run database migrations (up/down/redo/collect)" ; \
      exit 2 ;; \
  esac

# Run tests (watch, coverage, verbose, package, docker, score, report)
test cmd="":
  case "{{cmd}}" in \
    watch) gow test -tags dev -v ./... ;; \
    coverage) mkdir -p ./coverage && go test -tags dev -v ./... -coverprofile=./coverage/coverage.out ;; \
    verbose) go test -tags dev -v ./... ;; \
    docker) docker compose -f compose.testing.yml up --build erp_local ;; \
    score) go tool cover -func ./coverage/coverage.out | grep "total:" | awk '{print ((int($$3) > 80) != 1) }' ;; \
    report) go tool cover -html=./coverage/coverage.out -o ./coverage/cover.html ;; \
    "") go test -tags dev ./... ;; \
    *) \
      echo "Usage: just test [watch|coverage|verbose|docker|score|report]" ; \
      exit 2 ;; \
  esac

# Compile TailwindCSS (css, watch, dev, clean)
css cmd="":
  case "{{cmd}}" in \
    watch) tailwindcss -c tailwind.config.js -i {{TAILWIND_INPUT}} -o {{TAILWIND_OUTPUT}} --minify --watch ;; \
    dev) tailwindcss -c tailwind.config.js -i {{TAILWIND_INPUT}} -o {{TAILWIND_OUTPUT}} ;; \
    clean) rm -rf {{TAILWIND_OUTPUT}} ;; \
    "") tailwindcss -c tailwind.config.js -i {{TAILWIND_INPUT}} -o {{TAILWIND_OUTPUT}} --minify ;; \
    *) \
      echo "Usage: just css [watch|dev|clean]" ; \
      exit 2 ;; \
  esac

# E2E testing management (test, reset, seed, migrate, run, ci, dev, clean)
e2e cmd="help" direction="":
  case "{{cmd}}" in \
    test) go run cmd/command/main.go e2e test ;; \
    reset) go run cmd/command/main.go e2e reset ;; \
    seed) go run cmd/command/main.go e2e seed ;; \
    migrate) go run cmd/command/main.go e2e migrate {{direction}} ;; \
    run) (cd e2e && npx playwright test --ui) ;; \
    ci) (cd e2e && npx playwright test --workers=1 --reporter=list) ;; \
    clean) go run cmd/command/main.go e2e drop ;; \
    dev) PORT=3201 ORIGIN='http://localhost:3201' DB_NAME=iota_erp_e2e ENABLE_TEST_ENDPOINTS=true air ;; \
    *) \
      echo "Usage: just e2e [test|reset|seed|migrate|run|ci|dev|clean] [up|down|redo|collect]" ; \
      exit 2 ;; \
  esac

# Build and release management (dev, local, prod, linux, docker-base, docker-prod)
build cmd="help" v="":
  case "{{cmd}}" in \
    dev) go build -tags dev -ldflags="-s -w" -o run_server cmd/server/main.go ;; \
    local) go build -ldflags="-s -w" -o run_server cmd/server/main.go ;; \
    prod) go build -ldflags="-s -w" -o run_server cmd/server/main.go ;; \
    linux) CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o /build/run_server cmd/server/main.go ;; \
    docker-base) \
      if [ -z "{{v}}" ]; then echo "Usage: just build docker-base <version>"; exit 2; fi ; \
      docker buildx build --push --platform linux/amd64,linux/arm64 -t iotauz/sdk:base-{{v}} --target base . ;; \
    docker-prod) \
      if [ -z "{{v}}" ]; then echo "Usage: just build docker-prod <version>"; exit 2; fi ; \
      docker buildx build --push --platform linux/amd64,linux/arm64 -t iotauz/sdk:{{v}} --target production . ;; \
    *) \
      echo "Usage: just build [dev|local|prod|linux|docker-base|docker-prod] [version]" ; \
      exit 2 ;; \
  esac

# Super Admin server management (default, dev, seed)
superadmin cmd="":
  case "{{cmd}}" in \
    dev) PORT=4000 DOMAIN='localhost:4000' ORIGIN='http://localhost:4000' air -c .air.superadmin.toml ;; \
    seed) LOG_LEVEL=info go run cmd/command/main.go seed_superadmin ;; \
    "") PORT=4000 DOMAIN='localhost:4000' go run cmd/superadmin/main.go ;; \
    *) \
      echo "Usage: just superadmin [dev|seed]" ; \
      exit 2 ;; \
  esac

# Dependency graph generation
graph:
  goda graph ./modules/... | dot -Tpng -o dependencies.png

# Auto-fix code formatting and imports
fix cmd="help":
  case "{{cmd}}" in \
    fmt) go fmt ./... && templ fmt . ;; \
    imports) find . -name '*.go' -not -name '*_templ.go' -exec goimports -w {} + ;; \
    *) \
      echo "Usage: just fix [fmt|imports]" ; \
      exit 2 ;; \
  esac

# Code quality checks (lint, tr)
check cmd="help":
  case "{{cmd}}" in \
    lint) golangci-lint run --build-tags dev ./... ;; \
    tr) go run cmd/command/main.go check_tr_keys ;; \
    *) \
      echo "Usage: just check [lint|tr]" ; \
      exit 2 ;; \
  esac

# sdk-tools CLI management (install, test, help)
sdk-tools cmd="help":
  case "{{cmd}}" in \
    install) \
      echo "Installing sdk-tools..." ; \
      (cd .claude/tools && GOWORK=off go install .) ; \
      echo "✓ Installed sdk-tools to $(go env GOPATH)/bin/sdk-tools" ; \
      echo "Make sure $(go env GOPATH)/bin is in your PATH" ;; \
    test) \
      echo "Running sdk-tools tests..." ; \
      (cd .claude/tools && GOWORK=off go test ./... -v) ;; \
    help) sdk-tools --help ;; \
    *) \
      echo "Usage: just sdk-tools [install|test|help]" ; \
      exit 2 ;; \
  esac

# Cloudflared tunnel
tunnel:
  cloudflared tunnel --url http://localhost:3200 --loglevel debug

# Clean build artifacts
clean:
  rm -rf {{TAILWIND_OUTPUT}}

# Development watch mode and dev servers (watch, bichat)
dev cmd="help":
  case "{{cmd}}" in \
    watch) \
      echo "Starting development watch mode (templ + tailwind)..." ; \
      trap 'kill %1 %2 2>/dev/null || true; exit' INT TERM ; \
      templ generate --watch & \
      tailwindcss -c tailwind.config.js -i {{TAILWIND_INPUT}} -o {{TAILWIND_OUTPUT}} --watch & \
      wait ;; \
    bichat) ./scripts/dev-bichat.sh ;; \
    *) \
      echo "Usage: just dev [watch|bichat]" ; \
      exit 2 ;; \
  esac

# Full setup
setup:
  just deps
  just css
  just fix fmt
  just fix imports
  just check lint

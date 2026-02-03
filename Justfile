set shell := ["bash", "-eu", "-o", "pipefail", "-c"]
set dotenv-load

DEV_COMPOSE_FILE := "compose.dev.yml"
TESTING_COMPOSE_FILE := "compose.testing.yml"

TAILWIND_CONFIG := "tailwind.config.js"
TAILWIND_INPUT := "modules/core/presentation/assets/css/main.css"
TAILWIND_OUTPUT := "modules/core/presentation/assets/css/main.min.css"

GO_TEST_TAG := "dev"

[default]
[group("meta")]
[doc("Show recipes")]
help:
  @just --groups
  @echo
  @just --list --unsorted
  @echo
  @echo "Examples:"
  @echo "  just setup"
  @echo "  just css"
  @echo "  just dev watch"
  @echo "  just db migrate up"
  @echo "  just test coverage"
  @echo "  just e2e ci"

[group("meta")]
[doc("Install Go dependencies")]
deps:
  go get ./...

[group("meta")]
[doc("Install local dev tools")]
install-tools:
  bash scripts/install-dev-tools.sh

[group("docs")]
[doc("Documentation commands (dev|build|install)")]
docs cmd="help":
  case "{{cmd}}" in \
    dev) just docs-dev ;; \
    build) just docs-build ;; \
    install) just docs-install ;; \
    *) \
      echo "Usage: just docs [dev|build|install]" ; \
      exit 2 ;; \
  esac

[group("docs")]
[working-directory: "docs"]
docs-dev:
  pnpm dev

[group("docs")]
[working-directory: "docs"]
docs-build:
  pnpm build

[group("docs")]
[working-directory: "docs"]
docs-install:
  pnpm install

[group("codegen")]
[doc("Generate Go + templ (or watch)")]
generate cmd="":
  if [ "{{cmd}}" = "watch" ]; then \
    just generate-watch ; \
  elif [ -z "{{cmd}}" ]; then \
    just generate-all ; \
  else \
    echo "Usage: just generate [watch]" ; \
    exit 2 ; \
  fi

[group("codegen")]
generate-all:
  go generate ./... && templ generate

[group("codegen")]
generate-watch:
  templ generate --watch

[group("compose")]
[doc("Compose commands (up|down|restart|logs)")]
compose cmd="help":
  case "{{cmd}}" in \
    up) just compose-up ;; \
    down) just compose-down ;; \
    restart) just compose-restart ;; \
    logs) just compose-logs ;; \
    *) \
      echo "Usage: just compose [up|down|restart|logs]" ; \
      exit 2 ;; \
  esac

[group("compose")]
compose-up:
  docker compose -f {{DEV_COMPOSE_FILE}} up

[group("compose")]
compose-down:
  docker compose -f {{DEV_COMPOSE_FILE}} down

[group("compose")]
compose-restart:
  docker compose -f {{DEV_COMPOSE_FILE}} down && docker compose -f {{DEV_COMPOSE_FILE}} up

[group("compose")]
compose-logs:
  docker compose -f {{DEV_COMPOSE_FILE}} logs -f

[group("db")]
[doc("Database commands (local|stop|clean|reset|seed|migrate)")]
db cmd="help" direction="":
  case "{{cmd}}" in \
    local) just db-local ;; \
    stop) just db-stop ;; \
    clean) just db-clean ;; \
    reset) just db-reset ;; \
    seed) just db-seed ;; \
    migrate) just db-migrate {{direction}} ;; \
    *) \
      echo "Usage: just db [local|stop|clean|reset|seed|migrate] [up|down|redo|collect]" ; \
      exit 2 ;; \
  esac

[group("db")]
db-local:
  docker compose -f {{DEV_COMPOSE_FILE}} up db

[group("db")]
db-stop:
  docker compose -f {{DEV_COMPOSE_FILE}} stop db

[group("db")]
db-clean:
  docker volume rm iota-sdk-data || true

[group("db")]
db-reset:
  docker compose -f {{DEV_COMPOSE_FILE}} stop db && docker volume rm iota-sdk-data || true && docker compose -f {{DEV_COMPOSE_FILE}} up db

[group("db")]
db-seed:
  go run cmd/command/main.go seed

[group("db")]
db-migrate direction:
  go run cmd/command/main.go migrate {{direction}}

[group("quality")]
[doc("Format Go code and templ files")]
fmt: (fix "fmt")

[group("quality")]
[doc("Organize Go imports (excludes *_templ.go)")]
imports: (fix "imports")

[group("quality")]
[doc("Run golangci-lint")]
lint: (check "lint")

[group("quality")]
[doc("Validate translations")]
tr: (check "tr")

[group("quality")]
[doc("Auto-fix commands (fmt|imports)")]
fix cmd="help":
  case "{{cmd}}" in \
    fmt) go fmt ./... && templ fmt . ;; \
    imports) find . -name '*.go' -not -name '*_templ.go' -exec goimports -w {} + ;; \
    *) \
      echo "Usage: just fix [fmt|imports]" ; \
      exit 2 ;; \
  esac

[group("quality")]
[doc("Checks (lint|tr)")]
check cmd="help":
  case "{{cmd}}" in \
    lint) golangci-lint run --build-tags {{GO_TEST_TAG}} ./... ;; \
    tr) go run cmd/command/main.go check_tr_keys ;; \
    *) \
      echo "Usage: just check [lint|tr]" ; \
      exit 2 ;; \
  esac

[group("test")]
[doc("Test commands (watch|coverage|verbose|docker|score|report)")]
test cmd="":
  case "{{cmd}}" in \
    watch) just test-watch ;; \
    coverage) just test-coverage ;; \
    verbose) just test-verbose ;; \
    docker) just test-docker ;; \
    score) just test-score ;; \
    report) just test-report ;; \
    "") just test-all ;; \
    *) \
      echo "Usage: just test [watch|coverage|verbose|docker|score|report]" ; \
      exit 2 ;; \
  esac

[group("test")]
test-all:
  go test -tags {{GO_TEST_TAG}} ./...

[group("test")]
test-watch:
  gow test -tags {{GO_TEST_TAG}} -v ./...

[group("test")]
test-verbose:
  go test -tags {{GO_TEST_TAG}} -v ./...

[group("test")]
test-coverage:
  mkdir -p ./coverage && go test -tags {{GO_TEST_TAG}} -v ./... -coverprofile=./coverage/coverage.out

[group("test")]
test-score:
  go tool cover -func ./coverage/coverage.out | grep "total:" | awk '{print ((int($$3) > 80) != 1) }'

[group("test")]
test-report:
  go tool cover -html=./coverage/coverage.out -o ./coverage/cover.html

[group("test")]
test-docker:
  docker compose -f {{TESTING_COMPOSE_FILE}} up --build erp_local

[group("assets")]
[doc("TailwindCSS build (or watch/dev/clean)")]
css cmd="":
  case "{{cmd}}" in \
    watch) just css-watch ;; \
    dev) just css-dev ;; \
    clean) just css-clean ;; \
    "") just css-build ;; \
    *) \
      echo "Usage: just css [watch|dev|clean]" ; \
      exit 2 ;; \
  esac

[group("assets")]
css-build:
  tailwindcss -c {{TAILWIND_CONFIG}} -i {{TAILWIND_INPUT}} -o {{TAILWIND_OUTPUT}} --minify

[group("assets")]
css-watch:
  tailwindcss -c {{TAILWIND_CONFIG}} -i {{TAILWIND_INPUT}} -o {{TAILWIND_OUTPUT}} --minify --watch

[group("assets")]
css-dev:
  tailwindcss -c {{TAILWIND_CONFIG}} -i {{TAILWIND_INPUT}} -o {{TAILWIND_OUTPUT}}

[group("assets")]
css-clean:
  rm -rf {{TAILWIND_OUTPUT}}

[group("e2e")]
[doc("E2E commands (test|reset|seed|migrate|run|ci|dev|clean)")]
e2e cmd="help" direction="":
  case "{{cmd}}" in \
    test) just e2e-test ;; \
    reset) just e2e-reset ;; \
    seed) just e2e-seed ;; \
    migrate) just e2e-migrate {{direction}} ;; \
    run) just e2e-run ;; \
    ci) just e2e-ci ;; \
    dev) just e2e-dev ;; \
    clean) just e2e-clean ;; \
    *) \
      echo "Usage: just e2e [test|reset|seed|migrate|run|ci|dev|clean] [up|down|redo|collect]" ; \
      exit 2 ;; \
  esac

[group("e2e")]
e2e-test:
  go run cmd/command/main.go e2e test

[group("e2e")]
e2e-reset:
  go run cmd/command/main.go e2e reset

[group("e2e")]
e2e-seed:
  go run cmd/command/main.go e2e seed

[group("e2e")]
e2e-migrate direction:
  go run cmd/command/main.go e2e migrate {{direction}}

[group("e2e")]
[working-directory: "e2e"]
e2e-run:
  npx playwright test --ui

[group("e2e")]
[working-directory: "e2e"]
e2e-ci:
  npx playwright test --workers=1 --reporter=list

[group("e2e")]
e2e-clean:
  go run cmd/command/main.go e2e drop

[group("e2e")]
e2e-dev:
  PORT=3201 ORIGIN='http://localhost:3201' DB_NAME=iota_erp_e2e ENABLE_TEST_ENDPOINTS=true air

[group("build")]
[doc("Build commands (dev|local|prod|linux|docker-base|docker-prod)")]
build cmd="help" v="":
  case "{{cmd}}" in \
    dev) just build-dev ;; \
    local) just build-local ;; \
    prod) just build-prod ;; \
    linux) just build-linux ;; \
    docker-base) just build-docker-base {{v}} ;; \
    docker-prod) just build-docker-prod {{v}} ;; \
    *) \
      echo "Usage: just build [dev|local|prod|linux|docker-base|docker-prod] [version]" ; \
      exit 2 ;; \
  esac

[group("build")]
build-dev:
  go build -tags {{GO_TEST_TAG}} -ldflags="-s -w" -o run_server cmd/server/main.go

[group("build")]
build-local:
  go build -ldflags="-s -w" -o run_server cmd/server/main.go

[group("build")]
build-prod:
  go build -ldflags="-s -w" -o run_server cmd/server/main.go

[group("build")]
build-linux:
  CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o /build/run_server cmd/server/main.go

[group("build")]
build-docker-base v:
  if [ -z "{{v}}" ]; then echo "Usage: just build docker-base <version>"; exit 2; fi
  docker buildx build --push --platform linux/amd64,linux/arm64 -t iotauz/sdk:base-{{v}} --target base .

[group("build")]
build-docker-prod v:
  if [ -z "{{v}}" ]; then echo "Usage: just build docker-prod <version>"; exit 2; fi
  docker buildx build --push --platform linux/amd64,linux/arm64 -t iotauz/sdk:{{v}} --target production .

[group("superadmin")]
[doc("Superadmin commands (dev|seed)")]
superadmin cmd="":
  case "{{cmd}}" in \
    dev) just superadmin-dev ;; \
    seed) just superadmin-seed ;; \
    "") just superadmin-run ;; \
    *) \
      echo "Usage: just superadmin [dev|seed]" ; \
      exit 2 ;; \
  esac

[group("superadmin")]
superadmin-run:
  PORT=4000 DOMAIN='localhost:4000' go run cmd/superadmin/main.go

[group("superadmin")]
superadmin-dev:
  PORT=4000 DOMAIN='localhost:4000' ORIGIN='http://localhost:4000' air -c .air.superadmin.toml

[group("superadmin")]
superadmin-seed:
  LOG_LEVEL=info go run cmd/command/main.go seed_superadmin

[group("tools")]
[doc("sdk-tools commands (install|test|help)")]
sdk-tools cmd="help":
  case "{{cmd}}" in \
    install) just sdk-tools-install ;; \
    test) just sdk-tools-test ;; \
    help) sdk-tools --help ;; \
    *) \
      echo "Usage: just sdk-tools [install|test|help]" ; \
      exit 2 ;; \
  esac

[group("tools")]
[working-directory: ".claude/tools"]
sdk-tools-install:
  echo "Installing sdk-tools..."
  GOWORK=off go install .
  echo "✓ Installed sdk-tools to $(go env GOPATH)/bin/sdk-tools"
  echo "Make sure $(go env GOPATH)/bin is in your PATH"

[group("tools")]
[working-directory: ".claude/tools"]
sdk-tools-test:
  echo "Running sdk-tools tests..."
  GOWORK=off go test ./... -v

[group("tools")]
[doc("Dependency graph")]
graph:
  goda graph ./modules/... | dot -Tpng -o dependencies.png

[group("tools")]
[doc("Cloudflared tunnel")]
tunnel:
  cloudflared tunnel --url http://localhost:3200 --loglevel debug

[group("assets")]
[doc("Clean build artifacts")]
clean: (css "clean")

[group("dev")]
[doc("Dev commands (watch|bichat)")]
dev cmd="help":
  case "{{cmd}}" in \
    watch) just dev-watch ;; \
    bichat) just dev-bichat ;; \
    *) \
      echo "Usage: just dev [watch|bichat]" ; \
      exit 2 ;; \
  esac

[group("dev")]
dev-watch:
  echo "Starting development watch mode (templ + tailwind)..."
  trap 'kill %1 %2 2>/dev/null || true; exit' INT TERM
  templ generate --watch &
  tailwindcss -c {{TAILWIND_CONFIG}} -i {{TAILWIND_INPUT}} -o {{TAILWIND_OUTPUT}} --watch &
  wait

[group("dev")]
dev-bichat:
  ./scripts/dev-bichat.sh

[group("meta")]
[doc("Full local setup")]
setup: deps css fmt imports lint

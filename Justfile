set shell := ["bash", "-eu", "-o", "pipefail", "-c"]
set dotenv-load

DEV_COMPOSE_FILE := "compose.dev.yml"
TESTING_COMPOSE_FILE := "compose.testing.yml"

TAILWIND_INPUT := "styles/tailwind/input.css"
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
  @echo "  just css --watch"
  @echo "  just dev watch"
  @echo "  just db migrate up"
  @echo "  just test -v"
  @echo "  just test -v -coverprofile=./coverage/coverage.out"
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
docs cmd="help" *args="":
  case "{{cmd}}" in \
    dev|build|install) (cd docs && pnpm {{cmd}} {{args}}) ;; \
    *) \
      echo "Usage: just docs [dev|build|install]" ; \
      exit 2 ;; \
  esac

[group("codegen")]
[doc("Generate Go + templ (or watch)")]
generate cmd="":
  if [ "{{cmd}}" = "watch" ]; then \
    just _generate-watch ; \
  elif [ -z "{{cmd}}" ]; then \
    just _generate-all ; \
  else \
    echo "Usage: just generate [watch]" ; \
    exit 2 ; \
  fi

[group("codegen")]
_generate-all:
  go generate ./... && templ generate

[group("codegen")]
_generate-watch:
  templ generate --watch

[group("compose")]
[doc("Compose commands (up|down|restart|logs)")]
compose cmd="help":
  case "{{cmd}}" in \
    up) just _compose-up ;; \
    down) just _compose-down ;; \
    restart) just _compose-restart ;; \
    logs) just _compose-logs ;; \
    *) \
      echo "Usage: just compose [up|down|restart|logs]" ; \
      exit 2 ;; \
  esac

[group("compose")]
_compose-up:
  docker compose -f {{DEV_COMPOSE_FILE}} up

[group("compose")]
_compose-down:
  docker compose -f {{DEV_COMPOSE_FILE}} down

[group("compose")]
_compose-restart:
  docker compose -f {{DEV_COMPOSE_FILE}} down && docker compose -f {{DEV_COMPOSE_FILE}} up

[group("compose")]
_compose-logs:
  docker compose -f {{DEV_COMPOSE_FILE}} logs -f

[group("db")]
[doc("Database commands (local|stop|clean|reset|seed|migrate|status)")]
db cmd="help" direction="":
  case "{{cmd}}" in \
    local) just _db-local ;; \
    stop) just _db-stop ;; \
    clean) just _db-clean ;; \
    reset) just _db-reset ;; \
    seed) just _db-seed ;; \
    migrate) just _db-migrate {{direction}} ;; \
    status) just _db-migrate status ;; \
    *) \
      echo "Usage: just db [local|stop|clean|reset|seed|migrate|status] [for migrate: up|down|redo|status]" ; \
      exit 2 ;; \
  esac

[group("db")]
_db-local:
  docker compose -f {{DEV_COMPOSE_FILE}} up db

[group("db")]
_db-stop:
  docker compose -f {{DEV_COMPOSE_FILE}} stop db

[group("db")]
_db-clean:
  docker volume rm iota-sdk-data || true

[group("db")]
_db-reset:
  docker compose -f {{DEV_COMPOSE_FILE}} stop db && docker volume rm iota-sdk-data || true && docker compose -f {{DEV_COMPOSE_FILE}} up db

[group("db")]
_db-seed:
  go run cmd/command/main.go seed

[group("db")]
_db-migrate direction:
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
[doc("Run tests. Extra arguments are passed to `go test`")]
test *args="":
  mkdir -p ./coverage
  go test -tags {{GO_TEST_TAG}} {{args}} ./...

[group("test")]
[doc("Watch tests with gow")]
test-watch *args="":
  gow test -tags {{GO_TEST_TAG}} -v {{args}} ./...

[group("test")]
[doc("Run tests in docker")]
test-docker:
  docker compose -f {{TESTING_COMPOSE_FILE}} up --build erp_local

[group("test")]
[doc("Print coverage score (requires ./coverage/coverage.out)")]
coverage-score:
  go tool cover -func ./coverage/coverage.out | grep "total:" | awk '{print ((int($$3) > 80) != 1) }'

[group("test")]
[doc("Generate coverage HTML report (requires ./coverage/coverage.out)")]
coverage-report:
  go tool cover -html=./coverage/coverage.out -o ./coverage/cover.html

[group("assets")]
[doc("Build Tailwind CSS. Extra arguments are passed to `tailwindcss`")]
css *args="":
  pnpm exec tailwindcss --input {{TAILWIND_INPUT}} --output {{TAILWIND_OUTPUT}} --minify {{args}}

[group("assets")]
[doc("Remove generated CSS file")]
css-clean:
  rm -rf {{TAILWIND_OUTPUT}}

[group("e2e")]
[doc("E2E commands (test|reset|seed|migrate|status|run|ci|dev|clean)")]
e2e cmd="help" direction="":
  case "{{cmd}}" in \
    test) just _e2e-test ;; \
    reset) just _e2e-reset ;; \
    seed) just _e2e-seed ;; \
    migrate) just _e2e-migrate {{direction}} ;; \
    status) just _e2e-migrate status ;; \
    run) just _e2e-run ;; \
    ci) just _e2e-ci ;; \
    dev) just _e2e-dev ;; \
    clean) just _e2e-clean ;; \
    *) \
      echo "Usage: just e2e [test|reset|seed|migrate|status|run|ci|dev|clean] [for migrate: up|down|redo|status]" ; \
      exit 2 ;; \
  esac

[group("e2e")]
_e2e-test:
  go run cmd/command/main.go e2e test

[group("e2e")]
_e2e-reset:
  go run cmd/command/main.go e2e reset

[group("e2e")]
_e2e-seed:
  go run cmd/command/main.go e2e seed

[group("e2e")]
_e2e-migrate direction:
  go run cmd/command/main.go e2e migrate {{direction}}

[group("e2e")]
[working-directory: "e2e"]
_e2e-run:
  npx playwright test --ui

[group("e2e")]
[working-directory: "e2e"]
_e2e-ci:
  npx playwright test --workers=1 --reporter=list

[group("e2e")]
_e2e-clean:
  go run cmd/command/main.go e2e drop

[group("e2e")]
_e2e-dev:
  PORT=3201 ORIGIN='http://localhost:3201' DB_NAME=iota_erp_e2e ENABLE_TEST_ENDPOINTS=true air

[group("build")]
[doc("Build commands (dev|local|prod|linux|docker-base|docker-prod)")]
build cmd="help" v="":
  case "{{cmd}}" in \
    dev) just _build-dev ;; \
    local) just _build-local ;; \
    prod) just _build-prod ;; \
    linux) just _build-linux ;; \
    docker-base) just _build-docker-base {{v}} ;; \
    docker-prod) just _build-docker-prod {{v}} ;; \
    *) \
      echo "Usage: just build [dev|local|prod|linux|docker-base|docker-prod] [version]" ; \
      exit 2 ;; \
  esac

[group("build")]
_build-dev:
  go build -tags {{GO_TEST_TAG}} -ldflags="-s -w" -o run_server cmd/server/main.go

[group("build")]
_build-local:
  go build -ldflags="-s -w" -o run_server cmd/server/main.go

[group("build")]
_build-prod:
  go build -ldflags="-s -w" -o run_server cmd/server/main.go

[group("build")]
_build-linux:
  CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o /build/run_server cmd/server/main.go

[group("build")]
_build-docker-base v:
  if [ -z "{{v}}" ]; then echo "Usage: just build docker-base <version>"; exit 2; fi
  docker buildx build --push --platform linux/amd64,linux/arm64 -t iotauz/sdk:base-{{v}} --target base .

[group("build")]
_build-docker-prod v:
  if [ -z "{{v}}" ]; then echo "Usage: just build docker-prod <version>"; exit 2; fi
  docker buildx build --push --platform linux/amd64,linux/arm64 -t iotauz/sdk:{{v}} --target production .

[group("superadmin")]
[doc("Superadmin commands (dev|seed)")]
superadmin cmd="":
  case "{{cmd}}" in \
    dev) just _superadmin-dev ;; \
    seed) just _superadmin-seed ;; \
    "") just _superadmin-run ;; \
    *) \
      echo "Usage: just superadmin [dev|seed]" ; \
      exit 2 ;; \
  esac

[group("superadmin")]
_superadmin-run:
  PORT=4000 DOMAIN='localhost:4000' go run cmd/superadmin/main.go

[group("superadmin")]
_superadmin-dev:
  PORT=4000 DOMAIN='localhost:4000' ORIGIN='http://localhost:4000' air -c .air.superadmin.toml

[group("superadmin")]
_superadmin-seed:
  LOG_LEVEL=info go run cmd/command/main.go seed_superadmin

[group("tools")]
[doc("sdk-tools commands (install|test|help)")]
sdk-tools cmd="help":
  case "{{cmd}}" in \
    install) just _sdk-tools-install ;; \
    test) just _sdk-tools-test ;; \
    help) sdk-tools --help ;; \
    *) \
      echo "Usage: just sdk-tools [install|test|help]" ; \
      exit 2 ;; \
  esac

[group("tools")]
[working-directory: ".claude/tools"]
_sdk-tools-install:
  echo "Installing sdk-tools..."
  GOWORK=off go install .
  echo "✓ Installed sdk-tools to $(go env GOPATH)/bin/sdk-tools"
  echo "Make sure $(go env GOPATH)/bin is in your PATH"

[group("tools")]
[working-directory: ".claude/tools"]
_sdk-tools-test:
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
clean: css-clean

[group("dev")]
[doc("Start dev (just dev = watch, just dev bichat = with applet)")]
dev name="":
  go run cmd/dev/main.go {{name}}

[group("dev")]
[doc("Applet commands (rpc-gen)")]
applet cmd="help" name="":
  case "{{cmd}}" in \
    rpc-gen) ./scripts/applet/rpc-gen.sh "{{name}}" ;; \
    *) \
      echo "Usage: just applet [rpc-gen <name>]" ; \
      exit 2 ;; \
  esac

[group("meta")]
[doc("Full local setup")]
setup: deps css fmt imports lint

# Contributing Guide

## ⚙️ Prerequisites

Ensure these tools are installed before you begin:

- [Go v1.23.2](https://golang.org/doc/install)
- [Air v1.61.5](https://github.com/air-verse/air#Installation) - Hot-reloading
- [Docker v27.2.0](https://docs.docker.com/get-docker/) - Containerization
- [Templ v0.3.857](https://templ.guide/) - Templating
- [TailwindCSS v3.4.13](https://tailwindcss.com/docs/installation) - Styling
- [golangci-lint v1.61.0](https://golangci-lint.run/welcome/install/) - Linting

### Windows Setup

1. **Go**: Download from [golang.org](https://golang.org/doc/install), install, and verify with `go version`

2. **Air**:
   ```cmd
   go install github.com/air-verse/air@v1.61.5
   ```
   Add `%USERPROFILE%\go\bin` to your PATH

3. **Docker Desktop**: Download from [docker.com](https://docs.docker.com/desktop/install/windows-install/), enable WSL 2 during installation

4. **Templ**:
   ```cmd
   go install github.com/a-h/templ/cmd/templ@v0.3.857
   ```

5. **TailwindCSS**: Using npm (requires Node.js):
   ```cmd
   npm install -g tailwindcss
   ```
   Or with standalone executable:
   ```cmd
   curl.exe -sLO https://github.com/tailwindlabs/tailwindcss/releases/v3.4.13/download/tailwindcss-windows-x64.exe
   rename tailwindcss-windows-x64.exe tailwindcss.exe
   ```

6. **golangci-lint**:
   ```cmd
   curl.exe -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.ps1 | powershell -Command -
   ```
## 🛠️ Development Setup

1. **Clone the repository**:
   ```bash
   git clone https://github.com/iota-uz/iota-sdk.git
   cd iota-sdk
   ```

2. **Create env file**:
   ```bash
   cp .env.example .env
   ```
   Windows: `copy .env.example .env`

3. **Install dependencies**:
   ```bash
   make deps
   ```
   Windows: If `make` is unavailable, install via [GnuWin32](http://gnuwin32.sourceforge.net/packages/make.htm) or use [Git Bash](https://gitforwindows.org/)

4. **Run PostgreSQL**:
   ```bash
   make localdb
   ```
   Ensure Docker is running before executing this command

5. **Apply migrations**:
   ```bash
   make migrate up && make seed
   ```

6. **Run TailwindCSS in watch mode** (new terminal):
   ```bash
   make css-watch
   ```

7. **Start development server**:
   ```bash
   air
   ```

8. **Access the application**:
   - Web app: [http://localhost:8080](http://localhost:8080)
   - Login credentials:
     - Email: `test@gmail.com`
     - Password: `TestPass123!`
   - GraphQL: [http://localhost:3200/query](http://localhost:3200/query)

## 🧪 Running Tests

To run end-to-end Cypress tests:
1. Ensure you have a migrated and seeded database with a running backend
2. Set environment variables (`DB_USER`, `DB_PASSWORD`, `DB_HOST`, `DB_PORT`, `DB_NAME`) or use default local settings
3. Run the tests:
   ```bash
   cd e2e/
   pnpm cypress run --headed
   ```

## 📚 Documentation

Generate code documentation:

```bash
# For entire project
make docs

# With specific options
go run cmd/document/main.go -dir [directory] -out [output file] [-recursive] [-exclude "dir1,dir2"]
```

Options:
- `-dir`: Target directory (default: current directory)
- `-out`: Output file path (default: DOCUMENTATION.md)
- `-recursive`: Process subdirectories
- `-exclude`: Skip specified directories (comma-separated)

## Explicitly Name Database Constraints

When defining or altering table schemas in `.sql` files that are processed by the `schema/collector` (both in `db/migrations/` and embedded module schemas):

## IMPORTANT NOTE:
Migration tool only supports UNIQUE constraints (other constraints are not yet supported). Keep this in mind when mutating schemas.
Changes in configuration such as altering varchar(255) to varchar(500) or changing datetime from null to now() are not handled.
TODO list of limitations.

**```All constraints (PRIMARY KEY, UNIQUE, FOREIGN KEY, CHECK) MUST be explicitly named using the CONSTRAINT <constraint_name> syntax.```**

### Reasoning:

The `schema/collector` tool automatically generates `Up` and `Down` migration scripts by comparing schema states. To create correct `DROP CONSTRAINT` commands (especially critical for `Down` migrations and for modifying existing constraints), the tool relies on predictable constraint names. Database auto-generated names are inconsistent and difficult for the tool to determine reliably, leading to potential migration failures. Explicit naming ensures that schema comparisons and generated migrations are accurate and robust.

### Recommended Naming Convention:

Please use the following convention for consistency:

`<table>_<column(s)>_<type_suffix>`

**Suffixes:**

* `_pkey` for Primary Keys
* `_key` for Unique Constraints (please be consistent within the project)
* `_fk` for Foreign Keys
* `_check` for Check Constraints

**Note:** For multi-column constraints, include relevant column names separated by underscores if feasible, or provide a meaningful description.

This documentation provides context for LLMs working on the IOTA-SDK project.

## ❓ Known Issues and Troubleshooting

### Linting Issues

Do not run
```shell
golangci-lint run --fix
```
It will break the code.

When facing an error like this:
```
WARN [runner] Can't run linter goanalysis_metalinter: buildssa: failed to load package : could not load export data: no
export data for "github.com/iota-uz/iota-sdk/modules/core/domain/entities/expense_category"
```
Try running:
```shell
go mod tidy
```

### Windows Setup Issues

1. **Make commands fail**:
   - Install via [GnuWin32](http://gnuwin32.sourceforge.net/packages/make.htm)
   - Add installation directory to PATH
   - Or use Git Bash which includes Make

2. **Docker issues**:
   - Ensure WSL 2 is properly configured
   - Run `wsl --update` as administrator
   - Restart Docker Desktop

3. **Air hot-reloading problems**:
   - Verify Air is in your PATH
   - Check for `.air.toml` configuration
   - Try `air init` to create new configuration

4. **PostgreSQL connection issues**:
   - Ensure Docker is running
   - Check container status: `docker ps`
   - Verify database credentials in `.env`

## 🤝 Communication Guidelines

Contributors should close conversations when complete. Reviewers may reopen if needed.

For additional help, see our [FAQ](./FAQ.md) or open a GitHub issue.

# Contributing to GraphQL Schema and Resolvers

## Overview

Our GraphQL setup uses a **hybrid approach** to schema management. We have individual GraphQL schemas defined within each module (e.g., `modules/core/`, `modules/warehouse/`), but we also generate a **unified schema** at the top level (`graph/`) primarily to **enable correct GraphQL introspection** for development tools like Postman, Insomnia, or Apollo Sandbox.

**Why this approach?**

* **Introspection:** The previous method of registering module schemas only at runtime prevented standard introspection tools from seeing the complete API schema. This made development and testing difficult.
* **Unified View:** The build-time merge creates `graph/schema.merged.graphql` and generates corresponding Go code (`graph/generated.go`, `graph/models_gen.go`) that represents the entire API surface. This allows tools to introspect correctly.
* **Runtime Execution:** Currently, the server *still* uses the original `app.RegisterGraphSchema()` mechanism at runtime. This means the actual query execution relies on the schemas defined and registered within each module individually. *([Note: This might be refactored in the future to use the unified schema directly at runtime, which would simplify the process]).*

## Current Limitations & Workflow

Because we merge schemas via simple concatenation for the build-time generation step, but modules still need their *own* complete schemas for runtime registration, there's a necessary, slightly awkward workflow involving commenting/uncommenting common definitions:

1.  **Duplicate Definitions:** Common scalars (like `scalar Time`, `scalar Int64`) and base types (`type Query`, `type Mutation`, `type Subscription`) should ideally be defined *only once* (e.g., in `modules/core/interfaces/graph/base.graphql`).
2.  **Before `go generate`:** To allow the top-level `go generate ./graph/...` to succeed using the merged schema, you **MUST temporarily comment out** any re-definitions of these common scalars/types in other modules' `.graphql` files (e.g., comment out `scalar Time` in `modules/warehouse/interfaces/graph/base.graphql`).
3.  **After `go generate`:** You **MUST uncomment** those lines back in the module `.graphql` files. This is because the runtime `app.RegisterGraphSchema()` for that module needs the complete schema definition, including those scalars, to work correctly.

**Yes, this comment/uncomment step is cumbersome and error-prone.** It's a known trade-off of this hybrid approach. Adhering strictly to defining common types only once (See Solution 1 / Best Practice mentioned previously) and refactoring the runtime to use the single generated `ExecutableSchema` would eliminate this step.

## Build Process Summary

Running `go generate ./graph/...` from the project root performs these steps:

## How Resolvers Work

We have two "layers" of resolvers:

1.  **Module-Specific Resolvers:**
    * Located in `modules/<module_name>/interfaces/graph/*.resolvers.go`.
    * These contain the **actual business logic** for fetching data, calling services, etc.
    * They are associated with the schemas loaded individually at runtime via `app.RegisterGraphSchema`.
    * **Example:** `modules/core/interfaces/graph/users.resolvers.go` implements the logic for the `user` and `users` queries defined in `modules/core/interfaces/graph/users.graphql`.

2.  **Unified Top-Level Resolvers:**
    * Located in `graph/resolver.go` and `graph/*.resolvers.go`.
    * These are generated based on the *merged* schema (`schema.merged.graphql`).
    * The main `graph/resolver.go` defines a `Resolver` struct that holds references to the *module-specific* resolvers (or the main `app` instance).
      ```go
      // graph/resolver.go
      type Resolver struct {
          app               application.Application
          coreResolver      *coregraph.Resolver // From modules/core/interfaces/graph
          warehouseResolver *warehousegraph.Resolver // From modules/warehouse/interfaces/graph
          // ... other module resolvers
      }

      func NewResolver(app application.Application) *Resolver {
          // ... instantiate module resolvers ...
          return &Resolver{ /* ... */ }
      }
      ```
    * The implementation files (e.g., `graph/schema.merged.resolvers.go`) primarily **delegate** calls to the appropriate module-specific resolver.
      ```go
      // graph/schema.merged.resolvers.go (Illustrative)
      func (r *queryResolver) User(ctx context.Context, id int64) (*User, error) { // Uses graph.User model
          // Delegate to the core module's resolver
          coreUserResult, err := r.coreResolver.Query().User(ctx, id) // Calls core's User resolver
          if err != nil {
              return nil, err
          }
          return (*User)(coreUserResult), nil // Direct cast if structs are identical
      }

      func (r *queryResolver) WarehousePosition(ctx context.Context, id int64) (*WarehousePosition, error) { // Uses graph.WarehousePosition
          // Delegate to the warehouse module's resolver
          warehousePosResult, err := r.warehouseResolver.Query().WarehousePosition(ctx, id) // Calls warehouse's resolver
          if err != nil {
              return nil, err
          }
          // return warehousemappers.PositionToGraphModel(warehousePosResult), nil // Example using mapper
          return (*WarehousePosition)(warehousePosResult), nil // Direct cast if structs are identical
      }
      ```

## How to Add/Modify GraphQL Fields

1.  **Define Schema:** Add/modify types, queries, or mutations in the relevant module's `.graphql` file(s) (e.g., `modules/warehouse/interfaces/graph/new_feature.graphql`).
2.  **Implement Logic:** Add the corresponding resolver method implementation in the module's `*.resolvers.go` file (e.g., `modules/warehouse/interfaces/graph/new_feature.resolvers.go`). This implementation should contain the actual business logic.
3.  **Prepare for Generation:** **Temporarily comment out** duplicate scalar/base type definitions in non-core `.graphql` files.
4.  **Generate Unified Code:** Run `go generate ./graph/...` from the project root. This updates/creates:
    * `graph/generated.go`
    * `graph/models_gen.go`
    * Stubs for new resolvers in `graph/*.resolvers.go` (e.g., `graph/schema.merged.resolvers.go`).
5.  **Restore Source Schemas:** **Uncomment** the lines commented out in Step 3.
6.  **Implement Delegation:** Go to the generated stub in the top-level `graph/*.resolvers.go` file. Implement the method by:
    * Getting the correct module resolver instance (e.g., `r.warehouseResolver`).
    * Calling the corresponding method you implemented in the module resolver (Step 2).
    * Mapping the result to the top-level generated model type if necessary (often a simple type cast `(*graph.MyType)(result)` works if the underlying structs are the same, otherwise use a mapper).
7.  **Commit:** Commit changes to the module's `.graphql` and `*.resolvers.go` files, the top-level `graph/*.resolvers.go` file containing the delegation, and all updated generated files in `graph/`.

This workflow allows us to have working introspection while keeping the resolver logic located within the relevant module. Remember the manual comment/uncomment steps before and after generation until the runtime is potentially refactored.
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
   go install github.com/a-h/templ/cmd/templ@v0.3.819
   ```

5. **TailwindCSS**: Using npm (requires Node.js):
   ```cmd
   npm install -g tailwindcss
   ```
   Or with standalone executable:
   ```cmd
   curl.exe -sLO https://github.com/tailwindlabs/tailwindcss/releases/v3.4.15/download/tailwindcss-windows-x64.exe
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
Migration tool only support UNIQUE constraint and other constraints in TODO. Keep it in mind when mutating schemas.
Does not support change in configuration of the type like varchar(255) varchar(500) or datetime from null to now() is not handled.
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

## ❓ Troubleshooting Windows Setup

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
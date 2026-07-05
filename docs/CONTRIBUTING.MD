# Contributing Guide

## ⚙️ Prerequisites

Ensure these tools are installed before you begin:

- [Go v1.24.10](https://golang.org/doc/install)
- [Air v1.61.5](https://github.com/air-verse/air#Installation) - Hot-reloading
- [Docker v27.2.0](https://docs.docker.com/get-docker/) - Containerization
- [Templ v0.3.857](https://templ.guide/) - Templating
- [TailwindCSS v3.4.13](https://tailwindcss.com/docs/installation) - Styling
- [golangci-lint 1.64.8](https://golangci-lint.run/welcome/install/) - Linting
- [Just](https://github.com/casey/just) - Task runner
- [cloudflared](https://developers.cloudflare.com/cloudflare-one/connections/connect-networks/downloads/) - Tunnel (Optional)

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

7. **Just**
   ```cmd
   winget install --id Casey.Just
   ```

8. **cloudflared**
   ```cmd
   winget install --id Cloudflare.cloudflared
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
   just deps
   ```

4. **Run PostgreSQL**:
   ```bash
   just db local
   ```
   Ensure Docker is running before executing this command

5. **Apply migrations**:
   ```bash
   just db migrate up && just db seed
   ```

6. **Run TailwindCSS in watch mode** (new terminal):
   ```bash
   just css --watch
   ```

7. **Start development server**:
   ```bash
   just dev
   ```

8. **Access the application**:
   - Web app: [http://localhost:3200](http://localhost:3200)
   - Login credentials:
     - Email: `test@gmail.com`
     - Password: `TestPass123!`
   - GraphQL: [http://localhost:3200/query](http://localhost:3200/query)

### Development Tools Manager

For a more convenient development experience, use the development task runner to manage the project processes:

```bash
just dev
```

To include an applet during development, run `just dev <applet>`.

## 🧪 Running Tests

### E2E Testing

The project includes Playwright end-to-end tests that run against a separate database to isolate test data from development data.

#### Setup:
1. **Create and prepare e2e database**:
   ```bash
   just e2e reset
   ```

2. **Start e2e server** (in a separate terminal):
   ```bash
   just e2e dev
   ```
   This starts the server on port 3201 connected to the e2e database (`iota_erp_e2e`).

3. **Run tests**:
   ```bash
   # All tests
   just e2e ci

   # Interactive mode
   just e2e run

   # Or directly with npm
   cd e2e/
   npx playwright test
   ```

#### Available E2E Commands:
- `just e2e reset` - Drop and recreate e2e database with fresh data
- `just e2e seed` - Seed the e2e database
- `just e2e migrate up` - Apply e2e migrations
- `just e2e dev` - Start the e2e server on port 3201
- `just e2e ci` - Run all e2e tests in headless CI mode
- `just e2e run` - Open Playwright UI mode
- `just e2e clean` - Drop e2e database

#### Important Notes:
- E2E tests use a separate database (`iota_erp_e2e`) from development (`iota_erp`)
- E2E server runs on port 3201, development server runs on port 3200
- Always run `just e2e dev` before running tests to ensure server connects to correct database
- Configuration files: `/e2e/.env.e2e`, `/e2e/playwright.config.ts`

## 📚 Documentation

Work with the documentation site:

```bash
# Install documentation dependencies
just docs install

# Start the documentation site locally
just docs dev

# Build the documentation site
just docs build
```

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

1. **Just commands fail**:
   - Install Just with `winget install --id Casey.Just`
   - Add the installation directory to PATH
   - Restart your terminal after installation

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

### Start cloudflare tunnel

To intercept incoming traffic from third-party systems, such as payment gateways, you can use a Cloudflare Tunnel.

Try running:
```bash
just tunnel
```

## 🤖 Using AI

### Claude Code Setup

1. **Install Claude Code**:
   ```bash
   npm install -g @anthropic-ai/claude-code
   ```

   See [Claude Code docs](https://docs.anthropic.com/en/docs/claude-code/overview) for more details.

2. **Add MCP servers**:
   ```bash
   # Context7 (documentation and API references)
   claude mcp add -s user --transport http context7 https://mcp.context7.com/mcp

   # Puppeteer (browser automation)
   claude mcp add -s user puppeteer -e 'PUPPETEER_LAUNCH_OPTIONS={"headless":false,"args":["--remote-debugging-port=9222","--remote-debugging-address=127.0.0.1","--user-data-dir=/tmp/mcp-profile","--no-sandbox","--disable-setuid-sandbox"]}' -- npx -y puppeteer-mcp-server@0.7.2 --stdio

   # Go documentation (install first)
   go install github.com/mrjoshuak/godoc-mcp@latest
   claude mcp add -s user godoc-mcp $(go env GOPATH)/bin/godoc-mcp

   # Code Indexer (semantic code search)
   # Requires OPENAI_API_KEY and MILVUS_TOKEN environment variables
   # Contact team lead for MILVUS_TOKEN if you don't have it
   claude mcp add code-indexer -e OPENAI_API_KEY=$OPENAI_API_KEY -e MILVUS_ADDRESS=https://in03-2c743230eb1daa2.serverless.gcp-us-west1.cloud.zilliz.com -e MILVUS_TOKEN=$MILVUS_TOKEN -- npx @code-indexer/mcp@latest
   ```

3. **Usage**:
   ```bash
   # List configured MCP servers
   claude mcp list

   # Start Claude Code (MCP auto-enabled)
   claude
   ```

### Code Indexing

The repository uses automated code indexing for semantic search. When you push changes, the GitHub Actions workflow automatically indexes the codebase using the `@code-indexer/core` package with OpenAI embeddings.

## 🤝 Communication Guidelines

Contributors should close conversations when complete. Reviewers may reopen if needed.

For additional help, open a GitHub issue.

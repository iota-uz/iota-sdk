---
layout: default
title: Installation
parent: Getting Started
nav_order: 1
---

# Installation

This guide covers installing IOTA SDK and setting up your development environment.

## Prerequisites

Ensure these tools are installed before you begin:

- [Go v1.23.2](https://golang.org/doc/install)
- [Air v1.61.5](https://github.com/air-verse/air#Installation) - Hot-reloading
- [Docker v27.2.0](https://docs.docker.com/get-docker/) - Containerization
- [Templ v0.3.857](https://templ.guide/) - Templating
- [TailwindCSS v3.4.13](https://tailwindcss.com/docs/installation) - Styling
- [golangci-lint 1.64.8](https://golangci-lint.run/welcome/install/) - Linting
- [cloudflared](https://developers.cloudflare.com/cloudflare-one/connections/connect-networks/downloads/) - Tunnel (Optional)

## Windows Setup

### 1. Go

Download from [golang.org](https://golang.org/doc/install), install, and verify with:

```cmd
go version
```

### 2. Air

Install Air for hot-reloading:

```cmd
go install github.com/air-verse/air@v1.61.5
```

Add `%USERPROFILE%\go\bin` to your PATH.

### 3. Docker Desktop

Download and install from [docker.com](https://docs.docker.com/desktop/install/windows-install/). Enable WSL 2 during installation.

### 4. Templ

Install the Templ templating engine:

```cmd
go install github.com/a-h/templ/cmd/templ@v0.3.857
```

### 5. TailwindCSS

Install using npm (requires Node.js):

```cmd
npm install -g tailwindcss
```

Or download the standalone executable:

```cmd
curl.exe -sLO https://github.com/tailwindlabs/tailwindcss/releases/v3.4.13/download/tailwindcss-windows-x64.exe
rename tailwindcss-windows-x64.exe tailwindcss.exe
```

### 6. golangci-lint

Install the Go linter:

```cmd
curl.exe -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.ps1 | powershell -Command -
```

### 7. cloudflared

Install Cloudflare Tunnel:

```cmd
winget install --id Cloudflare.cloudflared
```

## macOS/Linux Setup

### 1. Go

Install Go using your package manager or download from [golang.org](https://golang.org/doc/install):

```bash
# Using Homebrew (macOS)
brew install go

# Verify installation
go version
```

### 2. Air

Install Air for hot-reloading:

```bash
go install github.com/air-verse/air@v1.61.5
```

### 3. Docker

Install Docker Desktop from [docker.com](https://docs.docker.com/get-docker/) or use your package manager:

```bash
# Homebrew (macOS)
brew install docker

# Or download Docker Desktop
```

### 4. Templ

Install the Templ templating engine:

```bash
go install github.com/a-h/templ/cmd/templ@v0.3.857
```

### 5. TailwindCSS

Install using npm or Homebrew:

```bash
# Using npm
npm install -g tailwindcss

# Using Homebrew (macOS)
brew install tailwindcss
```

### 6. golangci-lint

Install the Go linter:

```bash
curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin
```

### 7. cloudflared

Install Cloudflare Tunnel:

```bash
# macOS
brew install cloudflare/cloudflare/cloudflared

# Linux
curl -L --output cloudflared.deb https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-linux-amd64.deb
sudo dpkg -i cloudflared.deb
```

## Development Setup

### 1. Clone the Repository

```bash
git clone https://github.com/iota-uz/iota-sdk.git
cd iota-sdk
```

### 2. Create Environment File

Copy the example environment file:

```bash
# macOS/Linux
cp .env.example .env

# Windows
copy .env.example .env
```

Edit `.env` with your configuration as needed.

### 3. Install Dependencies

```bash
make deps
```

**Note for Windows**: If `make` is unavailable, install via [GnuWin32](http://gnuwin32.sourceforge.net/packages/make.htm) or use [Git Bash](https://gitforwindows.org/).

### 4. Run PostgreSQL

Ensure Docker is running, then start PostgreSQL:

```bash
make db local
```

### 5. Apply Migrations

```bash
make db migrate up && make db seed
```

This creates the database schema and loads initial data.

### 6. Run TailwindCSS in Watch Mode

Open a new terminal and run:

```bash
make css watch
```

### 7. Start Development Server

In another terminal, start the development server with hot-reloading:

```bash
air
```

### 8. Access the Application

- **Web Application**: [http://localhost:8080](http://localhost:8080)
- **Default Credentials**:
  - Email: `test@gmail.com`
  - Password: `TestPass123!`
- **GraphQL**: [http://localhost:3200/query](http://localhost:3200/query)

## Development Tools Manager

For convenience, use the DevHub TUI tool to manage all development services:

```bash
make devtools
```

The DevHub allows you to:

- Start/stop the development server
- Toggle template file watching (`templ generate --watch`)
- Toggle CSS file watching (TailwindCSS watch mode)
- Manage local PostgreSQL database
- Start/stop Cloudflare tunnel

Navigate with arrow keys, toggle services with Space/Enter, and quit with 'q'.

## Running Tests

### Unit and Integration Tests

Run the full test suite:

```bash
make test
```

Run tests with coverage:

```bash
make test coverage
```

Run a specific test:

```bash
go test -v ./path/to/package -run TestSpecificName
```

### E2E Testing with Playwright

The project includes Playwright end-to-end tests that run against a separate database.

#### Setup

Create and prepare the E2E database:

```bash
make e2e setup
```

#### Start E2E Server

In a separate terminal, start the server connected to the E2E database:

```bash
make e2e server
```

The server runs on port 3201 (development server on 3200).

#### Run Tests

```bash
# Run all tests
make e2e test

# Interactive mode
make e2e run

# Direct npm command
cd e2e/
pnpm run test:headed
```

#### Available E2E Commands

- `make e2e setup` - Create E2E database, run migrations, and seed
- `make e2e reset` - Drop and recreate E2E database with fresh data
- `make e2e server` - Start server on port 3201 (E2E database)
- `make e2e test` - Run all E2E tests
- `make e2e run` - Open Playwright UI mode
- `make e2e clean` - Drop E2E database

**Important Notes**:
- E2E tests use a separate database (`iota_erp_e2e`) from development (`iota_erp`)
- E2E server runs on port 3201, development server runs on port 3200
- Configuration: `/e2e/.env.e2e`, `/e2e/playwright.config.ts`

## Code Documentation

Generate code documentation:

```bash
# For entire project
make docs

# With specific options
go run cmd/command/main.go doc -dir [directory] -out [output file] [-recursive] [-exclude "dir1,dir2"]
```

**Options**:
- `-dir`: Target directory (default: current directory)
- `-out`: Output file path (default: DOCUMENTATION.md)
- `-recursive`: Process subdirectories
- `-exclude`: Skip specified directories (comma-separated)

## Troubleshooting

### Windows Setup Issues

#### Make commands fail

Install via [GnuWin32](http://gnuwin32.sourceforge.net/packages/make.htm) or use [Git Bash](https://gitforwindows.org/).

#### Docker issues

- Ensure WSL 2 is properly configured
- Run `wsl --update` as administrator
- Restart Docker Desktop

#### Air hot-reloading problems

- Verify Air is in your PATH
- Check for `.air.toml` configuration
- Try `air init` to create new configuration

#### PostgreSQL connection issues

- Ensure Docker is running: `docker ps`
- Verify database credentials in `.env`

### Linting Issues

Do not run:
```bash
golangci-lint run --fix
```

It may break the code. If you encounter linting errors, try:

```bash
go mod tidy
```

### Cloudflare Tunnel

To intercept incoming traffic from third-party systems (like payment gateways), use a Cloudflare Tunnel:

```bash
make tunnel
```

## Next Steps

- Explore the module documentation
- Review the [project guides](..) for development patterns
- Check the [Release Guide](./release.md) for release procedures

---

For help, see the [FAQ](../FAQ.md) or open a [GitHub issue](https://github.com/iota-uz/iota-sdk/issues).

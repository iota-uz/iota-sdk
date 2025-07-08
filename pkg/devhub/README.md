# DevHub

DevHub is a powerful, terminal-based tool designed to streamline the management of multiple development services. It provides a dynamic and interactive Terminal User Interface (TUI) for starting, stopping, monitoring, and viewing logs for all your project's services from a single place.

Built with Go and the `bubbletea` framework, DevHub simplifies complex development workflows by managing service dependencies, performing health checks, and providing real-time resource monitoring.

## Key Features

- **Centralized Service Management**: Control all your development services (backend APIs, frontend servers, databases, etc.) from a single, unified TUI.
- **YAML-Based Configuration**: Define all your services, commands, ports, and dependencies in a simple and intuitive `devhub.yml` file.
- **Dependency Resolution**: Automatically determines the correct start-up order for services based on `needs` relationships, ensuring that dependencies are met.
- **Advanced Health Checks**: Configure TCP, HTTP, or command-based health checks to ensure your services are not just running, but are fully operational.
- **Cross-Platform Support**: Define OS-specific commands for macOS (`darwin`), Linux, and Windows to handle differences in development environments seamlessly.
- **Real-time Monitoring**: Keep an eye on the status, port, PID, uptime, CPU usage, and memory consumption of each service.
- **Interactive Log Viewer**: Dive into the logs for any service with features like auto-scrolling, search, and easy navigation between services.
- **Interactive TUI**: A user-friendly interface that allows you to manage your services with simple keyboard commands.

## Configuration (`devhub.yml`)

DevHub uses a clean, intuitive YAML format where services are defined as top-level keys. Here's a complete example:

```yaml
# Service name as the key
postgres:
  desc: Database container
  port: 5432
  run: docker compose -f compose.dev.yml up db
  health:
    tcp: 5432      # Simple TCP port check
    wait: 20s      # Wait 20s before first health check

server:
  desc: Hot-reload Go app
  port: 3200
  run: air -c .air.toml
  needs: [postgres]  # Dependencies
  health:
    http: http://localhost:3200/health  # HTTP endpoint check
    wait: 10s
  os:
    windows: air.exe -c .air.toml      # Windows-specific command

templ:
  desc: Auto-compile templates
  run: templ generate --watch
  os:
    windows: templ.exe generate --watch

css:
  desc: Build Tailwind CSS
  run: tailwindcss -c tailwind.config.js -i modules/core/presentation/assets/css/main.css -o modules/core/presentation/assets/css/main.min.css --minify --watch
  os:
    windows: tailwindcss.exe -c tailwind.config.js -i modules/core/presentation/assets/css/main.css -o modules/core/presentation/assets/css/main.min.css --minify --watch

tunnel:
  desc: Cloudflare tunnel
  run: cloudflared tunnel --url http://localhost:3200 --loglevel debug
  needs: [server]
  os:
    windows: cloudflared.exe tunnel --url http://localhost:3200 --loglevel debug
```

### Configuration Options

#### Service Fields

- **`desc`**: Brief description of the service (shown in TUI)
- **`port`**: Port number the service listens on (optional)
- **`run`**: Command to execute (required)
- **`needs`**: Array of service names this service depends on (optional)
- **`health`**: Health check configuration (optional)
- **`os`**: OS-specific command overrides (optional)

#### Health Check Options

Health checks can be configured in three ways:

**TCP Port Check:**
```yaml
health:
  tcp: 5432        # Port to check
  wait: 20s        # Grace period before first check
  interval: 5s     # Check interval (default: 5s)
  timeout: 3s      # Check timeout (default: 3s)
  retries: 3       # Retry count (default: 3)
```

**HTTP Endpoint Check:**
```yaml
health:
  http: http://localhost:3200/health
  wait: 10s
  interval: 5s
  timeout: 3s
```

**Command Check:**
```yaml
health:
  cmd: pg_isready -h localhost -p 5432
  wait: 5s
  timeout: 5s
```

#### OS-Specific Commands

Define platform-specific command variations:

```yaml
os:
  darwin: ./bin/my-tool-macos      # macOS
  linux: ./bin/my-tool-linux        # Linux  
  windows: my-tool.exe              # Windows
```

## Installation

### Using go install

Install DevHub using Go's built-in install command:

```bash
go install github.com/iota-uz/iota-sdk/cmd/devhub@latest
```

This will install the `devhub` binary to your `$GOPATH/bin` directory. Make sure `$GOPATH/bin` is in your `PATH`.

### From Source

```bash
# Clone the repository
git clone https://github.com/iota-uz/iota-sdk.git
cd iota-sdk

# Build the binary
make devhub

# Install locally
make devhub-install
```

## Usage

```bash
# Run with default config file (devhub.yml in current directory)
devhub

# Run with custom config file
devhub --config /path/to/devhub.yml

# Show version information
devhub --version

# Set log level
devhub --log-level debug
```

## How It Works

DevHub is composed of several core components that work together to provide a seamless experience:

- **Service Manager**: The heart of DevHub, responsible for loading the configuration, resolving dependencies, and managing the lifecycle of each service.
- **TUI (Terminal User Interface)**: The `bubbletea`-based frontend that displays service information and captures user input. It has two main views: a service list and a log viewer.
- **Health Monitor**: A component that runs health checks for each service at a configured interval to ensure they are healthy.
- **Resource Monitor**: A utility that periodically fetches CPU and memory usage for each running service and its child processes.
- **Dependency Resolver**: A tool that performs a topological sort on the services to determine the correct start-up order.

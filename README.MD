# IOTA SDK

## A Modular and Configurable ERP Solution

**IOTA SDK** is an open-source, modular, and feature-complete **ERP system** designed to adapt to multiple economic sectors, including **finance**, **manufacturing**, and **warehouse management**. Inspired by leading platforms like Odoo, IOTA SDK provides an intuitive framework to accelerate enterprise operations and unlock efficiency with powerful tools and integrations.

---

## 🚀 Key Features

- **Configurable Architecture**: Fully modular components to customize workflows.
- **Industry-Specific Modules**: Finance, manufacturing, warehouse management, and more.
- **Modern Development Practices**: Built with **Go** for performance and scalability.
- **GraphQL API**: Simplifies querying and data management.
- **Open Source**: Fully extensible and transparent.

---

## 📚 Overview

### Architecture  
Visualize the system's architecture using this [interactive diagram](https://app.excalidraw.com/s/3x4l1qRpK2w/ADYN81ksZsd).

### Database Schema  
Explore the modular design through the [database schema](https://dbdiagram.io/d/ERP-SDK-65fd8cb4ae072629ceb7f50e).

### Glossary  
**UHR** - Unified Hourly Rate (a common standard in IOTA SDK for cost tracking and resource management).

---

## ⚙️ Prerequisites

Before starting, ensure the following tools are installed on your system:

- [Go](https://golang.org/doc/install)
- [Air](https://github.com/cosmtrek/air#Installation) for hot-reloading
- [Docker](https://docs.docker.com/get-docker/) for containerized services
- [SQL-Migrate](https://github.com/rubenv/sql-migrate) for database migrations
- [Templ](https://templ.guide/) for templating
- [TailwindCSS](https://tailwindcss.com/docs/installation) for styling
- [golangci-lint](https://golangci-lint.run/welcome/install/) for linting

---

## 🛠️ Development Setup

1. **Clone the repository**:
   ```bash
   git clone https://github.com/iota-agency/iota-sdk.git
   ```

2. **Install dependencies**:
   ```bash
   go get -u -d ./...
   ```

3. **Set up TailwindCSS globally**:
   ```bash
   pnpm install -g tailwindcss
   # or
   npm install -g tailwindcss
   ```

4. **Run PostgreSQL**:
   ```bash
   docker compose -f docker-compose.dev.yml up -d
   ```

5. **Apply database migrations**:
   ```bash
   sql-migrate up
   ```

6. **Start hot-reloading with Air**:
   ```bash
   air
   ```

7. **Enable Templ in watch mode**:
   ```bash
   templ generate -watch
   ```

8. **Compile TailwindCSS**:
   ```bash
   tailwindcss -c tailwind.config.js -i internal/presentation/assets/css/main.css -o internal/presentation/assets/css/main.min.css --minify --watch
   ```

9. **Access the GraphQL Schema**:  
   Open Postman and connect to:  
   [http://localhost:3200/query](http://localhost:3200/query)

---

## 🔍 Example Queries

### Fetch Employees and Positions  
Use the following GraphQL query to retrieve users and their roles:

```graphql
query Users {
    users(limit: 10, offset: 0) {
        total
        data {
            id
            email
            firstName
            lastName
            avatar {
                id
                size
            }
        }
    }
}
```

---

## 📖 Reference Commands

### Core Development
- **Run Linter**:  
  ```bash
  golangci-lint run
  ```
- **Regenerate from GraphQL schema**:  
  ```bash
  go generate ./...
  ```

### Database Management
- **Apply migrations**:  
  ```bash
  sql-migrate up
  ```
- **Rollback migrations**:  
  ```bash
  sql-migrate down
  ```
- **Redo migrations**:  
  ```bash
  sql-migrate redo
  ```
- **Create a new migration**:  
  ```bash
  sql-migrate new <migration-name>
  ```

### Testing
- **Run tests**:  
  ```bash
  go test -v ./...
  ```
- **Test with coverage**:  
  ```bash
  go test -coverprofile coverage/coverage.out ./... 
  go tool cover -html=coverage/coverage.out
  ```

### Server Operations
- **Run the server**:  
  ```bash
  go run cmd/server/main.go
  ```

### Templ Management
- **Format Templ files**:  
  ```bash
  templ fmt .
  ```

---

## 🎯 Conventions

- [Naming Guidelines](CONVENTIONS.MD#naming)
- [Code Style](CONVENTIONS.MD#code-style)

---

## 🌟 Join Us

We welcome contributions to improve and expand IOTA SDK! Check out our [contributing guide](CONTRIBUTING.md) to get started.

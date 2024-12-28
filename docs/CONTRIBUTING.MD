## ⚙️ Prerequisites

Before starting, ensure the following tools are installed on your system:

- [Go](https://golang.org/doc/install)
- [Air](https://github.com/cosmtrek/air#Installation) for hot-reloading
- [Docker](https://docs.docker.com/get-docker/) for containerized services
- [Templ](https://templ.guide/) for templating
- [TailwindCSS](https://tailwindcss.com/docs/installation) for styling
- [golangci-lint](https://golangci-lint.run/welcome/install/) for linting

---

## 🛠️ Development Setup

1. **Clone the repository**:
   ```bash
   git clone https://github.com/iota-agency/iota-sdk.git
   ```

2. **Run code gen**:
   ```bash
   make generate
   ```

3. **Make env file**:
   ```bash
   cp .env.example .env
   ```

4. **Install dependencies**:
   ```bash
   make deps
   ```

5. **Set up TailwindCSS**:
   ```bash
   pnpm install -g tailwindcss
   # or
   npm install -g tailwindcss
   # or
   curl -sLO https://github.com/tailwindlabs/tailwindcss/releases/v3.4.15/download/tailwindcss-linux-x64
   chmod +x tailwindcss-linux-x64
   ```

6. **Run PostgreSQL**:
   ```bash
   make localdb
   ```

7. **Apply database migrations**:
   ```bash
   make migrate-up && make seed
   ```

8. **Run TailwindCSS in watch mode**:
   ```bash
   # in a separate terminal
   make css-watch
   ```

9. **Start dev server with Air hot reloading**:
   ```bash
   air
   ```

10. **Access the web app**:

    Open your browser and navigate to:  
    [http://localhost:8080](http://localhost:8080)

    Use the following credentials to log in:

    **Email:**
    ```text
    test@gmail.com
    ```
    **Password:**
    ```text
    TestPass123!
    ```

11. **Access the GraphQL Schema**:  
    Open Postman and connect to:  
    [http://localhost:3200/query](http://localhost:3200/query)

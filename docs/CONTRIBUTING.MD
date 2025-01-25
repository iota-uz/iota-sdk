## ⚙️ Prerequisites

Before starting, ensure the following tools are installed on your system:

- [Go v1.23.2](https://golang.org/doc/install)
- [Air v1.61.5](https://github.com/cosmtrek/air#Installation) for hot-reloading
- [Docker v27.2.0](https://docs.docker.com/get-docker/) for containerized services
- [Templ v0.3.819](https://templ.guide/) for templating
- [TailwindCSS v3.4.13](https://tailwindcss.com/docs/installation) for styling
- [golangci-lint v1.61.0](https://golangci-lint.run/welcome/install/) for linting

---

## 🛠️ Development Setup

1. **Clone the repository**:
   ```bash
   git clone https://github.com/iota-uz/iota-sdk.git
   ```

2. **Make env file**:
   ```bash
   cp .env.example .env
   ```

3. **Install dependencies**:
   ```bash
   make deps
   ```

4. **Set up TailwindCSS**:
   ```bash
   pnpm install -g tailwindcss
   # or
   npm install -g tailwindcss
   # or
   curl -sLO https://github.com/tailwindlabs/tailwindcss/releases/v3.4.15/download/tailwindcss-linux-x64
   chmod +x tailwindcss-linux-x64
   ```

5. **Run PostgreSQL**:
   ```bash
   make localdb
   ```

6. **Apply database migrations**:
   ```bash
   make migrate-up && make seed
   ```

7. **Run TailwindCSS in watch mode**:
   ```bash
   # in a separate terminal
   make css-watch
   ```

8. **Start dev server with Air hot reloading**:
   ```bash
   air
   ```

9. **Access the web app**:

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

10. **Access the GraphQL Schema**:  
    Open Postman and connect to:  
    [http://localhost:3200/query](http://localhost:3200/query)

# Contributing Guide

## ⚙️ Prerequisites

Before starting, ensure the following tools are installed on your system:

- [Go v1.23.2](https://golang.org/doc/install)
- [Air v1.61.5](https://github.com/cosmtrek/air#Installation) for hot-reloading
- [Docker v27.2.0](https://docs.docker.com/get-docker/) for containerized services
- [Templ v0.3.819](https://templ.guide/) for templating
- [TailwindCSS v3.4.13](https://tailwindcss.com/docs/installation) for styling
- [golangci-lint v1.61.0](https://golangci-lint.run/welcome/install/) for linting

### Windows-Specific Setup

#### Required Tools Installation

1. **Install Go**:
   - Download the Windows MSI installer from [golang.org](https://golang.org/doc/install)
   - Run the installer and follow the prompts
   - Verify installation by opening Command Prompt and running:
     ```cmd
     go version
     ```

2. **Install Air**:
   ```cmd
   go install github.com/cosmtrek/air@v1.61.5
   ```
   Add `%USERPROFILE%\go\bin` to your system's PATH if not already included.

3. **Install Docker Desktop**:
   - Download Docker Desktop for Windows from [docker.com](https://docs.docker.com/desktop/install/windows-install/)
   - Enable WSL 2 when prompted during installation
   - Start Docker Desktop after installation

4. **Install Templ**:
   ```cmd
   go install github.com/a-h/templ/cmd/templ@v0.3.819
   ```

5. **Install TailwindCSS**:
   Using npm (requires Node.js):
   ```cmd
   npm install -g tailwindcss
   ```
   Or download the standalone executable:
   ```cmd
   curl.exe -sLO https://github.com/tailwindlabs/tailwindcss/releases/v3.4.15/download/tailwindcss-windows-x64.exe
   rename tailwindcss-windows-x64.exe tailwindcss.exe
   ```

6. **Install golangci-lint**:
   ```cmd
   curl.exe -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.ps1 | powershell -Command -
   ```

---

## 🛠️ Development Setup

1. **Clone the repository**:
   ```cmd
   git clone https://github.com/iota-uz/iota-sdk.git
   cd iota-sdk
   ```

2. **Make env file**:
   ```cmd
   copy .env.example .env
   ```

3. **Install dependencies**:
   ```cmd
   make deps
   ```
   Note: If `make` is not available, install it via [GnuWin32](http://gnuwin32.sourceforge.net/packages/make.htm) or use [Git Bash](https://gitforwindows.org/).

4. **Set up TailwindCSS**:
   ```cmd
   npm install -g tailwindcss
   ```
   Or if using the standalone executable:
   ```cmd
   mkdir %USERPROFILE%\bin
   move tailwindcss.exe %USERPROFILE%\bin
   ```
   Add `%USERPROFILE%\bin` to your system's PATH.

5. **Run PostgreSQL**:
   ```cmd
   make localdb
   ```
   Note: Ensure Docker Desktop is running before executing this command.

6. **Apply database migrations**:
   ```cmd
   make migrate-up && make seed
   ```

7. **Run TailwindCSS in watch mode**:
   Open a new Command Prompt window and run:
   ```cmd
   make css-watch
   ```

8. **Start dev server with Air hot reloading**:
   ```cmd
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

### Troubleshooting Windows Setup

1. **If `make` commands fail**:
   - Install Make for Windows via [GnuWin32](http://gnuwin32.sourceforge.net/packages/make.htm)
   - Add the installation directory to your system's PATH
   - Alternatively, use Git Bash which includes Make

2. **If Docker fails to start**:
   - Ensure WSL 2 is properly installed and configured
   - Run `wsl --update` in Command Prompt as administrator
   - Restart Docker Desktop

3. **If Air hot-reloading isn't working**:
   - Verify Air is in your PATH
   - Check if the `.air.toml` configuration file exists
   - Try running `air init` to create a new configuration

4. **PostgreSQL connection issues**:
   - Ensure Docker is running
   - Check if the PostgreSQL container is running:
     ```cmd
     docker ps
     ```
   - Verify the database credentials in your `.env` file

### Communication rules

1. The contributer should close the conversation if he consiters it's done. t can be reopened by the reviewer if needed. 

For additional help, please check our [FAQ](./FAQ.md) or open an issue on GitHub.

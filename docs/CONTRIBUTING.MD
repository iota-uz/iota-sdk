## ⚙️ Prerequisites

Before starting, ensure the following tools are installed on your system:

- [Docker v27.2.0](https://docs.docker.com/get-docker/) for containerized services

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

3. **Run the project using Docker**:
   ```bash
   make dev-docker
   ```

4. **Apply database migrations and seed data**:
   ```bash
   make migrate-up && make seed
   ```

5. **Access the web app**:

   Open your browser and navigate to:  
   [http://localhost:8090](http://localhost:8090)

   Use the following credentials to log in:

   **Email:**
   ```text
   test@gmail.com
   ```
   **Password:**
   ```text
   TestPass123!
   ```

6. **Access the GraphQL Schema**:  
    Open Postman and connect to:  
    [http://localhost:3200/query](http://localhost:3200/query)

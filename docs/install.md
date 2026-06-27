# Installation Guide

The Migration Safety Engine (MSE) runs as a control API and state-machine runner, alongside a Postgres instance for configuration/state tracking, and optional Prometheus and Grafana instances for observability.

---

## 1. Local Quick Start (Docker Compose)

The easiest way to run the entire stack is using the provided Docker Compose configuration, which launches:
- **Postgres** (acting as both the engine state store on port `5499` and the migration target)
- **Prometheus** (to monitor database health and scrape engine metrics on port `9093`)
- **Grafana** (pre-loaded with the MSE migration dashboard on port `3004`)

### Prerequisites
- **Go 1.24+**
- **Docker** and **Docker Compose**

### Step-by-Step Run:

1. Clone and compile the stack:
   ```bash
   git clone https://github.com/iamyadavvikas/migration-safety-engine.git
   cd migration-safety-engine
   ```

2. Start the services:
   ```bash
   make up
   ```
   This spins up the containers and blocks until Postgres is healthy and accepting connections.

3. Install the database schemas:
   ```bash
   make migrate
   ```
   This loads the engine control schema (`0001_state.sql`) and sets up a mock production table with 50,000 rows (`0002_demo_target.sql`) for demos.

4. Run the Engine:
   ```bash
   make run
   ```
   This starts the Go engine, which binds to `:8080`. It serves both the control API `/plans`, `/migrations` and the embedded UI dashboard at `http://localhost:8080`.

---

## 2. Running Frontend and Backend Separately for Development

If you want to edit the React dashboard with hot-reloading:

1. **Start the backend**:
   ```bash
   make run
   ```
   The engine starts on `http://localhost:8080`.

2. **Start Vite development server**:
   ```bash
   cd frontend
   npm install
   npm run dev
   ```
   Vite runs the frontend on `http://localhost:5173` or `5174` and proxies all API endpoints (like `/migrations`, `/plans`) directly to the backend engine on `:8080`.

---

## 3. Environment Variables

You can customize the Go engine binary configuration using the following environment variables:

| Variable | Description | Default |
|----------|-------------|---------|
| `DB_DSN` | Postgres connection string for the engine's state storage database. | `postgres://mse:mse@localhost:5499/mse?sslmode=disable` |
| `TARGET_DSN` | Connection string for the target application database being migrated. | Defaults to `DB_DSN` |
| `ENGINE_ADDR` | Listen address for the API server and embedded UI. | `:8080` |
| `MSE_TEST_DSN` | Connection string to run integration tests against a real database. | (If empty, integration tests are skipped) |

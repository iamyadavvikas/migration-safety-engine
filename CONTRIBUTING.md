# Contributing to Migration Safety Engine (MSE)

Thank you for your interest in contributing to the Migration Safety Engine! We welcome issues, suggestions, and pull requests to help make database schema migrations safer and more observable.

## Code of Conduct

Please be respectful, professional, and welcoming to all contributors.

## How to Contribute

### 1. Reporting Bugs & Feature Requests

Before opening a new issue, search the existing issues to see if it has already been reported. When reporting:
- Provide a clear, descriptive title.
- Explain the steps to reproduce the issue.
- Describe the expected behavior and what actually happened.
- Include logs, system configuration details, and migration plan files if relevant.

### 2. Local Development Setup

To work on MSE locally, you need the following prerequisites installed on your system:
- **Go 1.24+**
- **Docker & Docker Compose**
- **Node.js & npm** (for the frontend React dashboard)

#### Setting up the repository:

1. Clone your fork of the repository:
   ```bash
   git clone https://github.com/<your-username>/migration-safety-engine.git
   cd migration-safety-engine
   ```

2. Start the Postgres and monitoring stacks:
   ```bash
   make up
   ```

3. Initialize the database schema for the engine control and demo tables:
   ```bash
   make migrate
   ```

4. Build and run the engine locally:
   ```bash
   make build
   make run
   ```

### 3. Making Code Changes

- **Golang**: Keep code clean, run standard formatting (`make fmt`), and perform static analyses (`make vet` or `make lint`).
- **Frontend (Vite/React)**: Follow the existing component pattern. To run the frontend in development mode with hot-reloading:
  ```bash
  make frontend-install
  make frontend-dev
  ```
  Vite will run the UI locally (typically at `http://localhost:5173` or `5174`), proxying backend API requests to the engine on `:8080`.

### 4. Running Tests

Always make sure existing tests pass before submitting changes.
- To run unit tests:
  ```bash
  go test ./...
  ```
- To run integration tests against a real database instance:
  ```bash
  # Ensure the docker compose stack is running
  make up
  MSE_TEST_DSN="postgres://mse:mse@localhost:5499/mse?sslmode=disable" go test ./internal/statemachine/...
  ```

### 5. Submitting Pull Requests

1. Create a descriptive branch for your changes:
   ```bash
   git checkout -b feature/my-cool-feature
   ```
2. Commit your changes with clear, descriptive commit messages.
3. Push to your fork and submit a Pull Request to the `main` branch of the official repository.
4. Ensure your PR passes all automated build and test checks.

# Migration Safety Engine — Makefile
# Phase 1: durable, resumable state machine on Postgres with Prometheus metrics.

DB_DSN ?= postgres://mse:mse@localhost:5499/mse?sslmode=disable
ENGINE_ADDR ?= :8080

.PHONY: up down migrate run demo demo-rollback reset-demo test lint tidy build fmt vet load load-under-backfill frontend frontend-dev frontend-build

up: ## Start postgres + prometheus + grafana
	docker compose up -d
	@echo "waiting for postgres..."
	@until docker compose exec -T postgres pg_isready -U mse -d mse >/dev/null 2>&1; do sleep 1; done
	@echo "postgres ready"

down: ## Stop and remove the stack
	docker compose down

migrate: ## Apply the engine state schema + the demo target table
	docker compose exec -T postgres psql -U mse -d mse < migrations/0001_state.sql
	docker compose exec -T postgres psql -U mse -d mse < migrations/0002_demo_target.sql
	@echo "schema applied"

run: ## Run the engine (control API + state-machine runner)
	DB_DSN="$(DB_DSN)" ENGINE_ADDR="$(ENGINE_ADDR)" go run ./cmd/engine

reset-demo: ## Return catalog_product to its pre-migration shape (re-add legacy col, drop new col)
	docker compose exec -T postgres psql -U mse -d mse -c \
		"ALTER TABLE catalog_product ADD COLUMN IF NOT EXISTS legacy_shipping text; ALTER TABLE catalog_product DROP COLUMN IF EXISTS shipping_class;"
	@echo "demo table reset"

demo: ## Reset the demo table and drive the sample plan end-to-end
	DB_DSN="$(DB_DSN)" ENGINE_ADDR="$(ENGINE_ADDR)" ./scripts/demo.sh

demo-rollback: ## Drive the chaos plan and show SLO-gated auto-rollback
	DB_DSN="$(DB_DSN)" ENGINE_ADDR="$(ENGINE_ADDR)" ./scripts/demo_rollback.sh

load: ## Generate concurrent write load against the target (WORKERS, DURATION overridable)
	DB_DSN="$(DB_DSN)" go run ./cmd/loadgen -workers $(or $(WORKERS),16) -duration $(or $(DURATION),20s)

load-under-backfill: ## Run load WHILE a migration backfills, proving writes stay within SLO
	DB_DSN="$(DB_DSN)" ENGINE_ADDR="$(ENGINE_ADDR)" ./scripts/load_under_backfill.sh

test: ## Run unit + integration tests (integration needs Docker)
	go test ./...

build: frontend-build ## Build both binaries
	go build -o bin/engine ./cmd/engine
	go build -o bin/mgctl ./cmd/mgctl
	go build -o bin/loadgen ./cmd/loadgen

tidy: ## Sync go.mod/go.sum
	go mod tidy

fmt: ## Format
	go fmt ./...

vet: ## Static analysis
	go vet ./...

lint: vet ## Alias for vet (add golangci-lint later)

frontend-dev: ## Start the Vite dev server for the React dashboard
	cd frontend && npm run dev

frontend-build: ## Build the React dashboard for production
	@if [ ! -d "frontend/node_modules" ]; then \
		echo "node_modules missing. Installing frontend dependencies..."; \
		cd frontend && npm install; \
	fi
	cd frontend && npm run build

frontend-install: ## Install frontend dependencies
	cd frontend && npm install

run-all: up migrate frontend-build ## Full stack: infra + schema + built frontend
	@echo "=== Stack ready ==="
	@echo "Engine: http://localhost$(ENGINE_ADDR)"
	@echo "Grafana: http://localhost:3004"
	@echo "Prometheus: http://localhost:9093"

.PHONY: deploy deploy-up deploy-down tunnel-up tunnel-down

deploy-up: ## Start the full stack for production (includes engine container)
	docker compose up -d --build
	@echo "Waiting for engine..."
	@until curl -sf http://localhost:8080/healthz >/dev/null 2>&1; do sleep 2; done
	@echo "Engine is up: http://localhost:8080"
	@echo "Run 'make tunnel-up' to expose via Cloudflare"

deploy-down: ## Stop the production stack
	docker compose down

deploy: deploy-up ## Build, start, and expose (alias)

tunnel-up: ## Expose engine via Cloudflare Tunnel
	@echo "Starting Cloudflare Tunnel..."
	@if [ -n "$(TUNNEL_TOKEN)" ]; then \
		docker compose -f docker-compose.yml -f docker-compose.cloudflare.yml up -d cloudflared; \
		echo "Cloudflare Tunnel started with token"; \
	elif command -v cloudflared >/dev/null 2>&1; then \
		./scripts/tunnel.sh cloudflare start; \
	else \
		echo "No TUNNEL_TOKEN set and cloudflared not found. Install: brew install cloudflared"; \
		echo "Or use Serveo fallback: ./scripts/tunnel.sh serveo start"; \
		exit 1; \
	fi

tunnel-down: ## Stop Cloudflare Tunnel
	@if docker compose -f docker-compose.yml -f docker-compose.cloudflare.yml ps cloudflared >/dev/null 2>&1; then \
		docker compose -f docker-compose.yml -f docker-compose.cloudflare.yml rm -sf cloudflared; \
	else \
		./scripts/tunnel.sh cloudflare stop 2>/dev/null || true; \
		./scripts/tunnel.sh serveo stop 2>/dev/null || true; \
	fi
	@echo "Tunnel stopped"

# Migration Safety Engine — Makefile
# Phase 1: durable, resumable state machine on Postgres with Prometheus metrics.

DB_DSN ?= postgres://mse:mse@localhost:5499/mse?sslmode=disable
ENGINE_ADDR ?= :8080

.PHONY: up down migrate run demo demo-rollback reset-demo test lint tidy build fmt vet load load-under-backfill

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

build: ## Build both binaries
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

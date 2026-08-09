.PHONY: run run-worker migrate docker-up docker-down test tidy build

# Load local .env into the process environment when present (KEY=value lines).
define load_env
	set -a; [ -f .env ] && . ./.env; set +a;
endef

# Meta: CRUD + TS→JS compile on save (default :8080)
run:
	@$(load_env) go run ./cmd/meta

# Worker: fastschema/qjs runtime (default :9090)
run-worker:
	@$(load_env) go run ./cmd/worker

build:
	go build -o bin/lowcode-faas-meta ./cmd/meta
	go build -o bin/lowcode-faas-worker ./cmd/worker

migrate:
	@echo "Auto-migrate runs on meta start. SQL reference: migrations/000001_init.up.sql"
	@$(load_env) psql "$${LOWCODE_FAAS_POSTGRES_DSN:-postgres://faas:faas@localhost:5433/lowcode_faas?sslmode=disable}" -f migrations/000001_init.up.sql

docker-up:
	docker compose up -d

docker-down:
	docker compose down

test:
	go test ./...

tidy:
	go mod tidy

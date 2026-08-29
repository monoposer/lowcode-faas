.PHONY: run run-worker migrate docker-up docker-up-meta docker-down test tidy build build-example-worker

# Load local .env into the process environment when present (KEY=value lines).
define load_env
	set -a; [ -f .env ] && . ./.env; set +a;
endef

# Meta: CRUD + TS→JS compile on save (default :8080)
run:
	@$(load_env) go run ./cmd/meta

# Example worker: embed SDK with custom host (default :9090)
run-worker:
	@$(load_env) go run ./examples/worker-embed

build:
	go build -o bin/lowcode-faas-meta ./cmd/meta

build-example-worker:
	go build -o bin/lowcode-faas-worker-embed ./examples/worker-embed

migrate:
	@echo "Auto-migrate runs on meta start. SQL reference: migrations/000001_init.up.sql"
	@$(load_env) psql "$${LOWCODE_FAAS_POSTGRES_DSN:-postgres://faas:faas@localhost:5433/lowcode?sslmode=disable&search_path=faas}" -f migrations/000001_init.up.sql

docker-up:
	docker compose up -d

# Infra + Meta image (worker stays an SDK / example process)
docker-up-meta:
	docker compose --profile meta up -d --build

docker-down:
	docker compose --profile meta down

test:
	go test ./...

tidy:
	go mod tidy

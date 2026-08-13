.PHONY: infra-up infra-down up down logs logs-book logs-inventory logs-notification run-book run-inventory run-notification test test-integration seed

# ── Day 1: Start infra only (LocalStack SNS/SQS + 3x Postgres) ───────────────
infra-up:
	docker compose up -d localstack postgres-book postgres-inventory postgres-notification

infra-down:
	docker compose down -v

# ── Day 2+: Full stack ────────────────────────────────────────────────────────
up:
	docker compose up --build -d

down:
	docker compose down -v

logs:
	docker compose logs -f

logs-book:
	docker compose logs -f book-service

logs-inventory:
	docker compose logs -f inventory-service

logs-notification:
	docker compose logs -f notification-service

# ── Local Development ─────────────────────────────────────────────────────────
run-book:
	cd services/book-service && go run .

run-inventory:
	cd services/inventory-service && go run .

run-notification:
	cd services/notification-service && go run .

# ── Testing ───────────────────────────────────────────────────────────────────
test:
	cd services/book-service && go test -v ./...
	cd services/inventory-service && go test -v ./...
	cd services/notification-service && go test -v ./...

test-integration:
	cd services/book-service && go test -v -tags=integration ./...
	cd services/inventory-service && go test -v -tags=integration ./...
	cd services/notification-service && go test -v -tags=integration ./...

# ── Seed demo data ─────────────────────────────────────────────────────────────
seed:
	go run scripts/seed.go

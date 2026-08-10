.PHONY: infra-up infra-down rabbit-setup up down logs

# ── Day 1: Start infra only ───────────────────────────────────────────────────
infra-up:
	docker compose up -d rabbitmq postgres-book postgres-inventory postgres-notification

infra-down:
	docker compose down -v

# Declare RabbitMQ exchanges, queues, and bindings.
# Run this once after `make infra-up` and RabbitMQ is healthy.
rabbit-setup:
	sh rabbitmq/setup.sh

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

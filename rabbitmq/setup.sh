#!/bin/sh
# RabbitMQ Topology Setup
# Run this after RabbitMQ is healthy via: make rabbit-setup
#
# Topology:
#   Exchange: library.events  (topic)   — main event bus
#   Exchange: library.dlx     (direct)  — dead letter exchange
#   Queues:
#     library.dlq                       — dead letter queue (manual inspection)
#     inventory-svc.book-events         — book.* → inventory service
#     notification-svc.book-events      — book.* → notification service
#     notification-svc.inventory-events — inventory.* → notification service

BASE_URL="http://localhost:15672/api"
AUTH="guest:guest"

echo "Setting up RabbitMQ topology..."

# ── Exchanges ────────────────────────────────────────────────────────────────

echo "Creating exchanges..."

curl -sf -u "$AUTH" -X PUT "$BASE_URL/exchanges/%2F/library.dlx" \
  -H "Content-Type: application/json" \
  -d '{"type":"direct","durable":true}' \
  && echo " ✓ library.dlx" || echo " ✗ library.dlx FAILED"

curl -sf -u "$AUTH" -X PUT "$BASE_URL/exchanges/%2F/library.events" \
  -H "Content-Type: application/json" \
  -d '{"type":"topic","durable":true}' \
  && echo " ✓ library.events" || echo " ✗ library.events FAILED"

# ── Queues ───────────────────────────────────────────────────────────────────

echo "Creating queues..."

# Dead letter queue — no DLX of its own, messages sit here for manual inspection
curl -sf -u "$AUTH" -X PUT "$BASE_URL/queues/%2F/library.dlq" \
  -H "Content-Type: application/json" \
  -d '{"durable":true}' \
  && echo " ✓ library.dlq" || echo " ✗ library.dlq FAILED"

# Consumer queues — all declare DLX so failed messages go to library.dlq
QUEUE_ARGS='{"x-dead-letter-exchange":"library.dlx","x-dead-letter-routing-key":"dlq"}'

curl -sf -u "$AUTH" -X PUT "$BASE_URL/queues/%2F/inventory-svc.book-events" \
  -H "Content-Type: application/json" \
  -d "{\"durable\":true,\"arguments\":$QUEUE_ARGS}" \
  && echo " ✓ inventory-svc.book-events" || echo " ✗ inventory-svc.book-events FAILED"

curl -sf -u "$AUTH" -X PUT "$BASE_URL/queues/%2F/notification-svc.book-events" \
  -H "Content-Type: application/json" \
  -d "{\"durable\":true,\"arguments\":$QUEUE_ARGS}" \
  && echo " ✓ notification-svc.book-events" || echo " ✗ notification-svc.book-events FAILED"

curl -sf -u "$AUTH" -X PUT "$BASE_URL/queues/%2F/notification-svc.inventory-events" \
  -H "Content-Type: application/json" \
  -d "{\"durable\":true,\"arguments\":$QUEUE_ARGS}" \
  && echo " ✓ notification-svc.inventory-events" || echo " ✗ notification-svc.inventory-events FAILED"

# ── Bindings ─────────────────────────────────────────────────────────────────

echo "Creating bindings..."

# DLX → DLQ
curl -sf -u "$AUTH" -X POST "$BASE_URL/bindings/%2F/e/library.dlx/q/library.dlq" \
  -H "Content-Type: application/json" \
  -d '{"routing_key":"dlq"}' \
  && echo " ✓ library.dlx → library.dlq" || echo " ✗ binding FAILED"

# library.events → inventory-svc.book-events  (book.*)
curl -sf -u "$AUTH" -X POST "$BASE_URL/bindings/%2F/e/library.events/q/inventory-svc.book-events" \
  -H "Content-Type: application/json" \
  -d '{"routing_key":"book.*"}' \
  && echo " ✓ library.events[book.*] → inventory-svc.book-events" || echo " ✗ binding FAILED"

# library.events → notification-svc.book-events  (book.*)
curl -sf -u "$AUTH" -X POST "$BASE_URL/bindings/%2F/e/library.events/q/notification-svc.book-events" \
  -H "Content-Type: application/json" \
  -d '{"routing_key":"book.*"}' \
  && echo " ✓ library.events[book.*] → notification-svc.book-events" || echo " ✗ binding FAILED"

# library.events → notification-svc.inventory-events  (inventory.*)
curl -sf -u "$AUTH" -X POST "$BASE_URL/bindings/%2F/e/library.events/q/notification-svc.inventory-events" \
  -H "Content-Type: application/json" \
  -d '{"routing_key":"inventory.*"}' \
  && echo " ✓ library.events[inventory.*] → notification-svc.inventory-events" || echo " ✗ binding FAILED"

echo ""
echo "Done! Verify at http://localhost:15672 (guest / guest)"
echo "  → Exchanges tab: library.events, library.dlx"
echo "  → Queues tab: 4 queues should be listed"

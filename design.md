# LibraFlow — Design Document

## Services

### Book Service

The only service that owns book data. It handles create, update, borrow, return, and delete.
After every write, it publishes an event. That's it.

It does not know Inventory or Notification exist. If you remove those two services tomorrow,
Book Service keeps working the same way.

### Inventory Service

Tracks stock by listening to book events. It builds its own view of the world from
`book.added`, `book.borrowed`, `book.returned`, etc. It never queries Book's database.

When stock drops to 2 or fewer, it publishes `inventory.low_stock`. When it hits zero,
`inventory.out_of_stock`. These go back into RabbitMQ for Notification to pick up.

The data it stores (`title`, `author`) is copied from event payloads — denormalization is
intentional here. There's no cross-service query at runtime.

### Notification Service

Pure consumer, never publishes. It subscribes to `book.*` and `inventory.*` events and
saves a human-readable log entry for each one. Exposes a simple read API to query history.

Nothing fancy here — its job is to prove that adding a new consumer doesn't require touching
Book Service at all.

---

## Tech Stack

Kept minimal on purpose. Every dependency has a clear reason.

**Backend (per service)**

| What | Tool | Why |
|------|------|-----|
| HTTP | `go-chi/chi` | Stdlib-compatible, no magic. Middleware works with plain `http.Handler`. |
| Database | `jackc/pgx` | Native Postgres driver. Write SQL directly, no ORM. |
| Migration | `pressly/goose` | Simple Up/Down SQL files. Embeds cleanly into the binary. |
| Messaging | `rabbitmq/amqp091-go` | Official RabbitMQ client for Go. |
| Logging | `rs/zerolog` | JSON output, zero allocation, good API. |
| UUID | `google/uuid` | For event IDs and correlation IDs. |
| Config | `os.Getenv` + struct | Don't need viper for 5 env vars. |

**Frontend**

| What | Tool | Why |
|------|------|-----|
| UI | Vanilla HTML + CSS + JS | No build step, no framework. Single `index.html` is enough. |
| Server | `nginx:alpine` | Serves the static files and reverse-proxies API calls to avoid CORS issues. |

The frontend polls the three APIs every few seconds and renders inventory status and notification
feed. No framework needed — the whole thing is one HTML file with a `<script>` block.

No ORM, no code generation, no annotation-based docs. Just Go talking to Postgres and RabbitMQ.

---

## Events

Every event uses the same envelope:

```json
{
  "event_id":       "uuid-v4",
  "event_type":     "book.borrowed",
  "source_service": "book-service",
  "schema_version": "1.0",
  "timestamp":      "2024-01-15T10:30:00Z",
  "correlation_id": "uuid-v4",
  "payload":        {}
}
```

`event_id` doubles as the idempotency key — consumers store it in `processed_events` and skip
anything they've already seen. `correlation_id` comes from the original HTTP request and flows
through every downstream log entry.

### Events published by Book Service

| Event | When |
|-------|------|
| `book.added` | POST /books |
| `book.updated` | PATCH /books/{id} |
| `book.borrowed` | POST /books/{id}/borrow |
| `book.returned` | POST /books/{id}/return |
| `book.deleted` | DELETE /books/{id} |

### Events published by Inventory Service

| Event | When |
|-------|------|
| `inventory.low_stock` | available_count drops to ≤ 2 |
| `inventory.out_of_stock` | available_count hits 0 |

### Who subscribes to what

| Event | Inventory | Notification |
|-------|:---------:|:------------:|
| `book.added` | ✅ | ✅ |
| `book.updated` | ✅ | |
| `book.borrowed` | ✅ | ✅ |
| `book.returned` | ✅ | ✅ |
| `book.deleted` | ✅ | |
| `inventory.low_stock` | | ✅ |
| `inventory.out_of_stock` | | ✅ |

---

## RabbitMQ Setup

One topic exchange: `library.events`. Routing keys follow the `<domain>.<action>` pattern.

```
Exchange: library.events  (topic, durable)

Queues:
  inventory-svc.book-events          → binding: book.*
  notification-svc.book-events       → binding: book.*
  notification-svc.inventory-events  → binding: inventory.*

Dead letter:
  Exchange: library.dlx  (direct, durable)
  Queue:    library.dlq
```

Every consumer queue declares `x-dead-letter-exchange: library.dlx`. When a message
fails after 3 retries, it gets NACK'd without requeue and lands in the DLQ for inspection.

Retry timing: 1s → 2s → 4s → DLQ.

---

## Data Models

### book_db

```sql
-- +goose Up
CREATE TABLE books (
    id               UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    title            VARCHAR(255) NOT NULL,
    author           VARCHAR(255) NOT NULL,
    isbn             VARCHAR(20)  NOT NULL UNIQUE,
    total_quantity   INT         NOT NULL CHECK (total_quantity > 0),
    available_count  INT         NOT NULL CHECK (available_count >= 0),
    status           VARCHAR(20) NOT NULL DEFAULT 'active',
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE borrow_records (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    book_id     UUID        NOT NULL REFERENCES books(id),
    member_id   VARCHAR(100) NOT NULL,
    member_name VARCHAR(255),
    borrowed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    due_date    TIMESTAMPTZ NOT NULL,
    returned_at TIMESTAMPTZ,
    status      VARCHAR(20) NOT NULL DEFAULT 'borrowed'
);
-- +goose Down
DROP TABLE borrow_records; DROP TABLE books;
```

### inventory_db

```sql
-- +goose Up
CREATE TABLE inventory (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    book_id         UUID        NOT NULL UNIQUE,
    title           VARCHAR(255) NOT NULL,
    author          VARCHAR(255) NOT NULL,
    total_quantity  INT         NOT NULL DEFAULT 0,
    available_count INT         NOT NULL DEFAULT 0,
    borrowed_count  INT         NOT NULL DEFAULT 0,
    status          VARCHAR(20) NOT NULL DEFAULT 'available',
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- status: available | low_stock | out_of_stock | deleted

CREATE TABLE processed_events (
    event_id     UUID        PRIMARY KEY,
    event_type   VARCHAR(100) NOT NULL,
    processed_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- +goose Down
DROP TABLE processed_events; DROP TABLE inventory;
```

### notification_db

```sql
-- +goose Up
CREATE TABLE notifications (
    id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    event_id   UUID        NOT NULL UNIQUE,
    event_type VARCHAR(100) NOT NULL,
    member_id  VARCHAR(100),
    book_id    UUID,
    book_title VARCHAR(255),
    message    TEXT        NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE processed_events (
    event_id     UUID        PRIMARY KEY,
    event_type   VARCHAR(100) NOT NULL,
    processed_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- +goose Down
DROP TABLE processed_events; DROP TABLE notifications;
```

---

## APIs

### Book Service — :8001

| Method | Path | Event |
|--------|------|-------|
| POST | `/books` | `book.added` |
| GET | `/books` | |
| GET | `/books/{id}` | |
| PATCH | `/books/{id}` | `book.updated` |
| POST | `/books/{id}/borrow` | `book.borrowed` |
| POST | `/books/{id}/return` | `book.returned` |
| DELETE | `/books/{id}` | `book.deleted` |
| GET | `/health` | |

Borrow request body:
```json
{ "member_id": "m-001", "member_name": "Alice", "days": 14 }
```

Return request body:
```json
{ "member_id": "m-001" }
```

Error cases worth noting:
- Borrow when `available_count == 0` → `409 Conflict`
- Return when no active borrow record found → `404 Not Found`

### Inventory Service — :8002

| Method | Path |
|--------|------|
| GET | `/inventory` |
| GET | `/inventory/{book_id}` |
| GET | `/inventory/low-stock` |
| GET | `/inventory/out-of-stock` |
| GET | `/health` |

### Notification Service — :8003

| Method | Path |
|--------|------|
| GET | `/notifications` |
| GET | `/notifications/{id}` |
| GET | `/health` |

`/notifications` accepts `?member_id=`, `?book_id=`, `?event_type=` query params.

All three `/health` endpoints return:
```json
{ "status": "healthy", "service": "book-service", "database": "connected", "broker": "connected" }
```

---

## Patterns

### Idempotency

RabbitMQ delivers at-least-once, so consumers will occasionally see duplicates — on restart,
network blip, whatever. The fix is simple: check `processed_events` before doing anything,
and insert into it in the same transaction as the actual work.

```go
func (h *Handler) handle(ctx context.Context, env EventEnvelope) error {
    return h.db.WithTx(ctx, func(tx pgx.Tx) error {
        // Already seen this event? Skip it.
        var exists bool
        err := tx.QueryRow(ctx,
            `SELECT EXISTS(SELECT 1 FROM processed_events WHERE event_id = $1)`,
            env.EventID,
        ).Scan(&exists)
        if err != nil { return err }
        if exists { return nil }

        // Do the real work
        if err := applyEvent(ctx, tx, env); err != nil { return err }

        // Mark as done — same transaction, so it's atomic
        _, err = tx.Exec(ctx,
            `INSERT INTO processed_events (event_id, event_type) VALUES ($1, $2)`,
            env.EventID, env.EventType,
        )
        return err
    })
}
```

### Correlation ID

Every HTTP request that comes into Book Service either reads `X-Correlation-ID` from the header
or generates a new UUID. That ID goes into the event envelope. Every consumer logs it when
processing. Result: you can grep a single correlation ID across three service logs and see the
full chain of what happened.

### Structured logging

Using zerolog. Every log line includes `service`, `correlation_id`, and whatever context makes
sense for that operation. In Docker, all output goes to stdout as JSON — easy to pipe into
any log aggregator later.

```go
log.Info().
    Str("service", "book-service").
    Str("correlation_id", corrID).
    Str("book_id", bookID.String()).
    Str("member_id", memberID).
    Msg("book borrowed, publishing event")
```

---

## Project Structure

```
LibraFlow/
├── docker-compose.yml
├── docker-compose.override.yml   # dev: exposes DB ports
├── .env.example
├── Makefile
├── README.md
├── design.md
│
├── frontend/
│   ├── index.html                # single-page dashboard
│   ├── style.css
│   └── app.js                   # fetch APIs, poll every 5s, render
│
├── nginx/
│   └── nginx.conf               # serve frontend + reverse-proxy /api/* to services
│
└── services/
    ├── book-service/
    │   ├── Dockerfile
    │   ├── go.mod
    │   ├── cmd/server/main.go        # wire everything up, start server
    │   ├── internal/
    │   │   ├── config/config.go      # reads env vars into a struct
    │   │   ├── domain/book.go        # Book, BorrowRecord types
    │   │   ├── handler/book.go       # chi handlers
    │   │   ├── middleware/
    │   │   │   ├── correlation.go    # X-Correlation-ID
    │   │   │   └── logger.go         # request logging
    │   │   ├── repository/book.go    # pgx queries
    │   │   └── publisher/publisher.go
    │   └── migrations/
    │       ├── 001_create_books.sql
    │       └── 002_create_borrow_records.sql
    │
    ├── inventory-service/
    │   ├── Dockerfile
    │   ├── go.mod
    │   ├── cmd/server/main.go
    │   ├── internal/
    │   │   ├── config/config.go
    │   │   ├── domain/inventory.go
    │   │   ├── handler/inventory.go
    │   │   ├── repository/inventory.go
    │   │   ├── consumer/book_consumer.go   # book.* handler + idempotency
    │   │   └── publisher/publisher.go      # inventory.* events
    │   └── migrations/
    │       └── 001_create_inventory.sql
    │
    └── notification-service/
        ├── Dockerfile
        ├── go.mod
        ├── cmd/server/main.go
        ├── internal/
        │   ├── config/config.go
        │   ├── domain/notification.go
        │   ├── handler/notification.go
        │   ├── repository/notification.go
        │   └── consumer/event_consumer.go  # book.* + inventory.* handler
        └── migrations/
            └── 001_create_notifications.sql
```

No shared Go module between services — each service is its own Go module with its own `go.mod`.
They share the `EventEnvelope` type but it's small enough to just copy; avoiding a shared module
keeps the services genuinely independent.

---

## Docker

### Dockerfile (same pattern for all three services)

```dockerfile
FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /server ./cmd/server

FROM alpine:3.19
RUN apk add --no-cache ca-certificates curl
RUN addgroup -S app && adduser -S app -G app
WORKDIR /app
COPY --from=builder /server ./server
COPY migrations/ ./migrations/
USER app
EXPOSE 8000
HEALTHCHECK --interval=30s --timeout=10s --start-period=40s --retries=3 \
  CMD curl -f http://localhost:8000/health || exit 1
ENTRYPOINT ["./server"]
```

Final image is around 15–20 MB. The binary includes the migrations directory because goose
can embed them at startup.

### docker-compose.yml

```yaml
version: "3.9"

services:
  rabbitmq:
    image: rabbitmq:3.13-management-alpine
    ports: ["5672:5672", "15672:15672"]
    environment:
      RABBITMQ_DEFAULT_USER: guest
      RABBITMQ_DEFAULT_PASS: guest
    volumes: [rabbitmq_data:/var/lib/rabbitmq]
    healthcheck:
      test: ["CMD", "rabbitmq-diagnostics", "ping"]
      interval: 15s
      timeout: 10s
      retries: 5

  postgres-book:
    image: postgres:16-alpine
    environment: { POSTGRES_DB: book_db, POSTGRES_USER: book, POSTGRES_PASSWORD: book }
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U book -d book_db"]
      interval: 10s

  postgres-inventory:
    image: postgres:16-alpine
    environment: { POSTGRES_DB: inventory_db, POSTGRES_USER: inv, POSTGRES_PASSWORD: inv }
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U inv -d inventory_db"]
      interval: 10s

  postgres-notification:
    image: postgres:16-alpine
    environment: { POSTGRES_DB: notification_db, POSTGRES_USER: notif, POSTGRES_PASSWORD: notif }
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U notif -d notification_db"]
      interval: 10s

  book-service:
    build: { context: services/book-service }
    env_file: services/book-service/.env
    depends_on:
      rabbitmq: { condition: service_healthy }
      postgres-book: { condition: service_healthy }

  inventory-service:
    build: { context: services/inventory-service }
    env_file: services/inventory-service/.env
    depends_on:
      rabbitmq: { condition: service_healthy }
      postgres-inventory: { condition: service_healthy }

  notification-service:
    build: { context: services/notification-service }
    env_file: services/notification-service/.env
    depends_on:
      rabbitmq: { condition: service_healthy }
      postgres-notification: { condition: service_healthy }

  nginx:
    image: nginx:alpine
    ports: ["80:80"]
    volumes:
      - ./frontend:/usr/share/nginx/html:ro
      - ./nginx/nginx.conf:/etc/nginx/conf.d/default.conf:ro
    depends_on:
      - book-service
      - inventory-service
      - notification-service

volumes:
  rabbitmq_data:
  postgres_book_data:
  postgres_inventory_data:
  postgres_notification_data:
```

Note: backend services no longer expose ports directly — all traffic goes through nginx on `:80`.
The override file exposes them for local development.

### nginx.conf

```nginx
server {
    listen 80;

    # Serve the frontend
    location / {
        root /usr/share/nginx/html;
        index index.html;
    }

    # Reverse-proxy API calls — avoids CORS issues since everything is same origin
    location /api/books/         { proxy_pass http://book-service:8000/books/; }
    location /api/inventory/     { proxy_pass http://inventory-service:8000/inventory/; }
    location /api/notifications/ { proxy_pass http://notification-service:8000/notifications/; }
}
```

### Makefile

```makefile
.PHONY: up down logs

up:
	docker compose up --build -d

down:
	docker compose down -v

logs:
	docker compose logs -f
```

---

## Environment Variables

Each service reads these from its `.env` file (or environment in Docker):

| Variable | Example |
|----------|---------|
| `DATABASE_URL` | `postgres://book:book@postgres-book:5432/book_db?sslmode=disable` |
| `RABBITMQ_URL` | `amqp://guest:guest@rabbitmq:5672/` |
| `SERVER_PORT` | `8000` |
| `LOG_LEVEL` | `info` |
| `LOW_STOCK_THRESHOLD` | `2` (inventory only) |
| `MAX_RETRY_ATTEMPTS` | `3` (consumers only) |

---

## Execution Plan

**Day 1** — Docker Compose up, RabbitMQ exchange/queues declared, shared types in place.

**Day 2** — Book Service: migrations, repository, handlers, publisher. Verify events appear in RabbitMQ UI.

**Day 3** — Inventory Service: consumer with idempotency, stock transitions, inventory events, query API.

**Day 4** — Notification Service: consumer, message formatting, query API. Test DLQ by forcing a consumer error.

**Day 5** — Frontend dashboard (inventory cards + notification feed), nginx config, wire everything through `localhost:80`.

**Day 6 (buffer)** — Dockerfile polish (non-root user, healthcheck), correlation ID end-to-end test, README with demo.

---

## Demo

```bash
# Start everything
docker compose up --build

# Dashboard
open http://localhost          # inventory + notification feed

# RabbitMQ Management UI — show live queues, DLQ
open http://localhost:15672    # guest / guest
```

Or with curl if you prefer:

```bash
BOOK_ID="<id from POST /api/books>"

# Add a book
curl -s -X POST http://localhost/api/books/ \
  -H "Content-Type: application/json" \
  -d '{"title":"Clean Code","author":"Robert C. Martin","isbn":"978-1","total_quantity":3}' | jq

# Borrow twice — watch the dashboard update automatically
curl -s -X POST http://localhost/api/books/$BOOK_ID/borrow \
  -H "Content-Type: application/json" \
  -d '{"member_id":"m-001","member_name":"Alice","days":14}'

curl -s -X POST http://localhost/api/books/$BOOK_ID/borrow \
  -H "Content-Type: application/json" \
  -d '{"member_id":"m-002","member_name":"Bob","days":14}'

# Inventory and notifications updated with no direct service calls
curl -s http://localhost/api/inventory/ | jq
curl -s http://localhost/api/notifications/ | jq
```

# Event-Driven Library Management System — Design Document

**Stack**: Go 1.22 · chi · pgx · goose · LocalStack (AWS SNS + SQS) · Docker

---

## What is this?

A practice project for event-driven pub/sub using a library system as the domain.
The goal is to keep it small enough to finish in a week but designed well enough to
be worth showing — proper service boundaries, real messaging patterns (AWS SNS/SQS via LocalStack), and Docker-ready.

Three services: **Book**, **Inventory**, **Notification**. They never call each other over HTTP.
Everything goes through AWS SNS + SQS.

---

## Why these three services?

The combination demonstrates the two most important things about pub/sub:

1. **Decoupling** — Book Service publishes events to an SNS Topic without knowing who's listening.
2. **Fan-out** — SNS automatically fans out one event to multiple SQS queues simultaneously.

```
Book Service ──► [ SNS Topic: library-events ]
                        │
       ┌────────────────┴────────────────┐
       ▼                                 ▼
[ SQS Queue: inventory ]        [ SQS Queue: notification ]
       │                                 │
       ▼                                 ▼
Inventory Service               Notification Service
```

Adding a User/Member service would just be more CRUD without teaching anything new about
messaging. So `member_id` is a plain string field in the borrow request — the caller owns it.

---

## Services

### Book Service

The only service that owns book data. It handles create, update, borrow, return, and delete.
After every write, it publishes an event to AWS SNS. That's it.

It does not know Inventory or Notification exist. If you remove those two services tomorrow,
Book Service keeps working the same way.

### Inventory Service

Tracks stock by listening to its SQS queue (`inventory-book-events`). It builds its own view of the world from
`book.added`, `book.borrowed`, `book.returned`, etc. It never queries Book's database.

When stock drops to 2 or fewer, it publishes `inventory.low_stock`. When it hits zero,
`inventory.out_of_stock`. These go back into SNS for Notification to pick up.

The data it stores (`title`, `author`) is copied from event payloads — denormalization is
intentional here. There's no cross-service query at runtime.

### Notification Service

Pure consumer, never publishes. It subscribes to its SQS queue (`notification-events`) and
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
| Messaging | `aws-sdk-go-v2` | Official AWS SDK v2 for Go (SNS & SQS). |
| Cloud Mock | `localstack/localstack` | Emulates AWS SNS and SQS locally in Docker. |
| Logging | `rs/zerolog` | JSON output, zero allocation, good API. |
| UUID | `google/uuid` | For event IDs and correlation IDs. |
| Config | `os.Getenv` + struct | Don't need viper for 5 env vars. |

**Frontend**

| What | Tool | Why |
|------|------|-----|
| UI | Vanilla HTML + CSS + JS | No build step, no framework. Single `index.html` is enough. |
| Server | `nginx:alpine` | Serves static files and reverse-proxies API calls to avoid CORS issues. |

The frontend polls the three APIs every few seconds and renders inventory status and notification feed.

### Frontend Dashboard Design & Canvas

```
┌──────────────────────────────────────────────────────────────────────────────────────────────────┐
│ Library System          Catalog     Inventory     Activity Log          [● All services online]  │
├──────────────────────────────────────────────────────────────────────────────────────────────────┤
│                                                                                                  │
│ Books & Circulation                                                            [ Add Book ]      │
│ Manage book records, track available copies, and handle lending operations.                      │
│                                                                                                  │
├──────────────────────────────────────────────────────────────────────────────────────────────────┤
│ [ Search by title, author, or ISBN...             ]  [ Status: All ▼ ]     [ Auto-refresh: 2s ]  │
│                                                                                                  │
│ ┌──────────────────────────────────────────────────────────────────────────────────────────────┐ │
│ │ Title & Author                 ISBN            Copies      Status         Actions            │ │
│ ├──────────────────────────────────────────────────────────────────────────────────────────────┤ │
│ │ Clean Code                     9780132350884   5 / 5       Available      [Borrow] [Return]  │ │
│ │ Robert C. Martin                                                                             │ │
│ ├──────────────────────────────────────────────────────────────────────────────────────────────┤ │
│ │ The Go Programming Language    9780134190440   1 / 3       Low stock      [Borrow] [Return]  │ │
│ │ Alan Donovan, Brian Kernighan                                                                │ │
│ ├──────────────────────────────────────────────────────────────────────────────────────────────┤ │
│ │ Designing Data-Intensive Apps  9781449373320   0 / 1       Out of stock   [Borrow] [Return]  │ │
│ │ Martin Kleppmann                                                                             │ │
│ └──────────────────────────────────────────────────────────────────────────────────────────────┘ │
│                                                                                                  │
├──────────────────────────────────────────────────────────────────────────────────────────────────┤
│ Event Stream (AWS SNS / SQS)                                              [ Showing last 20 ]    │
│ ┌──────────────────────────────────────────────────────────────────────────────────────────────┐ │
│ │ 11:58:24  book.borrowed          Alice borrowed 'The Go Programming Language'                │ │
│ │ 11:58:24  inventory.low_stock    Available stock for book 9780134190440 dropped to 1         │ │
│ │ 11:57:10  book.added             Added new book 'Clean Code' with 5 copies                   │ │
│ └──────────────────────────────────────────────────────────────────────────────────────────────┘ │
└──────────────────────────────────────────────────────────────────────────────────────────────────┘
```

**Key Principles:**
1. **Clean & Minimalist**: Standard typography, clear table layouts, subtle border lines, flat status badges, no unnecessary decorative emojis or icons.
2. **Real-world Functionality**: Instant search, status filtering, add/borrow/return modals, and live activity stream.
3. **Vanilla Tech Stack**: Pure HTML5, CSS3, and JavaScript without build steps or runtime overhead.

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

---

## AWS SNS + SQS Setup (LocalStack)

One SNS Topic: `library-events`.

```
SNS Topic: arn:aws:sns:us-east-1:000000000000:library-events

SQS Queues:
  inventory-book-events   ← Subscribed to library-events
  notification-events     ← Subscribed to library-events

Dead Letter Queue:
  library-dlq             ← Attached via SQS RedrivePolicy (maxReceiveCount: 3)
```

Auto-initialized on LocalStack startup via `localstack/init-aws.sh`.

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
    id             UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    event_id       UUID        NOT NULL UNIQUE,
    correlation_id VARCHAR(100),
    event_type     VARCHAR(100) NOT NULL,
    member_id      VARCHAR(100),
    book_id        UUID,
    book_title     VARCHAR(255),
    message        TEXT        NOT NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
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
| GET | `/books/{id}/borrows` | |
| POST | `/books/{id}/borrow` | `book.borrowed` |
| POST | `/books/{id}/return` | `book.returned` |
| DELETE | `/books/{id}` | `book.deleted` |
| GET | `/health` | |

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

---

## Patterns

### Idempotency

SQS delivers at-least-once, so consumers will occasionally see duplicates. The fix: check `processed_events` table before doing anything, and insert into it in the same transaction as the actual work.

```go
func (h *Handler) handle(ctx context.Context, env EventEnvelope) error {
    return h.db.WithTx(ctx, func(tx pgx.Tx) error {
        var exists bool
        err := tx.QueryRow(ctx,
            `SELECT EXISTS(SELECT 1 FROM processed_events WHERE event_id = $1)`,
            env.EventID,
        ).Scan(&exists)
        if err != nil { return err }
        if exists { return nil }

        if err := applyEvent(ctx, tx, env); err != nil { return err }

        _, err = tx.Exec(ctx,
            `INSERT INTO processed_events (event_id, event_type) VALUES ($1, $2)`,
            env.EventID, env.EventType,
        )
        return err
    })
}
```

---

## Project Structure

```
library-system/
├── docker-compose.yml
├── .env.example
├── Makefile
├── README.md
├── design.md
│
├── frontend/
│   ├── index.html
│   ├── style.css
│   └── app.js
│
├── nginx/
│   └── nginx.conf
│
├── localstack/
│   └── init-aws.sh              # Auto-creates SNS topics & SQS queues on boot
│
├── scripts/
│   ├── seed.go                  # Demo data populator
│   └── test_e2e.go              # Automated integration tests
│
└── services/
    ├── book-service/
    ├── inventory-service/
    └── notification-service/
```

---

## Environment Variables

| Variable | Service | Example |
|----------|---------|---------|
| `DATABASE_URL` | all | `postgres://book:book@postgres-book:5432/book_db?sslmode=disable` |
| `AWS_REGION` | all | `us-east-1` |
| `AWS_ENDPOINT_URL` | all | `http://localstack:4566` |
| `SNS_TOPIC_ARN` | book, inventory | `arn:aws:sns:us-east-1:000000000000:library-events` |
| `SQS_QUEUE_URL` | inventory, notification | `http://localstack:4566/000000000000/inventory-book-events` |
| `SERVER_PORT` | all | `8001` (book), `8002` (inventory), `8003` (notification) |
| `LOW_STOCK_THRESHOLD` | inventory | `2` |
| `LOG_LEVEL` | all | `info` |

---

## Execution Plan

**Day 1** — Docker Compose up (LocalStack SNS/SQS + 3x Postgres auto-initialized).

**Day 2** — Book Service: migrations, repository, handlers, SNS publisher (`aws-sdk-go-v2`).

**Day 3** — Inventory Service: SQS consumer with idempotency, stock transitions, SNS publisher.

**Day 4** — Notification Service: SQS consumer, message formatting, query API. DLQ test.

**Day 5** — Frontend dashboard (inventory cards + notification feed), nginx config.

**Day 6 (buffer)** — Dockerfile polish, correlation ID trace, README with demo.

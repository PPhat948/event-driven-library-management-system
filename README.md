# Event-Driven Library Management System

An event-driven library management system demonstrating pub/sub messaging patterns using Go, AWS SNS/SQS (LocalStack), PostgreSQL, and a vanilla JavaScript dashboard.

---

## Architecture

Three decoupled services communicate asynchronously over AWS SNS and SQS:

```
[ Book Service ] ──► ( SNS Topic: library-events )
                              │
              ┌───────────────┴───────────────┐
              ▼                               ▼
  [ SQS: inventory-book-events ]     [ SQS: notification-events ]
              │                               │
              ▼                               ▼
    [ Inventory Service ]           [ Notification Service ]
```

### Services

- **Book Service (`:8001`)**: Owns the book catalog and handles borrow/return transactions. Publishes events (`book.added`, `book.updated`, `book.borrowed`, `book.returned`, `book.deleted`) to the SNS topic.
- **Inventory Service (`:8002`)**: Consumes events from `inventory-book-events` queue to maintain its own stock view. Publishes stock transition alerts (`inventory.low_stock`, `inventory.out_of_stock`) back to SNS.
- **Notification Service (`:8003`)**: Consumes events from `notification-events` queue, logs them with correlation IDs for tracing, and exposes an activity history API.
- **Nginx Gateway & Web UI (`:80`)**: Serves the frontend dashboard and reverse-proxies requests to `/api/*`.

---

## Quick Start

### 1. Run the entire stack

```bash
make up
```

This starts LocalStack, 3 PostgreSQL instances, all 3 Go services, and Nginx.

Access points:
- Dashboard: http://localhost
- LocalStack Gateway: http://localhost:4566

### 2. Seed initial data

```bash
make seed
```

Creates sample books and borrow records to demonstrate stock transitions and event fan-out.

### 3. Run tests

```bash
# Unit tests
make test

# Integration tests
make test-integration

# End-to-end integration test
make test-e2e
```

### 4. Stop the stack

```bash
make down
```

---

## API Reference

### Book Service (`/api/books` or port `8001`)

| Method | Path | Description |
|---|---|---|
| POST | `/books` | Create a new book |
| GET | `/books` | List books |
| GET | `/books/{id}` | Get book by ID |
| PATCH | `/books/{id}` | Update book title, author, or total quantity |
| GET | `/books/{id}/borrows` | List active borrow records for a book |
| POST | `/books/{id}/borrow` | Borrow a copy |
| POST | `/books/{id}/return` | Return a borrowed copy |
| DELETE | `/books/{id}` | Delete book |
| GET | `/health` | Health check |

### Inventory Service (`/api/inventory` or port `8002`)

| Method | Path | Description |
|---|---|---|
| GET | `/inventory` | List all inventory items |
| GET | `/inventory/{book_id}` | Get inventory item by book ID |
| GET | `/inventory/low-stock` | List items with low stock (<= 2) |
| GET | `/inventory/out-of-stock` | List out-of-stock items (0 copies) |
| GET | `/health` | Health check |

### Notification Service (`/api/notifications` or port `8003`)

| Method | Path | Description |
|---|---|---|
| GET | `/notifications` | List event activity feed (supports `limit`, `offset`) |
| GET | `/notifications/{id}` | Get notification by ID |
| GET | `/health` | Health check |

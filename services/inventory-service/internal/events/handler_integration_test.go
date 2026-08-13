//go:build integration

package events

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"

	"inventory-service/internal"
)

func setupTestDB(t *testing.T) *pgxpool.Pool {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://inv:inv@localhost:5433/inventory_db?sslmode=disable"
	}

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Skipf("skipping integration test: cannot connect to postgres at %s: %v", dbURL, err)
		return nil
	}

	if err := pool.Ping(ctx); err != nil {
		t.Skipf("skipping integration test: postgres ping failed at %s: %v", dbURL, err)
		return nil
	}

	// Clean tables before running tests
	_, _ = pool.Exec(ctx, "TRUNCATE TABLE inventory, processed_events RESTART IDENTITY CASCADE;")
	return pool
}

func TestHandlerIntegration(t *testing.T) {
	pool := setupTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()

	log := zerolog.New(os.Stdout)
	repo := internal.NewInventoryRepo(pool)
	mockPub := NewMockPublisher()
	handler := NewHandler(pool, repo, mockPub, 2, log)
	ctx := context.Background()

	t.Run("HandleBookAdded upserts book into inventory", func(t *testing.T) {
		bookID := uuid.New()
		eventID := uuid.New()

		payload, _ := json.Marshal(BookAddedPayload{
			BookID:        bookID.String(),
			Title:         "Integration Test Book",
			Author:        "Author A",
			TotalQuantity: 5,
			AvailableCount: 5,
		})
		env, _ := json.Marshal(EventEnvelope{
			EventID:       eventID.String(),
			EventType:     "book.added",
			SourceService: "book-service",
			SchemaVersion: "1.0",
			Timestamp:     time.Now().UTC(),
			CorrelationID: "corr-101",
			Payload:       payload,
		})

		err := handler.Handle(ctx, string(env))
		if err != nil {
			t.Fatalf("Handle() error = %v", err)
		}

		item, err := repo.GetByBookID(ctx, bookID)
		if err != nil {
			t.Fatalf("GetByBookID() error = %v", err)
		}
		if item.Title != "Integration Test Book" || item.AvailableCount != 5 || item.Status != "available" {
			t.Errorf("Inventory item = %+v, want title='Integration Test Book', avail=5, status='available'", item)
		}
	})

	t.Run("Idempotency: Duplicate event_id is skipped without double insertion", func(t *testing.T) {
		bookID := uuid.New()
		eventID := uuid.New()

		payload, _ := json.Marshal(BookAddedPayload{
			BookID:        bookID.String(),
			Title:         "Idempotency Book",
			Author:        "Author B",
			TotalQuantity: 3,
			AvailableCount: 3,
		})
		env, _ := json.Marshal(EventEnvelope{
			EventID:       eventID.String(),
			EventType:     "book.added",
			SourceService: "book-service",
			SchemaVersion: "1.0",
			Timestamp:     time.Now().UTC(),
			CorrelationID: "corr-102",
			Payload:       payload,
		})

		// Deliver first time
		if err := handler.Handle(ctx, string(env)); err != nil {
			t.Fatalf("First Handle() error = %v", err)
		}

		// Deliver second time (duplicate event_id)
		if err := handler.Handle(ctx, string(env)); err != nil {
			t.Fatalf("Second Handle() error = %v", err)
		}

		// Verify only 1 record exists
		var count int
		_ = pool.QueryRow(ctx, "SELECT COUNT(*) FROM inventory WHERE book_id = $1", bookID).Scan(&count)
		if count != 1 {
			t.Errorf("Inventory count for book = %d, want 1", count)
		}
	})

	t.Run("HandleBookBorrowed triggers low_stock alert when available <= threshold", func(t *testing.T) {
		mockPub.Reset()
		bookID := uuid.New()
		addEventID := uuid.New()

		// 1. Add book with Qty 3
		addPayload, _ := json.Marshal(BookAddedPayload{
			BookID:        bookID.String(),
			Title:         "Low Stock Book",
			Author:        "Author C",
			TotalQuantity: 3,
			AvailableCount: 3,
		})
		addEnv, _ := json.Marshal(EventEnvelope{
			EventID:       addEventID.String(),
			EventType:     "book.added",
			CorrelationID: "corr-103",
			Payload:       addPayload,
		})
		_ = handler.Handle(ctx, string(addEnv))

		// 2. Borrow book to reduce available to 1 (Threshold is 2)
		borrowEventID := uuid.New()
		borrowPayload, _ := json.Marshal(BookBorrowedPayload{
			BookID:              bookID.String(),
			BookTitle:           "Low Stock Book",
			AvailableCountAfter: 1,
		})
		borrowEnv, _ := json.Marshal(EventEnvelope{
			EventID:       borrowEventID.String(),
			EventType:     "book.borrowed",
			CorrelationID: "corr-103",
			Payload:       borrowPayload,
		})

		if err := handler.Handle(ctx, string(borrowEnv)); err != nil {
			t.Fatalf("Borrow Handle() error = %v", err)
		}

		// Verify DB status updated to low_stock
		item, _ := repo.GetByBookID(ctx, bookID)
		if item.Status != "low_stock" || item.AvailableCount != 1 {
			t.Errorf("Item status = %q, available = %d; want 'low_stock', 1", item.Status, item.AvailableCount)
		}

		// Verify MockPublisher received inventory.low_stock alert
		events := mockPub.Events()
		if len(events) != 1 || events[0].EventType != "inventory.low_stock" {
			t.Errorf("Published events = %+v, want 1 'inventory.low_stock' event", events)
		}
	})

	t.Run("HandleBookBorrowed triggers out_of_stock alert when available == 0", func(t *testing.T) {
		mockPub.Reset()
		bookID := uuid.New()
		addEventID := uuid.New()

		addPayload, _ := json.Marshal(BookAddedPayload{
			BookID:        bookID.String(),
			Title:         "Out of Stock Book",
			Author:        "Author D",
			TotalQuantity: 1,
			AvailableCount: 1,
		})
		addEnv, _ := json.Marshal(EventEnvelope{
			EventID:       addEventID.String(),
			EventType:     "book.added",
			CorrelationID: "corr-104",
			Payload:       addPayload,
		})
		_ = handler.Handle(ctx, string(addEnv))

		borrowEventID := uuid.New()
		borrowPayload, _ := json.Marshal(BookBorrowedPayload{
			BookID:              bookID.String(),
			BookTitle:           "Out of Stock Book",
			AvailableCountAfter: 0,
		})
		borrowEnv, _ := json.Marshal(EventEnvelope{
			EventID:       borrowEventID.String(),
			EventType:     "book.borrowed",
			CorrelationID: "corr-104",
			Payload:       borrowPayload,
		})

		_ = handler.Handle(ctx, string(borrowEnv))

		item, _ := repo.GetByBookID(ctx, bookID)
		if item.Status != "out_of_stock" || item.AvailableCount != 0 {
			t.Errorf("Item status = %q, available = %d; want 'out_of_stock', 0", item.Status, item.AvailableCount)
		}

		events := mockPub.Events()
		if len(events) != 1 || events[0].EventType != "inventory.out_of_stock" {
			t.Errorf("Published events = %+v, want 1 'inventory.out_of_stock' event", events)
		}
	})

	t.Run("OutOfOrder delivery for untracked book skips gracefully without error", func(t *testing.T) {
		untrackedID := uuid.New()
		borrowEventID := uuid.New()

		borrowPayload, _ := json.Marshal(BookBorrowedPayload{
			BookID:              untrackedID.String(),
			BookTitle:           "Untracked Book",
			AvailableCountAfter: 1,
		})
		borrowEnv, _ := json.Marshal(EventEnvelope{
			EventID:       borrowEventID.String(),
			EventType:     "book.borrowed",
			CorrelationID: "corr-105",
			Payload:       borrowPayload,
		})

		err := handler.Handle(ctx, string(borrowEnv))
		if err != nil {
			t.Errorf("Handle() for untracked book returned error = %v, want nil", err)
		}
	})
}

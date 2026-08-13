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

	"notification-service/internal"
)

func setupTestDB(t *testing.T) *pgxpool.Pool {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://notif:notif@localhost:5434/notification_db?sslmode=disable"
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

	_, _ = pool.Exec(ctx, "TRUNCATE TABLE notifications, processed_events RESTART IDENTITY CASCADE;")
	return pool
}

func TestNotificationHandlerIntegration(t *testing.T) {
	pool := setupTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()

	log := zerolog.New(os.Stdout)
	repo := internal.NewNotificationRepo(pool)
	handler := NewHandler(pool, repo, log)
	ctx := context.Background()

	t.Run("Handle stores book.added notification in DB", func(t *testing.T) {
		eventID := uuid.New()
		bookID := uuid.New()

		payload, _ := json.Marshal(BookAddedPayload{
			BookID:        bookID.String(),
			Title:         "Notification Test Book",
			Author:        "Author N",
			TotalQuantity: 4,
		})
		env, _ := json.Marshal(EventEnvelope{
			EventID:       eventID.String(),
			EventType:     "book.added",
			SourceService: "book-service",
			SchemaVersion: "1.0",
			Timestamp:     time.Now().UTC(),
			CorrelationID: "corr-201",
			Payload:       payload,
		})

		if err := handler.Handle(ctx, string(env)); err != nil {
			t.Fatalf("Handle() error = %v", err)
		}

		list, err := repo.List(ctx, 10, 0)
		if err != nil {
			t.Fatalf("repo.List() error = %v", err)
		}

		if len(list) != 1 {
			t.Fatalf("Notification list len = %d, want 1", len(list))
		}

		if list[0].EventType != "book.added" || list[0].Message != `New book added: "Notification Test Book" by Author N (4 copies)` {
			t.Errorf("Notification item = %+v", list[0])
		}
	})

	t.Run("Idempotency prevents duplicate notification insertion", func(t *testing.T) {
		eventID := uuid.New()
		bookID := uuid.New()

		payload, _ := json.Marshal(OutOfStockPayload{
			BookID: bookID.String(),
			Title:  "Idempotent Book",
		})
		env, _ := json.Marshal(EventEnvelope{
			EventID:       eventID.String(),
			EventType:     "inventory.out_of_stock",
			SourceService: "inventory-service",
			SchemaVersion: "1.0",
			Timestamp:     time.Now().UTC(),
			CorrelationID: "corr-202",
			Payload:       payload,
		})

		// Deliver 1st time
		if err := handler.Handle(ctx, string(env)); err != nil {
			t.Fatalf("First Handle() error = %v", err)
		}

		// Deliver 2nd time (duplicate event_id)
		if err := handler.Handle(ctx, string(env)); err != nil {
			t.Fatalf("Second Handle() error = %v", err)
		}

		// Verify processed_events and notifications count
		var count int
		_ = pool.QueryRow(ctx, "SELECT COUNT(*) FROM notifications WHERE event_id = $1", eventID).Scan(&count)
		if count != 1 {
			t.Errorf("Notifications count for event_id = %d, want 1", count)
		}
	})
}

//go:build integration

package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"

	"book-service/internal"
	"book-service/internal/events"
)

func setupTestDB(t *testing.T) *pgxpool.Pool {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://book:book@localhost:5432/book_db?sslmode=disable"
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

	_, _ = pool.Exec(ctx, "TRUNCATE TABLE borrow_records, books RESTART IDENTITY CASCADE;")
	return pool
}

func TestBookServiceIntegration(t *testing.T) {
	pool := setupTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()

	log := zerolog.New(os.Stdout)
	bookRepo := internal.NewBookRepo(pool)
	borrowRepo := internal.NewBorrowRepo(pool)
	mockPub := events.NewMockPublisher()

	h := New(pool, bookRepo, borrowRepo, mockPub, log)
	router := h.Routes()

	t.Run("POST /books creates book in DB and publishes book.added event", func(t *testing.T) {
		mockPub.Reset()
		body := `{"title":"Domain Driven Design","author":"Eric Evans","isbn":"978-0321125217","total_quantity":4}`
		req := httptest.NewRequest(http.MethodPost, "/books", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusCreated {
			t.Fatalf("StatusCode = %d, want %d", rec.Code, http.StatusCreated)
		}

		var created map[string]any
		_ = json.Unmarshal(rec.Body.Bytes(), &created)
		bookIDStr, _ := created["id"].(string)

		if bookIDStr == "" {
			t.Fatal("Response missing book id")
		}

		// Verify event published
		events := mockPub.Events()
		if len(events) != 1 || events[0].EventType != "book.added" {
			t.Errorf("Published events = %+v, want 1 'book.added' event", events)
		}
	})

	t.Run("POST /books with duplicate ISBN returns 409 Conflict", func(t *testing.T) {
		mockPub.Reset()
		body := `{"title":"Duplicate ISBN Book","author":"Author Dup","isbn":"978-0321125217","total_quantity":2}`
		req := httptest.NewRequest(http.MethodPost, "/books", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusConflict {
			t.Errorf("StatusCode = %d, want %d (Conflict)", rec.Code, http.StatusConflict)
		}
	})
}

package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/rs/zerolog"
)

func TestHandlerRoutesWithoutDB(t *testing.T) {
	log := zerolog.New(os.Stdout)
	h := New(nil, nil, nil, nil, log)
	router := h.Routes()

	t.Run("GET /health returns 200 OK", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("StatusCode = %d, want %d", rec.Code, http.StatusOK)
		}

		var got map[string]string
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatalf("Failed to decode body: %v", err)
		}

		if got["status"] != "ok" || got["service"] != "book-service" {
			t.Errorf("Health response = %v, want status=ok and service=book-service", got)
		}
	})

	t.Run("GET /books/{id} with invalid UUID returns 400", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/books/invalid-uuid", nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("StatusCode = %d, want %d", rec.Code, http.StatusBadRequest)
		}

		var got map[string]string
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatalf("Failed to decode body: %v", err)
		}

		if got["error"] != "invalid book id" {
			t.Errorf("Error message = %q, want %q", got["error"], "invalid book id")
		}
	})

	t.Run("POST /books with invalid JSON body returns 400", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/books", strings.NewReader(`{invalid_json`))
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("StatusCode = %d, want %d", rec.Code, http.StatusBadRequest)
		}
	})

	t.Run("POST /books with missing title returns 400", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/books", strings.NewReader(`{"author":"Me","isbn":"123","total_quantity":1}`))
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("StatusCode = %d, want %d", rec.Code, http.StatusBadRequest)
		}
	})
}

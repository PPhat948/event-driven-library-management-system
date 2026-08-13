package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/rs/zerolog"
)

func TestHandlerRoutesWithoutDB(t *testing.T) {
	log := zerolog.New(os.Stdout)
	h := New(nil, log)
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
			t.Fatalf("Failed to decode response body: %v", err)
		}

		if got["status"] != "ok" || got["service"] != "notification-service" {
			t.Errorf("Health response = %v, want status=ok and service=notification-service", got)
		}
	})

	t.Run("GET /notifications/{id} with invalid UUID returns 400 Bad Request", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/notifications/not-a-uuid", nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("StatusCode = %d, want %d", rec.Code, http.StatusBadRequest)
		}

		var got map[string]string
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatalf("Failed to decode response body: %v", err)
		}

		if got["error"] != "invalid notification id" {
			t.Errorf("Error message = %q, want %q", got["error"], "invalid notification id")
		}
	})
}

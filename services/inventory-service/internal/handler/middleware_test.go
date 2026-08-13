package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
)

func TestCorrelationIDMiddleware(t *testing.T) {
	t.Run("preserves existing correlation ID header", func(t *testing.T) {
		existingID := "custom-corr-id-999"

		var capturedID string
		nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			capturedID = GetCorrelationID(r.Context())
			w.WriteHeader(http.StatusOK)
		})

		req := httptest.NewRequest(http.MethodGet, "/inventory", nil)
		req.Header.Set("X-Correlation-ID", existingID)
		rec := httptest.NewRecorder()

		CorrelationID(nextHandler).ServeHTTP(rec, req)

		if capturedID != existingID {
			t.Errorf("GetCorrelationID() = %q, want %q", capturedID, existingID)
		}
		if gotHeader := rec.Header().Get("X-Correlation-ID"); gotHeader != existingID {
			t.Errorf("X-Correlation-ID response header = %q, want %q", gotHeader, existingID)
		}
	})

	t.Run("generates new UUID v4 when header missing", func(t *testing.T) {
		var capturedID string
		nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			capturedID = GetCorrelationID(r.Context())
			w.WriteHeader(http.StatusOK)
		})

		req := httptest.NewRequest(http.MethodGet, "/inventory", nil)
		rec := httptest.NewRecorder()

		CorrelationID(nextHandler).ServeHTTP(rec, req)

		if capturedID == "" {
			t.Fatal("GetCorrelationID() returned empty string")
		}
		if _, err := uuid.Parse(capturedID); err != nil {
			t.Errorf("generated ID %q is not a valid UUID: %v", capturedID, err)
		}
	})
}

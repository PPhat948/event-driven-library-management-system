package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWriteJSON(t *testing.T) {
	rec := httptest.NewRecorder()
	payload := map[string]string{"message": "hello"}

	writeJSON(rec, http.StatusCreated, payload)

	if rec.Code != http.StatusCreated {
		t.Errorf("StatusCode = %d, want %d", rec.Code, http.StatusCreated)
	}

	if gotType := rec.Header().Get("Content-Type"); gotType != "application/json" {
		t.Errorf("Content-Type = %q, want %q", gotType, "application/json")
	}

	var got map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("Failed to unmarshal body: %v", err)
	}

	if got["message"] != "hello" {
		t.Errorf("Body message = %q, want %q", got["message"], "hello")
	}
}

func TestWriteError(t *testing.T) {
	rec := httptest.NewRecorder()

	writeError(rec, http.StatusBadRequest, "invalid request")

	if rec.Code != http.StatusBadRequest {
		t.Errorf("StatusCode = %d, want %d", rec.Code, http.StatusBadRequest)
	}

	var got map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("Failed to unmarshal body: %v", err)
	}

	if got["error"] != "invalid request" {
		t.Errorf("Body error = %q, want %q", got["error"], "invalid request")
	}
}

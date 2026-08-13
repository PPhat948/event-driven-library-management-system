package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWriteJSON(t *testing.T) {
	rec := httptest.NewRecorder()
	payload := map[string]string{"status": "ok"}

	writeJSON(rec, http.StatusOK, payload)

	if rec.Code != http.StatusOK {
		t.Errorf("StatusCode = %d, want %d", rec.Code, http.StatusOK)
	}

	if gotType := rec.Header().Get("Content-Type"); gotType != "application/json" {
		t.Errorf("Content-Type = %q, want %q", gotType, "application/json")
	}

	var got map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("Failed to unmarshal body: %v", err)
	}

	if got["status"] != "ok" {
		t.Errorf("Body status = %q, want %q", got["status"], "ok")
	}
}

func TestWriteError(t *testing.T) {
	rec := httptest.NewRecorder()

	writeError(rec, http.StatusNotFound, "item not found")

	if rec.Code != http.StatusNotFound {
		t.Errorf("StatusCode = %d, want %d", rec.Code, http.StatusNotFound)
	}

	var got map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("Failed to unmarshal body: %v", err)
	}

	if got["error"] != "item not found" {
		t.Errorf("Body error = %q, want %q", got["error"], "item not found")
	}
}

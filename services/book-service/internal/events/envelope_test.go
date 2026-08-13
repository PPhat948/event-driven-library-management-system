package events

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestNewEnvelope(t *testing.T) {
	payload := json.RawMessage(`{"test":"data"}`)
	corrID := "test-corr-id-123"
	eventType := "book.added"

	env := newEnvelope(eventType, corrID, payload)

	if _, err := uuid.Parse(env.EventID); err != nil {
		t.Errorf("EventID %q is not a valid UUID: %v", env.EventID, err)
	}

	if env.EventType != eventType {
		t.Errorf("EventType = %q, want %q", env.EventType, eventType)
	}

	if env.SourceService != "book-service" {
		t.Errorf("SourceService = %q, want %q", env.SourceService, "book-service")
	}

	if env.SchemaVersion != "1.0" {
		t.Errorf("SchemaVersion = %q, want %q", env.SchemaVersion, "1.0")
	}

	if env.CorrelationID != corrID {
		t.Errorf("CorrelationID = %q, want %q", env.CorrelationID, corrID)
	}

	if string(env.Payload) != string(payload) {
		t.Errorf("Payload = %s, want %s", string(env.Payload), string(payload))
	}

	if time.Since(env.Timestamp) > 5*time.Second {
		t.Errorf("Timestamp %v is too old", env.Timestamp)
	}
}

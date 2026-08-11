package events

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type Envelope struct {
	EventID       string          `json:"event_id"`
	EventType     string          `json:"event_type"`
	SourceService string          `json:"source_service"`
	SchemaVersion string          `json:"schema_version"`
	Timestamp     time.Time       `json:"timestamp"`
	CorrelationID string          `json:"correlation_id"`
	Payload       json.RawMessage `json:"payload"`
}

func newEnvelope(eventType, correlationID string, payload json.RawMessage) Envelope {
	return Envelope{
		EventID:       uuid.NewString(),
		EventType:     eventType,
		SourceService: "book-service",
		SchemaVersion: "1.0",
		Timestamp:     time.Now().UTC(),
		CorrelationID: correlationID,
		Payload:       payload,
	}
}

package internal

import (
	"time"

	"github.com/google/uuid"
)

type Notification struct {
	ID            uuid.UUID  `json:"id"`
	EventID       uuid.UUID  `json:"event_id"`
	CorrelationID *string    `json:"correlation_id,omitempty"`
	EventType     string     `json:"event_type"`
	MemberID      *string    `json:"member_id,omitempty"`
	BookID        *uuid.UUID `json:"book_id,omitempty"`
	BookTitle     *string    `json:"book_title,omitempty"`
	Message       string     `json:"message"`
	CreatedAt     time.Time  `json:"created_at"`
}

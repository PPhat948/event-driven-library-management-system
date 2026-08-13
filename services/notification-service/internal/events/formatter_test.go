package events

import (
	"encoding/json"
	"testing"
)

func TestFormatMessage(t *testing.T) {
	tests := []struct {
		name          string
		envelope      EventEnvelope
		wantMsg       string
		wantMemberID  string
		wantBookID    string
		wantBookTitle string
		wantErr       bool
	}{
		{
			name: "book.added event",
			envelope: EventEnvelope{
				EventType: "book.added",
				Payload: rawPayload(BookAddedPayload{
					BookID:        "b-101",
					Title:         "Clean Code",
					Author:        "Robert Martin",
					TotalQuantity: 3,
				}),
			},
			wantMsg:       `New book added: "Clean Code" by Robert Martin (3 copies)`,
			wantBookID:    "b-101",
			wantBookTitle: "Clean Code",
			wantErr:       false,
		},
		{
			name: "book.updated event",
			envelope: EventEnvelope{
				EventType: "book.updated",
				Payload: rawPayload(BookUpdatedPayload{
					BookID: "b-101",
					Title:  "Clean Code 2nd Ed",
					Author: "Robert Martin",
				}),
			},
			wantMsg:       `Book updated: "Clean Code 2nd Ed" by Robert Martin`,
			wantBookID:    "b-101",
			wantBookTitle: "Clean Code 2nd Ed",
			wantErr:       false,
		},
		{
			name: "book.borrowed with member_name",
			envelope: EventEnvelope{
				EventType: "book.borrowed",
				Payload: rawPayload(BookBorrowedPayload{
					BookID:     "b-101",
					BookTitle:  "Clean Code",
					MemberID:   "m-001",
					MemberName: "Alice",
				}),
			},
			wantMsg:       `Alice borrowed "Clean Code"`,
			wantMemberID:  "m-001",
			wantBookID:    "b-101",
			wantBookTitle: "Clean Code",
			wantErr:       false,
		},
		{
			name: "book.borrowed fallback to member_id when member_name empty",
			envelope: EventEnvelope{
				EventType: "book.borrowed",
				Payload: rawPayload(BookBorrowedPayload{
					BookID:    "b-101",
					BookTitle: "Clean Code",
					MemberID:  "m-001",
				}),
			},
			wantMsg:       `m-001 borrowed "Clean Code"`,
			wantMemberID:  "m-001",
			wantBookID:    "b-101",
			wantBookTitle: "Clean Code",
			wantErr:       false,
		},
		{
			name: "book.returned event",
			envelope: EventEnvelope{
				EventType: "book.returned",
				Payload: rawPayload(BookReturnedPayload{
					BookID:    "b-101",
					BookTitle: "Clean Code",
					MemberID:  "m-001",
				}),
			},
			wantMsg:       `Member m-001 returned "Clean Code"`,
			wantMemberID:  "m-001",
			wantBookID:    "b-101",
			wantBookTitle: "Clean Code",
			wantErr:       false,
		},
		{
			name: "book.deleted event",
			envelope: EventEnvelope{
				EventType: "book.deleted",
				Payload: rawPayload(BookDeletedPayload{
					BookID: "b-101",
					Title:  "Clean Code",
				}),
			},
			wantMsg:       `Book removed from catalog: "Clean Code"`,
			wantBookID:    "b-101",
			wantBookTitle: "Clean Code",
			wantErr:       false,
		},
		{
			name: "inventory.low_stock event",
			envelope: EventEnvelope{
				EventType: "inventory.low_stock",
				Payload: rawPayload(LowStockPayload{
					BookID:         "b-101",
					Title:          "Clean Code",
					AvailableCount: 1,
					TotalQuantity:  3,
				}),
			},
			wantMsg:       `⚠️ Low stock alert: "Clean Code" — 1 of 3 copies remaining`,
			wantBookID:    "b-101",
			wantBookTitle: "Clean Code",
			wantErr:       false,
		},
		{
			name: "inventory.out_of_stock event",
			envelope: EventEnvelope{
				EventType: "inventory.out_of_stock",
				Payload: rawPayload(OutOfStockPayload{
					BookID: "b-101",
					Title:  "Clean Code",
				}),
			},
			wantMsg:       `🚨 Out of stock: "Clean Code" — no copies available`,
			wantBookID:    "b-101",
			wantBookTitle: "Clean Code",
			wantErr:       false,
		},
		{
			name: "unknown event type",
			envelope: EventEnvelope{
				EventType: "unknown.event",
				Payload:   json.RawMessage(`{}`),
			},
			wantErr: true,
		},
		{
			name: "corrupted json payload",
			envelope: EventEnvelope{
				EventType: "book.added",
				Payload:   json.RawMessage(`{invalid_json`),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg, memberID, bookID, bookTitle, err := FormatMessage(tt.envelope)
			if (err != nil) != tt.wantErr {
				t.Errorf("FormatMessage() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}

			if msg != tt.wantMsg {
				t.Errorf("FormatMessage() msg = %q, want %q", msg, tt.wantMsg)
			}
			if memberID != tt.wantMemberID {
				t.Errorf("FormatMessage() memberID = %q, want %q", memberID, tt.wantMemberID)
			}
			if bookID != tt.wantBookID {
				t.Errorf("FormatMessage() bookID = %q, want %q", bookID, tt.wantBookID)
			}
			if bookTitle != tt.wantBookTitle {
				t.Errorf("FormatMessage() bookTitle = %q, want %q", bookTitle, tt.wantBookTitle)
			}
		})
	}
}

func rawPayload(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return json.RawMessage(b)
}

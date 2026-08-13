package events

import (
	"encoding/json"
	"fmt"
)

// FormatMessage converts an EventEnvelope into a human-readable notification message and metadata.
func FormatMessage(env EventEnvelope) (msg, memberID, bookID, bookTitle string, err error) {
	switch env.EventType {
	case "book.added":
		var p BookAddedPayload
		if err = json.Unmarshal(env.Payload, &p); err != nil {
			return
		}
		msg = fmt.Sprintf("New book added: \"%s\" by %s (%d copies)", p.Title, p.Author, p.TotalQuantity)
		bookID, bookTitle = p.BookID, p.Title

	case "book.updated":
		var p BookUpdatedPayload
		if err = json.Unmarshal(env.Payload, &p); err != nil {
			return
		}
		msg = fmt.Sprintf("Book updated: \"%s\" by %s", p.Title, p.Author)
		bookID, bookTitle = p.BookID, p.Title

	case "book.borrowed":
		var p BookBorrowedPayload
		if err = json.Unmarshal(env.Payload, &p); err != nil {
			return
		}
		name := p.MemberName
		if name == "" {
			name = p.MemberID
		}
		msg = fmt.Sprintf("%s borrowed \"%s\"", name, p.BookTitle)
		memberID, bookID, bookTitle = p.MemberID, p.BookID, p.BookTitle

	case "book.returned":
		var p BookReturnedPayload
		if err = json.Unmarshal(env.Payload, &p); err != nil {
			return
		}
		msg = fmt.Sprintf("Member %s returned \"%s\"", p.MemberID, p.BookTitle)
		memberID, bookID, bookTitle = p.MemberID, p.BookID, p.BookTitle

	case "book.deleted":
		var p BookDeletedPayload
		if err = json.Unmarshal(env.Payload, &p); err != nil {
			return
		}
		msg = fmt.Sprintf("Book removed from catalog: \"%s\"", p.Title)
		bookID, bookTitle = p.BookID, p.Title

	case "inventory.low_stock":
		var p LowStockPayload
		if err = json.Unmarshal(env.Payload, &p); err != nil {
			return
		}
		msg = fmt.Sprintf("Low stock alert: \"%s\" — %d of %d copies remaining",
			p.Title, p.AvailableCount, p.TotalQuantity)
		bookID, bookTitle = p.BookID, p.Title

	case "inventory.out_of_stock":
		var p OutOfStockPayload
		if err = json.Unmarshal(env.Payload, &p); err != nil {
			return
		}
		msg = fmt.Sprintf("Out of stock: \"%s\" — no copies available", p.Title)
		bookID, bookTitle = p.BookID, p.Title

	default:
		err = fmt.Errorf("unknown event type: %s", env.EventType)
	}
	return
}

package events

// ── Incoming payloads (from book-service) ────────────────────────────────────
// These mirror the structs in book-service/internal/events/payloads.go.
// Only the fields inventory-service actually needs are included.

type BookAddedPayload struct {
	BookID         string `json:"book_id"`
	Title          string `json:"title"`
	Author         string `json:"author"`
	TotalQuantity  int    `json:"total_quantity"`
	AvailableCount int    `json:"available_count"`
}

type BookUpdatedPayload struct {
	BookID         string `json:"book_id"`
	Title          string `json:"title"`
	Author         string `json:"author"`
	TotalQuantity  int    `json:"total_quantity"`
	AvailableCount int    `json:"available_count"`
}

type BookBorrowedPayload struct {
	BookID              string `json:"book_id"`
	BookTitle           string `json:"book_title"`
	AvailableCountAfter int    `json:"available_count_after"`
}

type BookReturnedPayload struct {
	BookID              string `json:"book_id"`
	BookTitle           string `json:"book_title"`
	AvailableCountAfter int    `json:"available_count_after"`
}

type BookDeletedPayload struct {
	BookID string `json:"book_id"`
	Title  string `json:"title"`
}

// ── Outgoing payloads (published by inventory-service) ───────────────────────

type LowStockPayload struct {
	BookID         string `json:"book_id"`
	Title          string `json:"title"`
	AvailableCount int    `json:"available_count"`
	TotalQuantity  int    `json:"total_quantity"`
}

type OutOfStockPayload struct {
	BookID string `json:"book_id"`
	Title  string `json:"title"`
}

package events

type BookAddedPayload struct {
	BookID        string `json:"book_id"`
	Title         string `json:"title"`
	Author        string `json:"author"`
	TotalQuantity int    `json:"total_quantity"`
}

type BookUpdatedPayload struct {
	BookID string `json:"book_id"`
	Title  string `json:"title"`
	Author string `json:"author"`
}

type BookBorrowedPayload struct {
	BookID     string `json:"book_id"`
	BookTitle  string `json:"book_title"`
	MemberID   string `json:"member_id"`
	MemberName string `json:"member_name,omitempty"`
}

type BookReturnedPayload struct {
	BookID    string `json:"book_id"`
	BookTitle string `json:"book_title"`
	MemberID  string `json:"member_id"`
}

type BookDeletedPayload struct {
	BookID string `json:"book_id"`
	Title  string `json:"title"`
}

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

package events

import "time"

type BookAdded struct {
	BookID         string `json:"book_id"`
	Title          string `json:"title"`
	Author         string `json:"author"`
	ISBN           string `json:"isbn"`
	TotalQuantity  int    `json:"total_quantity"`
	AvailableCount int    `json:"available_count"`
}

type BookUpdated struct {
	BookID         string `json:"book_id"`
	Title          string `json:"title"`
	Author         string `json:"author"`
	TotalQuantity  int    `json:"total_quantity"`
	AvailableCount int    `json:"available_count"`
}

type BookBorrowed struct {
	BookID              string    `json:"book_id"`
	BookTitle           string    `json:"book_title"`
	MemberID            string    `json:"member_id"`
	MemberName          string    `json:"member_name,omitempty"`
	DueDate             time.Time `json:"due_date"`
	AvailableCountAfter int       `json:"available_count_after"`
}

type BookReturned struct {
	BookID              string    `json:"book_id"`
	BookTitle           string    `json:"book_title"`
	MemberID            string    `json:"member_id"`
	BorrowRecordID      string    `json:"borrow_record_id"`
	ReturnedAt          time.Time `json:"returned_at"`
	AvailableCountAfter int       `json:"available_count_after"`
}

type BookDeleted struct {
	BookID string `json:"book_id"`
	Title  string `json:"title"`
}

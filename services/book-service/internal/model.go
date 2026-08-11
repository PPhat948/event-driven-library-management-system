package internal

import (
	"time"

	"github.com/google/uuid"
)

type Book struct {
	ID             uuid.UUID `json:"id"`
	Title          string    `json:"title"`
	Author         string    `json:"author"`
	ISBN           string    `json:"isbn"`
	TotalQuantity  int       `json:"total_quantity"`
	AvailableCount int       `json:"available_count"`
	Status         string    `json:"status"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type BorrowRecord struct {
	ID         uuid.UUID  `json:"id"`
	BookID     uuid.UUID  `json:"book_id"`
	MemberID   string     `json:"member_id"`
	MemberName string     `json:"member_name,omitempty"`
	BorrowedAt time.Time  `json:"borrowed_at"`
	DueDate    time.Time  `json:"due_date"`
	ReturnedAt *time.Time `json:"returned_at,omitempty"`
	Status     string     `json:"status"`
}

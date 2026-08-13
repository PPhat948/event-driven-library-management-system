package internal

import (
	"time"

	"github.com/google/uuid"
)

type InventoryItem struct {
	ID             uuid.UUID `json:"id"`
	BookID         uuid.UUID `json:"book_id"`
	Title          string    `json:"title"`
	Author         string    `json:"author"`
	TotalQuantity  int       `json:"total_quantity"`
	AvailableCount int       `json:"available_count"`
	BorrowedCount  int       `json:"borrowed_count"`
	Status         string    `json:"status"` // "available" | "low_stock" | "out_of_stock" | "removed"
	UpdatedAt      time.Time `json:"updated_at"`
}

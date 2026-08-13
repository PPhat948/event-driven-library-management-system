package internal

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type InventoryRepo struct {
	pool *pgxpool.Pool
}

func NewInventoryRepo(pool *pgxpool.Pool) *InventoryRepo {
	return &InventoryRepo{pool: pool}
}

func scanItem(row interface {
	Scan(...any) error
}, item *InventoryItem) error {
	return row.Scan(
		&item.ID, &item.BookID, &item.Title, &item.Author,
		&item.TotalQuantity, &item.AvailableCount, &item.BorrowedCount,
		&item.Status, &item.UpdatedAt,
	)
}

func (r *InventoryRepo) List(ctx context.Context) ([]*InventoryItem, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, book_id, title, author, total_quantity,
		       available_count, borrowed_count, status, updated_at
		FROM inventory
		WHERE status != 'removed'
		ORDER BY updated_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []*InventoryItem
	for rows.Next() {
		item := &InventoryItem{}
		if err := scanItem(rows, item); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *InventoryRepo) GetByBookID(ctx context.Context, bookID uuid.UUID) (*InventoryItem, error) {
	item := &InventoryItem{}
	err := scanItem(r.pool.QueryRow(ctx, `
		SELECT id, book_id, title, author, total_quantity,
		       available_count, borrowed_count, status, updated_at
		FROM inventory
		WHERE book_id = $1
	`, bookID), item)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return item, err
}

func (r *InventoryRepo) ListByStatus(ctx context.Context, status string) ([]*InventoryItem, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, book_id, title, author, total_quantity,
		       available_count, borrowed_count, status, updated_at
		FROM inventory
		WHERE status = $1
		ORDER BY available_count ASC, updated_at DESC
	`, status)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []*InventoryItem
	for rows.Next() {
		item := &InventoryItem{}
		if err := scanItem(rows, item); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// ─── transactional write methods ──────────────────────────────────────────────────────

func (r *InventoryRepo) IsEventProcessed(ctx context.Context, tx pgx.Tx, eventID uuid.UUID) (bool, error) {
	var exists bool
	err := tx.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM processed_events WHERE event_id = $1)`,
		eventID,
	).Scan(&exists)
	return exists, err
}

func (r *InventoryRepo) MarkEventProcessed(ctx context.Context, tx pgx.Tx, eventID uuid.UUID, eventType string) error {
	_, err := tx.Exec(ctx,
		`INSERT INTO processed_events (event_id, event_type) VALUES ($1, $2)`,
		eventID, eventType,
	)
	return err
}

// UpsertFromBookAdded inserts a new inventory row. ON CONFLICT DO NOTHING makes it idempotent.
func (r *InventoryRepo) UpsertFromBookAdded(ctx context.Context, tx pgx.Tx, bookID uuid.UUID, title, author string, totalQty, availCount int) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO inventory
			(book_id, title, author, total_quantity, available_count, borrowed_count, status)
		VALUES ($1, $2, $3, $4, $5, 0, 'available')
		ON CONFLICT (book_id) DO NOTHING
	`, bookID, title, author, totalQty, availCount)
	return err
}

// UpdateFromBookUpdated syncs title, author, and quantities. Returns ErrNotFound if book not yet tracked.
func (r *InventoryRepo) UpdateFromBookUpdated(ctx context.Context, tx pgx.Tx, bookID uuid.UUID, title, author string, totalQty, availCount, threshold int) (*InventoryItem, error) {
	item := &InventoryItem{}
	err := scanItem(tx.QueryRow(ctx, `
		UPDATE inventory
		SET title           = $1,
		    author          = $2,
		    total_quantity  = $3,
		    available_count = $4,
		    borrowed_count  = $3 - $4,
		    status          = CASE
		                        WHEN $4 = 0   THEN 'out_of_stock'
		                        WHEN $4 <= $5 THEN 'low_stock'
		                        ELSE 'available'
		                      END,
		    updated_at = now()
		WHERE book_id = $6 AND status != 'removed'
		RETURNING id, book_id, title, author, total_quantity,
		          available_count, borrowed_count, status, updated_at
	`, title, author, totalQty, availCount, threshold, bookID), item)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return item, err
}

// UpdateAvailableCount sets available_count to the authoritative value from the payload.
// Using the payload value (not ±1) keeps inventory convergent even if events arrive out of order.
func (r *InventoryRepo) UpdateAvailableCount(ctx context.Context, tx pgx.Tx, bookID uuid.UUID, availCount, threshold int) (*InventoryItem, error) {
	item := &InventoryItem{}
	err := scanItem(tx.QueryRow(ctx, `
		UPDATE inventory
		SET available_count = $1,
		    borrowed_count  = total_quantity - $1,
		    status          = CASE
		                        WHEN $1 = 0   THEN 'out_of_stock'
		                        WHEN $1 <= $2 THEN 'low_stock'
		                        ELSE 'available'
		                      END,
		    updated_at = now()
		WHERE book_id = $3 AND status != 'removed'
		RETURNING id, book_id, title, author, total_quantity,
		          available_count, borrowed_count, status, updated_at
	`, availCount, threshold, bookID), item)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return item, err
}

func (r *InventoryRepo) MarkRemoved(ctx context.Context, tx pgx.Tx, bookID uuid.UUID) error {
	_, err := tx.Exec(ctx, `
		UPDATE inventory
		SET status = 'removed', updated_at = now()
		WHERE book_id = $1
	`, bookID)
	return err
}

package internal

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNoActiveBorrow = errors.New("no active borrow record")

type BorrowRepo struct {
	pool *pgxpool.Pool
}

func NewBorrowRepo(pool *pgxpool.Pool) *BorrowRepo {
	return &BorrowRepo{pool: pool}
}

// scanRecord scans a borrow_records row. member_name is nullable so we use *string.
func scanRecord(row interface {
	Scan(...any) error
}, rec *BorrowRecord) error {
	var name *string
	err := row.Scan(
		&rec.ID, &rec.BookID, &rec.MemberID, &name,
		&rec.BorrowedAt, &rec.DueDate, &rec.ReturnedAt, &rec.Status,
	)
	if err != nil {
		return err
	}
	if name != nil {
		rec.MemberName = *name
	}
	return nil
}

func (r *BorrowRepo) Create(ctx context.Context, tx pgx.Tx, bookID uuid.UUID, memberID, memberName string, dueDate time.Time) (*BorrowRecord, error) {
	rec := &BorrowRecord{}
	row := tx.QueryRow(ctx, `
		INSERT INTO borrow_records (book_id, member_id, member_name, due_date)
		VALUES ($1, $2, $3, $4)
		RETURNING id, book_id, member_id, member_name, borrowed_at, due_date, returned_at, status
	`, bookID, memberID, memberName, dueDate)
	if err := scanRecord(row, rec); err != nil {
		return nil, err
	}
	return rec, nil
}

func (r *BorrowRepo) GetActive(ctx context.Context, bookID uuid.UUID, memberID string) (*BorrowRecord, error) {
	rec := &BorrowRecord{}
	row := r.pool.QueryRow(ctx, `
		SELECT id, book_id, member_id, member_name, borrowed_at, due_date, returned_at, status
		FROM borrow_records
		WHERE book_id=$1 AND member_id=$2 AND status='borrowed'
	`, bookID, memberID)
	if err := scanRecord(row, rec); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNoActiveBorrow
		}
		return nil, err
	}
	return rec, nil
}

func (r *BorrowRepo) ListActiveByBookID(ctx context.Context, bookID uuid.UUID) ([]*BorrowRecord, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, book_id, member_id, member_name, borrowed_at, due_date, returned_at, status
		FROM borrow_records
		WHERE book_id=$1 AND status='borrowed'
		ORDER BY borrowed_at DESC
	`, bookID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []*BorrowRecord
	for rows.Next() {
		rec := &BorrowRecord{}
		if err := scanRecord(rows, rec); err != nil {
			return nil, err
		}
		records = append(records, rec)
	}
	return records, rows.Err()
}

func (r *BorrowRepo) MarkReturned(ctx context.Context, tx pgx.Tx, id uuid.UUID) (*BorrowRecord, error) {
	rec := &BorrowRecord{}
	row := tx.QueryRow(ctx, `
		UPDATE borrow_records
		SET status='returned', returned_at=now()
		WHERE id=$1
		RETURNING id, book_id, member_id, member_name, borrowed_at, due_date, returned_at, status
	`, id)
	if err := scanRecord(row, rec); err != nil {
		return nil, err
	}
	return rec, nil
}

package internal

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound          = errors.New("not found")
	ErrNoAvailableCopies = errors.New("no available copies")
	ErrDuplicateISBN     = errors.New("isbn already exists")
)

type BookRepo struct {
	pool *pgxpool.Pool
}

func NewBookRepo(pool *pgxpool.Pool) *BookRepo {
	return &BookRepo{pool: pool}
}

func (r *BookRepo) Create(ctx context.Context, title, author, isbn string, qty int) (*Book, error) {
	b := &Book{}
	err := r.pool.QueryRow(ctx, `
		INSERT INTO books (title, author, isbn, total_quantity, available_count)
		VALUES ($1, $2, $3, $4, $4)
		RETURNING id, title, author, isbn, total_quantity, available_count, status, created_at, updated_at
	`, title, author, isbn, qty).Scan(
		&b.ID, &b.Title, &b.Author, &b.ISBN,
		&b.TotalQuantity, &b.AvailableCount, &b.Status,
		&b.CreatedAt, &b.UpdatedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, ErrDuplicateISBN
		}
		return nil, err
	}
	return b, nil
}

func (r *BookRepo) GetByID(ctx context.Context, id uuid.UUID) (*Book, error) {
	b := &Book{}
	err := r.pool.QueryRow(ctx, `
		SELECT id, title, author, isbn, total_quantity, available_count, status, created_at, updated_at
		FROM books WHERE id = $1 AND status != 'deleted'
	`, id).Scan(
		&b.ID, &b.Title, &b.Author, &b.ISBN,
		&b.TotalQuantity, &b.AvailableCount, &b.Status,
		&b.CreatedAt, &b.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return b, err
}

func (r *BookRepo) List(ctx context.Context) ([]*Book, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, title, author, isbn, total_quantity, available_count, status, created_at, updated_at
		FROM books WHERE status != 'deleted'
		ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var books []*Book
	for rows.Next() {
		b := &Book{}
		if err := rows.Scan(
			&b.ID, &b.Title, &b.Author, &b.ISBN,
			&b.TotalQuantity, &b.AvailableCount, &b.Status,
			&b.CreatedAt, &b.UpdatedAt,
		); err != nil {
			return nil, err
		}
		books = append(books, b)
	}
	return books, rows.Err()
}

type UpdateBookInput struct {
	Title         *string
	Author        *string
	TotalQuantity *int
}

func (r *BookRepo) Update(ctx context.Context, id uuid.UUID, in UpdateBookInput) (*Book, error) {
	b, err := r.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if in.Title != nil {
		b.Title = *in.Title
	}
	if in.Author != nil {
		b.Author = *in.Author
	}
	if in.TotalQuantity != nil {
		diff := *in.TotalQuantity - b.TotalQuantity
		b.TotalQuantity = *in.TotalQuantity
		b.AvailableCount += diff
		if b.AvailableCount < 0 {
			b.AvailableCount = 0
		}
	}

	updated := &Book{}
	err = r.pool.QueryRow(ctx, `
		UPDATE books
		SET title=$1, author=$2, total_quantity=$3, available_count=$4, updated_at=now()
		WHERE id=$5
		RETURNING id, title, author, isbn, total_quantity, available_count, status, created_at, updated_at
	`, b.Title, b.Author, b.TotalQuantity, b.AvailableCount, id).Scan(
		&updated.ID, &updated.Title, &updated.Author, &updated.ISBN,
		&updated.TotalQuantity, &updated.AvailableCount, &updated.Status,
		&updated.CreatedAt, &updated.UpdatedAt,
	)
	return updated, err
}

func (r *BookRepo) SoftDelete(ctx context.Context, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE books SET status='deleted', updated_at=now() WHERE id=$1 AND status='active'`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// DecrementAvailable does the UPDATE and RETURNING in one shot.
// If available_count is already 0, the WHERE clause matches nothing → ErrNoAvailableCopies.
func (r *BookRepo) DecrementAvailable(ctx context.Context, tx pgx.Tx, id uuid.UUID) (*Book, error) {
	b := &Book{}
	err := tx.QueryRow(ctx, `
		UPDATE books
		SET available_count = available_count - 1, updated_at=now()
		WHERE id=$1 AND available_count > 0 AND status='active'
		RETURNING id, title, author, isbn, total_quantity, available_count, status, created_at, updated_at
	`, id).Scan(
		&b.ID, &b.Title, &b.Author, &b.ISBN,
		&b.TotalQuantity, &b.AvailableCount, &b.Status,
		&b.CreatedAt, &b.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNoAvailableCopies
	}
	return b, err
}

func (r *BookRepo) IncrementAvailable(ctx context.Context, tx pgx.Tx, id uuid.UUID) (*Book, error) {
	b := &Book{}
	err := tx.QueryRow(ctx, `
		UPDATE books
		SET available_count = available_count + 1, updated_at=now()
		WHERE id=$1
		RETURNING id, title, author, isbn, total_quantity, available_count, status, created_at, updated_at
	`, id).Scan(
		&b.ID, &b.Title, &b.Author, &b.ISBN,
		&b.TotalQuantity, &b.AvailableCount, &b.Status,
		&b.CreatedAt, &b.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return b, err
}

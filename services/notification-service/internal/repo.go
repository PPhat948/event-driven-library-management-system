package internal

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type NotificationRepo struct {
	pool *pgxpool.Pool
}

func NewNotificationRepo(pool *pgxpool.Pool) *NotificationRepo {
	return &NotificationRepo{pool: pool}
}

func scanNotification(row interface {
	Scan(...any) error
}, n *Notification) error {
	var correlationID *string
	var memberID *string
	var bookID *uuid.UUID
	var bookTitle *string

	err := row.Scan(
		&n.ID, &n.EventID, &correlationID, &n.EventType, &memberID,
		&bookID, &bookTitle, &n.Message, &n.CreatedAt,
	)
	if err != nil {
		return err
	}
	n.CorrelationID = correlationID
	n.MemberID = memberID
	n.BookID = bookID
	n.BookTitle = bookTitle
	return nil
}

func (r *NotificationRepo) List(ctx context.Context, limit, offset int) ([]*Notification, error) {
	if limit <= 0 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	rows, err := r.pool.Query(ctx, `
		SELECT id, event_id, correlation_id, event_type, member_id, book_id, book_title, message, created_at
		FROM notifications
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []*Notification
	for rows.Next() {
		n := &Notification{}
		if err := scanNotification(rows, n); err != nil {
			return nil, err
		}
		list = append(list, n)
	}
	return list, rows.Err()
}

func (r *NotificationRepo) Count(ctx context.Context) (int, error) {
	var total int
	err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM notifications`).Scan(&total)
	return total, err
}

func (r *NotificationRepo) GetByID(ctx context.Context, id uuid.UUID) (*Notification, error) {
	n := &Notification{}
	err := scanNotification(r.pool.QueryRow(ctx, `
		SELECT id, event_id, correlation_id, event_type, member_id, book_id, book_title, message, created_at
		FROM notifications
		WHERE id = $1
	`, id), n)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return n, err
}

// ─── transactional write methods ──────────────────────────────────────────────────────

func (r *NotificationRepo) IsEventProcessed(ctx context.Context, tx pgx.Tx, eventID uuid.UUID) (bool, error) {
	var exists bool
	err := tx.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM processed_events WHERE event_id = $1)`,
		eventID,
	).Scan(&exists)
	return exists, err
}

func (r *NotificationRepo) MarkEventProcessed(ctx context.Context, tx pgx.Tx, eventID uuid.UUID, eventType string) error {
	_, err := tx.Exec(ctx,
		`INSERT INTO processed_events (event_id, event_type) VALUES ($1, $2)`,
		eventID, eventType,
	)
	return err
}

func (r *NotificationRepo) Insert(ctx context.Context, tx pgx.Tx, eventID uuid.UUID, correlationID, eventType, memberID, bookID, bookTitle, message string) error {
	var corrID *string
	if correlationID != "" {
		corrID = &correlationID
	}
	var mID *string
	if memberID != "" {
		mID = &memberID
	}
	var bID *uuid.UUID
	if bookID != "" {
		if id, err := uuid.Parse(bookID); err == nil {
			bID = &id
		}
	}
	var bTitle *string
	if bookTitle != "" {
		bTitle = &bookTitle
	}

	_, err := tx.Exec(ctx, `
		INSERT INTO notifications (event_id, correlation_id, event_type, member_id, book_id, book_title, message)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, eventID, corrID, eventType, mID, bID, bTitle, message)
	return err
}

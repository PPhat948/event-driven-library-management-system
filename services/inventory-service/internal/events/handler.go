package events

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"

	"inventory-service/internal"
)

type Handler struct {
	db                *pgxpool.Pool
	repo              *internal.InventoryRepo
	pub               EventPublisher
	lowStockThreshold int
	log               zerolog.Logger
}

func NewHandler(db *pgxpool.Pool, repo *internal.InventoryRepo, pub EventPublisher, threshold int, log zerolog.Logger) *Handler {
	return &Handler{
		db:                db,
		repo:              repo,
		pub:               pub,
		lowStockThreshold: threshold,
		log:               log,
	}
}

// Handle decodes the SQS message body and dispatches by event_type.
// Bad JSON and unknown types return nil (delete from SQS). DB errors return an error (retry).
func (h *Handler) Handle(ctx context.Context, body string) error {
	var env EventEnvelope
	if err := json.Unmarshal([]byte(body), &env); err != nil {
		h.log.Error().Err(err).Str("body", body).Msg("unmarshal envelope failed — skipping")
		return nil
	}

	h.log.Info().
		Str("event_id", env.EventID).
		Str("event_type", env.EventType).
		Str("correlation_id", env.CorrelationID).
		Msg("received event")

	switch env.EventType {
	case "book.added":
		return h.handleBookAdded(ctx, env)
	case "book.updated":
		return h.handleBookUpdated(ctx, env)
	case "book.borrowed":
		return h.handleBookBorrowed(ctx, env)
	case "book.returned":
		return h.handleBookReturned(ctx, env)
	case "book.deleted":
		return h.handleBookDeleted(ctx, env)
	default:
		h.log.Warn().Str("event_type", env.EventType).Msg("unknown event type — skipping")
		return nil
	}
}

// ─── book.added ─────────────────────────────────────────────────────────────────

func (h *Handler) handleBookAdded(ctx context.Context, env EventEnvelope) error {
	var p BookAddedPayload
	if err := json.Unmarshal(env.Payload, &p); err != nil {
		h.log.Error().Err(err).Msg("unmarshal book.added payload — skipping")
		return nil
	}
	bookID, err := uuid.Parse(p.BookID)
	if err != nil {
		h.log.Error().Err(err).Str("book_id", p.BookID).Msg("invalid book_id — skipping")
		return nil
	}

	tx, err := h.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if done, err := h.idempotencyCheck(ctx, tx, env); err != nil || done {
		return err
	}

	if err := h.repo.UpsertFromBookAdded(ctx, tx, bookID, p.Title, p.Author, p.TotalQuantity, p.AvailableCount); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// ─── book.updated ───────────────────────────────────────────────────────────────

func (h *Handler) handleBookUpdated(ctx context.Context, env EventEnvelope) error {
	var p BookUpdatedPayload
	if err := json.Unmarshal(env.Payload, &p); err != nil {
		h.log.Error().Err(err).Msg("unmarshal book.updated payload — skipping")
		return nil
	}
	bookID, err := uuid.Parse(p.BookID)
	if err != nil {
		h.log.Error().Err(err).Str("book_id", p.BookID).Msg("invalid book_id — skipping")
		return nil
	}

	tx, err := h.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if done, err := h.idempotencyCheck(ctx, tx, env); err != nil || done {
		return err
	}

	item, err := h.repo.UpdateFromBookUpdated(ctx, tx, bookID, p.Title, p.Author, p.TotalQuantity, p.AvailableCount, h.lowStockThreshold)
	if errors.Is(err, internal.ErrNotFound) {
		h.log.Warn().Str("book_id", p.BookID).Msg("book not in inventory yet — skipping update")
		return tx.Commit(ctx)
	}
	if err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return err
	}

	h.publishStockAlert(ctx, env.CorrelationID, item)
	return nil
}

// ─── book.borrowed ──────────────────────────────────────────────────────────────

func (h *Handler) handleBookBorrowed(ctx context.Context, env EventEnvelope) error {
	var p BookBorrowedPayload
	if err := json.Unmarshal(env.Payload, &p); err != nil {
		h.log.Error().Err(err).Msg("unmarshal book.borrowed payload — skipping")
		return nil
	}
	bookID, err := uuid.Parse(p.BookID)
	if err != nil {
		h.log.Error().Err(err).Str("book_id", p.BookID).Msg("invalid book_id — skipping")
		return nil
	}

	tx, err := h.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if done, err := h.idempotencyCheck(ctx, tx, env); err != nil || done {
		return err
	}

	item, err := h.repo.UpdateAvailableCount(ctx, tx, bookID, p.AvailableCountAfter, h.lowStockThreshold)
	if errors.Is(err, internal.ErrNotFound) {
		h.log.Warn().Str("book_id", p.BookID).Msg("book not in inventory yet — skipping borrow event")
		return tx.Commit(ctx)
	}
	if err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return err
	}

	h.publishStockAlert(ctx, env.CorrelationID, item)
	return nil
}

// ─── book.returned ──────────────────────────────────────────────────────────────

func (h *Handler) handleBookReturned(ctx context.Context, env EventEnvelope) error {
	var p BookReturnedPayload
	if err := json.Unmarshal(env.Payload, &p); err != nil {
		h.log.Error().Err(err).Msg("unmarshal book.returned payload — skipping")
		return nil
	}
	bookID, err := uuid.Parse(p.BookID)
	if err != nil {
		h.log.Error().Err(err).Str("book_id", p.BookID).Msg("invalid book_id — skipping")
		return nil
	}

	tx, err := h.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if done, err := h.idempotencyCheck(ctx, tx, env); err != nil || done {
		return err
	}

	item, err := h.repo.UpdateAvailableCount(ctx, tx, bookID, p.AvailableCountAfter, h.lowStockThreshold)
	if errors.Is(err, internal.ErrNotFound) {
		h.log.Warn().Str("book_id", p.BookID).Msg("book not in inventory yet — skipping return event")
		return tx.Commit(ctx)
	}
	if err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return err
	}

	h.publishStockAlert(ctx, env.CorrelationID, item)
	return nil
}

// ─── book.deleted ───────────────────────────────────────────────────────────────

func (h *Handler) handleBookDeleted(ctx context.Context, env EventEnvelope) error {
	var p BookDeletedPayload
	if err := json.Unmarshal(env.Payload, &p); err != nil {
		h.log.Error().Err(err).Msg("unmarshal book.deleted payload — skipping")
		return nil
	}
	bookID, err := uuid.Parse(p.BookID)
	if err != nil {
		h.log.Error().Err(err).Str("book_id", p.BookID).Msg("invalid book_id — skipping")
		return nil
	}

	tx, err := h.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if done, err := h.idempotencyCheck(ctx, tx, env); err != nil || done {
		return err
	}

	if err := h.repo.MarkRemoved(ctx, tx, bookID); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// ─── helpers ────────────────────────────────────────────────────────────────────

// idempotencyCheck returns (true, nil) if the event was already processed — caller should return.
// Inserts into processed_events inside tx so the check and the mutation are atomic.
func (h *Handler) idempotencyCheck(ctx context.Context, tx pgx.Tx, env EventEnvelope) (alreadyDone bool, err error) {
	eventID, parseErr := uuid.Parse(env.EventID)
	if parseErr != nil {
		h.log.Error().Err(parseErr).Str("event_id", env.EventID).Msg("invalid event_id — skipping")
		// Commit empty transaction and signal done so caller skips processing.
		return true, tx.Commit(ctx)
	}

	done, err := h.repo.IsEventProcessed(ctx, tx, eventID)
	if err != nil {
		return false, err
	}
	if done {
		h.log.Debug().Str("event_id", env.EventID).Msg("duplicate event — skipping")
		return true, tx.Commit(ctx)
	}

	// Insert into processed_events now, inside the same transaction as the
	// actual inventory mutation — atomicity guarantees no double-processing.
	if err := h.repo.MarkEventProcessed(ctx, tx, eventID, env.EventType); err != nil {
		return false, err
	}

	return false, nil
}

// publishStockAlert publishes low_stock or out_of_stock after commit. Best-effort: never rolls back.
func (h *Handler) publishStockAlert(ctx context.Context, corrID string, item *internal.InventoryItem) {
	switch item.Status {
	case "out_of_stock":
		if err := h.pub.Publish(ctx, "inventory.out_of_stock", corrID, OutOfStockPayload{
			BookID: item.BookID.String(),
			Title:  item.Title,
		}); err != nil {
			h.log.Error().Err(err).Str("book_id", item.BookID.String()).Msg("publish out_of_stock failed")
		}
	case "low_stock":
		if err := h.pub.Publish(ctx, "inventory.low_stock", corrID, LowStockPayload{
			BookID:         item.BookID.String(),
			Title:          item.Title,
			AvailableCount: item.AvailableCount,
			TotalQuantity:  item.TotalQuantity,
		}); err != nil {
			h.log.Error().Err(err).Str("book_id", item.BookID.String()).Msg("publish low_stock failed")
		}
	}
}

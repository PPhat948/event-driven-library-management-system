package events

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"

	"notification-service/internal"
)

type Handler struct {
	db   *pgxpool.Pool
	repo *internal.NotificationRepo
	log  zerolog.Logger
}

func NewHandler(db *pgxpool.Pool, repo *internal.NotificationRepo, log zerolog.Logger) *Handler {
	return &Handler{
		db:   db,
		repo: repo,
		log:  log,
	}
}

// Handle decodes the SQS message body and processes the notification event.
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

	return h.handleEvent(ctx, env)
}

func (h *Handler) handleEvent(ctx context.Context, env EventEnvelope) error {
	msg, memberID, bookID, bookTitle, err := FormatMessage(env)
	if err != nil {
		h.log.Warn().Str("event_type", env.EventType).Msg("unknown or unparseable event — skipping")
		return nil
	}

	tx, err := h.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	eventID, done, err := h.idempotencyCheck(ctx, tx, env)
	if err != nil || done {
		return err
	}

	if err := h.repo.Insert(ctx, tx, eventID, env.CorrelationID, env.EventType, memberID, bookID, bookTitle, msg); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// ─── helpers ────────────────────────────────────────────────────────────────────

// idempotencyCheck returns (eventID, true, nil) if already processed — caller should return.
func (h *Handler) idempotencyCheck(ctx context.Context, tx pgx.Tx, env EventEnvelope) (uuid.UUID, bool, error) {
	eventID, parseErr := uuid.Parse(env.EventID)
	if parseErr != nil {
		h.log.Error().Err(parseErr).Str("event_id", env.EventID).Msg("invalid event_id — skipping")
		return uuid.Nil, true, tx.Commit(ctx)
	}

	done, err := h.repo.IsEventProcessed(ctx, tx, eventID)
	if err != nil {
		return eventID, false, err
	}
	if done {
		h.log.Debug().Str("event_id", env.EventID).Msg("duplicate event — skipping")
		return eventID, true, tx.Commit(ctx)
	}

	if err := h.repo.MarkEventProcessed(ctx, tx, eventID, env.EventType); err != nil {
		return eventID, false, err
	}

	return eventID, false, nil
}

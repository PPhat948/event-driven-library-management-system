package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"notification-service/internal"
)

type Handler struct {
	repo *internal.NotificationRepo
	log  zerolog.Logger
}

func New(repo *internal.NotificationRepo, log zerolog.Logger) *Handler {
	return &Handler{repo: repo, log: log}
}

func (h *Handler) Routes() http.Handler {
	r := chi.NewRouter()
	r.Get("/health", h.health)
	r.Get("/notifications/{id}", h.getNotification)
	r.Get("/notifications", h.listNotifications)
	return r
}

// ─── health ────────────────────────────────────────────────────────────────────

func (h *Handler) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "service": "notification-service"})
}

// ─── GET /notifications ────────────────────────────────────────────────────────

func (h *Handler) listNotifications(w http.ResponseWriter, r *http.Request) {
	limit := 20
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}
	offset := 0
	if o := r.URL.Query().Get("offset"); o != "" {
		if n, err := strconv.Atoi(o); err == nil && n >= 0 {
			offset = n
		}
	}

	list, err := h.repo.List(r.Context(), limit, offset)
	if err != nil {
		h.log.Error().Err(err).Msg("listNotifications")
		writeError(w, http.StatusInternalServerError, "something went wrong")
		return
	}
	if list == nil {
		list = []*internal.Notification{}
	}

	total, err := h.repo.Count(r.Context())
	if err != nil {
		h.log.Error().Err(err).Msg("countNotifications")
		total = len(list)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"notifications": list,
		"total":         total,
		"limit":         limit,
		"offset":        offset,
	})
}

// ─── GET /notifications/{id} ────────────────────────────────────────────────────

func (h *Handler) getNotification(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid notification id")
		return
	}

	n, err := h.repo.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, internal.ErrNotFound) {
			writeError(w, http.StatusNotFound, "notification not found")
			return
		}
		h.log.Error().Err(err).Msg("getNotification")
		writeError(w, http.StatusInternalServerError, "something went wrong")
		return
	}

	writeJSON(w, http.StatusOK, n)
}

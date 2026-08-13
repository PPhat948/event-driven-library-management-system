package handler

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"inventory-service/internal"
)

type Handler struct {
	repo *internal.InventoryRepo
	log  zerolog.Logger
}

func New(repo *internal.InventoryRepo, log zerolog.Logger) *Handler {
	return &Handler{repo: repo, log: log}
}

func (h *Handler) Routes() http.Handler {
	r := chi.NewRouter()
	r.Get("/health", h.health)

	// Static sub-routes MUST come before the wildcard {book_id} route
	// so chi matches "low-stock" as a literal path, not a parameter.
	r.Get("/inventory/low-stock", h.getLowStock)
	r.Get("/inventory/out-of-stock", h.getOutOfStock)
	r.Get("/inventory/{book_id}", h.getInventory)
	r.Get("/inventory", h.listInventory)

	return r
}

// ── GET /health ───────────────────────────────────────────────────────────────

func (h *Handler) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "service": "inventory-service"})
}

// ── GET /inventory ────────────────────────────────────────────────────────────

func (h *Handler) listInventory(w http.ResponseWriter, r *http.Request) {
	items, err := h.repo.List(r.Context())
	if err != nil {
		h.log.Error().Err(err).Msg("listInventory")
		writeError(w, http.StatusInternalServerError, "something went wrong")
		return
	}
	if items == nil {
		items = []*internal.InventoryItem{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"inventory": items, "total": len(items)})
}

// ── GET /inventory/{book_id} ──────────────────────────────────────────────────

func (h *Handler) getInventory(w http.ResponseWriter, r *http.Request) {
	bookID, err := uuid.Parse(chi.URLParam(r, "book_id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid book_id")
		return
	}

	item, err := h.repo.GetByBookID(r.Context(), bookID)
	if err != nil {
		if errors.Is(err, internal.ErrNotFound) {
			writeError(w, http.StatusNotFound, "inventory item not found")
			return
		}
		h.log.Error().Err(err).Msg("getInventory")
		writeError(w, http.StatusInternalServerError, "something went wrong")
		return
	}

	writeJSON(w, http.StatusOK, item)
}

// ── GET /inventory/low-stock ──────────────────────────────────────────────────

func (h *Handler) getLowStock(w http.ResponseWriter, r *http.Request) {
	items, err := h.repo.ListByStatus(r.Context(), "low_stock")
	if err != nil {
		h.log.Error().Err(err).Msg("getLowStock")
		writeError(w, http.StatusInternalServerError, "something went wrong")
		return
	}
	if items == nil {
		items = []*internal.InventoryItem{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "total": len(items)})
}

// ── GET /inventory/out-of-stock ───────────────────────────────────────────────

func (h *Handler) getOutOfStock(w http.ResponseWriter, r *http.Request) {
	items, err := h.repo.ListByStatus(r.Context(), "out_of_stock")
	if err != nil {
		h.log.Error().Err(err).Msg("getOutOfStock")
		writeError(w, http.StatusInternalServerError, "something went wrong")
		return
	}
	if items == nil {
		items = []*internal.InventoryItem{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "total": len(items)})
}

package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"

	"book-service/internal"
	"book-service/internal/events"
)

type Handler struct {
	pool    *pgxpool.Pool
	books   *internal.BookRepo
	borrows *internal.BorrowRepo
	pub     *events.Publisher
	log     zerolog.Logger
}

func New(pool *pgxpool.Pool, books *internal.BookRepo, borrows *internal.BorrowRepo, pub *events.Publisher, log zerolog.Logger) *Handler {
	return &Handler{
		pool:    pool,
		books:   books,
		borrows: borrows,
		pub:     pub,
		log:     log,
	}
}

func (h *Handler) Routes() http.Handler {
	r := chi.NewRouter()
	r.Get("/health", h.health)
	r.Post("/books", h.createBook)
	r.Get("/books", h.listBooks)
	r.Get("/books/{id}", h.getBook)
	r.Patch("/books/{id}", h.updateBook)
	r.Post("/books/{id}/borrow", h.borrowBook)
	r.Post("/books/{id}/return", h.returnBook)
	r.Delete("/books/{id}", h.deleteBook)
	return r
}

// publish is a helper that logs if SNS publish fails but doesn't return an error.
// The HTTP response has already been committed; failing to publish is an ops issue.
func (h *Handler) publish(r *http.Request, eventType string, payload any) {
	corrID := GetCorrelationID(r.Context())
	if err := h.pub.Publish(r.Context(), eventType, corrID, payload); err != nil {
		h.log.Error().Err(err).Str("event_type", eventType).Msg("publish failed")
	}
}

// ─── health ────────────────────────────────────────────────────────────────────

func (h *Handler) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "service": "book-service"})
}

// ─── POST /books ───────────────────────────────────────────────────────────────

type createBookReq struct {
	Title         string `json:"title"`
	Author        string `json:"author"`
	ISBN          string `json:"isbn"`
	TotalQuantity int    `json:"total_quantity"`
}

func (h *Handler) createBook(w http.ResponseWriter, r *http.Request) {
	var req createBookReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Title == "" || req.Author == "" || req.ISBN == "" || req.TotalQuantity <= 0 {
		writeError(w, http.StatusBadRequest, "title, author, isbn, and total_quantity (>0) are required")
		return
	}

	book, err := h.books.Create(r.Context(), req.Title, req.Author, req.ISBN, req.TotalQuantity)
	if err != nil {
		if errors.Is(err, internal.ErrDuplicateISBN) {
			writeError(w, http.StatusConflict, "isbn already exists")
			return
		}
		h.log.Error().Err(err).Msg("createBook")
		writeError(w, http.StatusInternalServerError, "something went wrong")
		return
	}

	h.publish(r, "book.added", events.BookAdded{
		BookID:         book.ID.String(),
		Title:          book.Title,
		Author:         book.Author,
		ISBN:           book.ISBN,
		TotalQuantity:  book.TotalQuantity,
		AvailableCount: book.AvailableCount,
	})

	writeJSON(w, http.StatusCreated, book)
}

// ─── GET /books ────────────────────────────────────────────────────────────────

func (h *Handler) listBooks(w http.ResponseWriter, r *http.Request) {
	books, err := h.books.List(r.Context())
	if err != nil {
		h.log.Error().Err(err).Msg("listBooks")
		writeError(w, http.StatusInternalServerError, "something went wrong")
		return
	}
	if books == nil {
		books = []*internal.Book{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"books": books, "total": len(books)})
}

// ─── GET /books/{id} ───────────────────────────────────────────────────────────

func (h *Handler) getBook(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid book id")
		return
	}

	book, err := h.books.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, internal.ErrNotFound) {
			writeError(w, http.StatusNotFound, "book not found")
			return
		}
		h.log.Error().Err(err).Msg("getBook")
		writeError(w, http.StatusInternalServerError, "something went wrong")
		return
	}

	writeJSON(w, http.StatusOK, book)
}

// ─── PATCH /books/{id} ─────────────────────────────────────────────────────────

type updateBookReq struct {
	Title         *string `json:"title"`
	Author        *string `json:"author"`
	TotalQuantity *int    `json:"total_quantity"`
}

func (h *Handler) updateBook(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid book id")
		return
	}

	var req updateBookReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	book, err := h.books.Update(r.Context(), id, internal.UpdateBookInput{
		Title:         req.Title,
		Author:        req.Author,
		TotalQuantity: req.TotalQuantity,
	})
	if err != nil {
		if errors.Is(err, internal.ErrNotFound) {
			writeError(w, http.StatusNotFound, "book not found")
			return
		}
		h.log.Error().Err(err).Msg("updateBook")
		writeError(w, http.StatusInternalServerError, "something went wrong")
		return
	}

	h.publish(r, "book.updated", events.BookUpdated{
		BookID:         book.ID.String(),
		Title:          book.Title,
		Author:         book.Author,
		TotalQuantity:  book.TotalQuantity,
		AvailableCount: book.AvailableCount,
	})

	writeJSON(w, http.StatusOK, book)
}

// ─── POST /books/{id}/borrow ───────────────────────────────────────────────────

type borrowBookReq struct {
	MemberID   string    `json:"member_id"`
	MemberName string    `json:"member_name"`
	DueDate    time.Time `json:"due_date"`
}

func (h *Handler) borrowBook(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid book id")
		return
	}

	var req borrowBookReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.MemberID == "" {
		writeError(w, http.StatusBadRequest, "member_id is required")
		return
	}
	if req.DueDate.IsZero() || req.DueDate.Before(time.Now()) {
		writeError(w, http.StatusBadRequest, "due_date must be a future date")
		return
	}

	// check book exists before starting a transaction
	if _, err := h.books.GetByID(r.Context(), id); err != nil {
		if errors.Is(err, internal.ErrNotFound) {
			writeError(w, http.StatusNotFound, "book not found")
			return
		}
		h.log.Error().Err(err).Msg("borrowBook: get book")
		writeError(w, http.StatusInternalServerError, "something went wrong")
		return
	}

	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		h.log.Error().Err(err).Msg("borrowBook: begin tx")
		writeError(w, http.StatusInternalServerError, "something went wrong")
		return
	}
	defer tx.Rollback(r.Context())

	book, err := h.books.DecrementAvailable(r.Context(), tx, id)
	if err != nil {
		if errors.Is(err, internal.ErrNoAvailableCopies) {
			writeError(w, http.StatusConflict, "no copies available")
			return
		}
		h.log.Error().Err(err).Msg("borrowBook: decrement")
		writeError(w, http.StatusInternalServerError, "something went wrong")
		return
	}

	record, err := h.borrows.Create(r.Context(), tx, id, req.MemberID, req.MemberName, req.DueDate)
	if err != nil {
		h.log.Error().Err(err).Msg("borrowBook: create record")
		writeError(w, http.StatusInternalServerError, "something went wrong")
		return
	}

	if err := tx.Commit(r.Context()); err != nil {
		h.log.Error().Err(err).Msg("borrowBook: commit")
		writeError(w, http.StatusInternalServerError, "something went wrong")
		return
	}

	h.publish(r, "book.borrowed", events.BookBorrowed{
		BookID:              book.ID.String(),
		BookTitle:           book.Title,
		MemberID:            req.MemberID,
		MemberName:          req.MemberName,
		DueDate:             req.DueDate,
		AvailableCountAfter: book.AvailableCount,
	})

	writeJSON(w, http.StatusOK, record)
}

// ─── POST /books/{id}/return ───────────────────────────────────────────────────

type returnBookReq struct {
	MemberID string `json:"member_id"`
}

func (h *Handler) returnBook(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid book id")
		return
	}

	var req returnBookReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.MemberID == "" {
		writeError(w, http.StatusBadRequest, "member_id is required")
		return
	}

	// find the active borrow record before starting a transaction
	record, err := h.borrows.GetActive(r.Context(), id, req.MemberID)
	if err != nil {
		if errors.Is(err, internal.ErrNoActiveBorrow) {
			writeError(w, http.StatusNotFound, "no active borrow record found")
			return
		}
		h.log.Error().Err(err).Msg("returnBook: get active")
		writeError(w, http.StatusInternalServerError, "something went wrong")
		return
	}

	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		h.log.Error().Err(err).Msg("returnBook: begin tx")
		writeError(w, http.StatusInternalServerError, "something went wrong")
		return
	}
	defer tx.Rollback(r.Context())

	record, err = h.borrows.MarkReturned(r.Context(), tx, record.ID)
	if err != nil {
		h.log.Error().Err(err).Msg("returnBook: mark returned")
		writeError(w, http.StatusInternalServerError, "something went wrong")
		return
	}

	book, err := h.books.IncrementAvailable(r.Context(), tx, id)
	if err != nil {
		h.log.Error().Err(err).Msg("returnBook: increment")
		writeError(w, http.StatusInternalServerError, "something went wrong")
		return
	}

	if err := tx.Commit(r.Context()); err != nil {
		h.log.Error().Err(err).Msg("returnBook: commit")
		writeError(w, http.StatusInternalServerError, "something went wrong")
		return
	}

	h.publish(r, "book.returned", events.BookReturned{
		BookID:              book.ID.String(),
		BookTitle:           book.Title,
		MemberID:            req.MemberID,
		BorrowRecordID:      record.ID.String(),
		ReturnedAt:          *record.ReturnedAt,
		AvailableCountAfter: book.AvailableCount,
	})

	writeJSON(w, http.StatusOK, record)
}

// ─── DELETE /books/{id} ────────────────────────────────────────────────────────

func (h *Handler) deleteBook(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid book id")
		return
	}

	// fetch before delete so we have the title for the event payload
	book, err := h.books.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, internal.ErrNotFound) {
			writeError(w, http.StatusNotFound, "book not found")
			return
		}
		h.log.Error().Err(err).Msg("deleteBook: get book")
		writeError(w, http.StatusInternalServerError, "something went wrong")
		return
	}

	if err := h.books.SoftDelete(r.Context(), id); err != nil {
		h.log.Error().Err(err).Msg("deleteBook")
		writeError(w, http.StatusInternalServerError, "something went wrong")
		return
	}

	h.publish(r, "book.deleted", events.BookDeleted{
		BookID: id.String(),
		Title:  book.Title,
	})

	writeJSON(w, http.StatusOK, map[string]string{"message": "book deleted", "id": id.String()})
}

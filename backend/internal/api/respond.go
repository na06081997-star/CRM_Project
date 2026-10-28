package api

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/na06081997-star/CRM_Project/backend/internal/domain"
	"github.com/na06081997-star/CRM_Project/backend/internal/storage"
)

type errorBody struct {
	Error     string            `json:"error"`
	Message   string            `json:"message"`
	Fields    map[string]string `json:"fields,omitempty"`
	Conflicts []domain.Conflict `json:"conflicts,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if v != nil {
		json.NewEncoder(w).Encode(v) //nolint:errcheck
	}
}

func ok(w http.ResponseWriter, v any)      { writeJSON(w, http.StatusOK, v) }
func created(w http.ResponseWriter, v any) { writeJSON(w, http.StatusCreated, v) }
func noContent(w http.ResponseWriter)      { w.WriteHeader(http.StatusNoContent) }

// fail maps domain errors to HTTP responses with Hebrew user messages.
func fail(w http.ResponseWriter, r *http.Request, err error) {
	var ve *domain.ValidationError
	var ce *domain.ConflictError
	switch {
	case errors.As(err, &ve):
		writeJSON(w, http.StatusUnprocessableEntity, errorBody{Error: "validation", Message: "יש לתקן את השדות המסומנים", Fields: ve.Fields})
	case errors.As(err, &ce):
		writeJSON(w, http.StatusConflict, errorBody{Error: "conflict", Message: "המועד מתנגש עם הזמנה קיימת", Conflicts: ce.Conflicts})
	case errors.Is(err, domain.ErrNotFound):
		writeJSON(w, http.StatusNotFound, errorBody{Error: "not_found", Message: "הפריט לא נמצא"})
	case errors.Is(err, domain.ErrUnauthorized):
		writeJSON(w, http.StatusUnauthorized, errorBody{Error: "unauthorized", Message: "יש להתחבר מחדש"})
	case errors.Is(err, domain.ErrForbidden):
		writeJSON(w, http.StatusForbidden, errorBody{Error: "forbidden", Message: "אין לך הרשאה לפעולה זו"})
	case errors.Is(err, storage.ErrDisabled):
		writeJSON(w, http.StatusServiceUnavailable, errorBody{Error: "storage_disabled", Message: "אחסון קבצים אינו מוגדר"})
	case errors.Is(err, errBadRequest):
		writeJSON(w, http.StatusBadRequest, errorBody{Error: "bad_request", Message: err.Error()})
	default:
		slog.Error("request failed", "method", r.Method, "path", r.URL.Path, "err", err)
		writeJSON(w, http.StatusInternalServerError, errorBody{Error: "internal", Message: "אירעה שגיאה, נסו שוב"})
	}
}

var errBadRequest = errors.New("בקשה לא תקינה")

func decode(r *http.Request, v any) error {
	dec := json.NewDecoder(http.MaxBytesReader(nil, r.Body, 1<<20))
	if err := dec.Decode(v); err != nil {
		return errBadRequest
	}
	return nil
}

func pathID(r *http.Request) (uuid.UUID, error) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		return uuid.Nil, domain.ErrNotFound
	}
	return id, nil
}

// ── query helpers ────────────────────────────────────────────

func qUUID(r *http.Request, key string) *uuid.UUID {
	if id, err := uuid.Parse(r.URL.Query().Get(key)); err == nil {
		return &id
	}
	return nil
}

func qUUIDs(r *http.Request, key string) []uuid.UUID {
	var out []uuid.UUID
	for _, p := range qList(r, key) {
		if id, err := uuid.Parse(p); err == nil {
			out = append(out, id)
		}
	}
	return out
}

func qList(r *http.Request, key string) []string {
	var out []string
	for _, v := range r.URL.Query()[key] {
		for _, p := range strings.Split(v, ",") {
			if p = strings.TrimSpace(p); p != "" {
				out = append(out, p)
			}
		}
	}
	return out
}

func qTime(r *http.Request, key string) time.Time {
	v := r.URL.Query().Get(key)
	if t, err := time.Parse(time.RFC3339, v); err == nil {
		return t
	}
	if t, err := time.Parse("2006-01-02", v); err == nil {
		return t
	}
	return time.Time{}
}

func qInt(r *http.Request, key string, def int) int {
	if n, err := strconv.Atoi(r.URL.Query().Get(key)); err == nil {
		return n
	}
	return def
}

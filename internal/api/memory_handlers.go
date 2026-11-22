package api

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/rs/zerolog"
	"github.com/trezutil/trezutil/internal/application/memory"
	"github.com/trezutil/trezutil/internal/domain"
	"github.com/trezutil/trezutil/packages/auth"
)

// MemoryUseCases is the commercial memory of the signed-in person (memory.Service).
type MemoryUseCases interface {
	Get(ctx context.Context, userID string) (*memory.Snapshot, error)
	Add(ctx context.Context, userID string, kind domain.MemoryKind, text string) (*domain.MemoryItem, error)
	Update(ctx context.Context, userID, itemID, text string) (*domain.MemoryItem, error)
	Delete(ctx context.Context, userID, itemID string) error
	Clear(ctx context.Context, userID string) error
	SetEnabled(ctx context.Context, userID string, enabled bool) error
}

// MemoryHandler serves /api/v1/memory. Every route needs a signed-in user and only ever touches that user's memory.
type MemoryHandler struct {
	memory MemoryUseCases
	logger zerolog.Logger
}

// NewMemoryHandler builds the handler.
func NewMemoryHandler(m MemoryUseCases, logger zerolog.Logger) *MemoryHandler {
	return &MemoryHandler{memory: m, logger: logger}
}

type memoryItemJSON struct {
	ID        string `json:"id"`
	Kind      string `json:"kind"`
	Text      string `json:"text"`
	CreatedAt int64  `json:"created_at"`
	UpdatedAt int64  `json:"updated_at"`
}

func toMemoryItemJSON(item domain.MemoryItem) memoryItemJSON {
	return memoryItemJSON{ID: item.ID, Kind: string(item.Kind), Text: item.Text, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt}
}

// ServeHTTP routes:
//
//	GET    /api/v1/memory               → {"enabled": true, "items": [...]}
//	PUT    /api/v1/memory/settings      {"enabled": false}
//	POST   /api/v1/memory/items         {"kind": "brand", "text": "Dell"} → 201 item
//	PATCH  /api/v1/memory/items/{id}    {"text": "…"} → item
//	DELETE /api/v1/memory/items/{id}    → 204
//	DELETE /api/v1/memory/items         → 204 (everything)
func (h *MemoryHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	userID, ok := auth.RequireUser(w, r)
	if !ok {
		return
	}
	path := strings.TrimSuffix(r.URL.Path, "/")
	itemID := strings.TrimPrefix(path, "/api/v1/memory/items/")
	hasItemID := itemID != path && itemID != "" && !strings.Contains(itemID, "/")
	switch {
	case path == "/api/v1/memory" && r.Method == http.MethodGet:
		h.get(w, r, userID)
	case path == "/api/v1/memory/settings" && r.Method == http.MethodPut:
		h.settings(w, r, userID)
	case path == "/api/v1/memory/items" && r.Method == http.MethodPost:
		h.add(w, r, userID)
	case path == "/api/v1/memory/items" && r.Method == http.MethodDelete:
		if err := h.memory.Clear(r.Context(), userID); err != nil {
			h.fail(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	case hasItemID && r.Method == http.MethodPatch:
		h.update(w, r, userID, itemID)
	case hasItemID && r.Method == http.MethodDelete:
		if err := h.memory.Delete(r.Context(), userID, itemID); err != nil {
			h.fail(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		writeJSON(w, ErrorResponse{Error: Error{Code: "NOT_FOUND", Message: "Route not found"}}, http.StatusNotFound)
	}
}

func (h *MemoryHandler) get(w http.ResponseWriter, r *http.Request, userID string) {
	snap, err := h.memory.Get(r.Context(), userID)
	if err != nil {
		h.fail(w, err)
		return
	}
	items := make([]memoryItemJSON, 0, len(snap.Items))
	for _, item := range snap.Items {
		items = append(items, toMemoryItemJSON(item))
	}
	writeJSON(w, map[string]interface{}{"enabled": snap.Enabled, "items": items}, http.StatusOK)
}

func (h *MemoryHandler) settings(w http.ResponseWriter, r *http.Request, userID string) {
	var req struct {
		Enabled *bool `json:"enabled"`
	}
	if err := readJSONBody(r, &req); err != nil || req.Enabled == nil {
		writeJSON(w, ErrorResponse{Error: Error{Code: "INVALID_REQUEST", Message: "Send {\"enabled\": true|false}"}}, http.StatusBadRequest)
		return
	}
	if err := h.memory.SetEnabled(r.Context(), userID, *req.Enabled); err != nil {
		h.fail(w, err)
		return
	}
	writeJSON(w, map[string]interface{}{"enabled": *req.Enabled}, http.StatusOK)
}

func (h *MemoryHandler) add(w http.ResponseWriter, r *http.Request, userID string) {
	var req struct {
		Kind string `json:"kind"`
		Text string `json:"text"`
	}
	if err := readJSONBody(r, &req); err != nil {
		writeJSON(w, ErrorResponse{Error: Error{Code: "INVALID_REQUEST", Message: "Invalid request format"}}, http.StatusBadRequest)
		return
	}
	item, err := h.memory.Add(r.Context(), userID, domain.MemoryKind(req.Kind), req.Text)
	if err != nil {
		h.fail(w, err)
		return
	}
	writeJSON(w, toMemoryItemJSON(*item), http.StatusCreated)
}

func (h *MemoryHandler) update(w http.ResponseWriter, r *http.Request, userID, itemID string) {
	var req struct {
		Text string `json:"text"`
	}
	if err := readJSONBody(r, &req); err != nil {
		writeJSON(w, ErrorResponse{Error: Error{Code: "INVALID_REQUEST", Message: "Invalid request format"}}, http.StatusBadRequest)
		return
	}
	item, err := h.memory.Update(r.Context(), userID, itemID, req.Text)
	if err != nil {
		h.fail(w, err)
		return
	}
	writeJSON(w, toMemoryItemJSON(*item), http.StatusOK)
}

func (h *MemoryHandler) fail(w http.ResponseWriter, err error) {
	var sensitive *memory.SensitiveError
	switch {
	case errors.As(err, &sensitive):
		writeJSON(w, map[string]interface{}{"error": map[string]string{
			"code": "SENSITIVE_DATA", "message": "Commercial memory never keeps payment data, codes or secrets", "reason": sensitive.Reason,
		}}, http.StatusUnprocessableEntity)
	case errors.Is(err, memory.ErrEmptyText):
		writeJSON(w, ErrorResponse{Error: Error{Code: "EMPTY_TEXT", Message: "text is required"}}, http.StatusBadRequest)
	case errors.Is(err, memory.ErrInvalidKind):
		writeJSON(w, ErrorResponse{Error: Error{Code: "INVALID_KIND", Message: "kind must be preference, brand, budget, constraint, supplier or project"}}, http.StatusBadRequest)
	case errors.Is(err, memory.ErrTooMany):
		writeJSON(w, ErrorResponse{Error: Error{Code: "MEMORY_FULL", Message: "The commercial memory is full; remove an item first"}}, http.StatusConflict)
	case errors.Is(err, memory.ErrNotFound):
		writeJSON(w, ErrorResponse{Error: Error{Code: "NOT_FOUND", Message: "Memory item not found"}}, http.StatusNotFound)
	case errors.Is(err, memory.ErrForbidden):
		auth.WriteUnauthorized(w, "AUTHENTICATION_REQUIRED", "Sign in to use the commercial memory")
	default:
		h.logger.Error().Err(err).Msg("Commercial memory failed")
		writeJSON(w, ErrorResponse{Error: Error{Code: "MEMORY_FAILED", Message: "The commercial memory could not be read or changed"}}, http.StatusInternalServerError)
	}
}

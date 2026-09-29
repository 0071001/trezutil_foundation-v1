package api

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/trezutil/trezutil/internal/domain"
	"github.com/trezutil/trezutil/packages/auth"
)

// CategoryStore is the subset of the category repository used by the API.
type CategoryStore interface {
	Create(ctx context.Context, category *domain.Category) error
	FindActive(ctx context.Context) ([]domain.Category, error)
}

// UserRoleChecker resolves platform roles for the authenticated user.
type UserRoleChecker interface {
	FindByID(ctx context.Context, id string) (*domain.User, error)
}

type categoryJSON struct {
	ID       string  `json:"id"`
	ParentID *string `json:"parent_id,omitempty"`
	Name     string  `json:"name"`
	Slug     string  `json:"slug"`
}

// ListCategoriesH handles GET /v1/categories (public).
func ListCategoriesH(categories CategoryStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		items, err := categories.FindActive(r.Context())
		if err != nil {
			writeJSON(w, ErrorResponse{Error: Error{Code: "CATEGORY_LIST_FAILED", Message: "Could not load categories"}}, http.StatusInternalServerError)
			return
		}
		out := make([]categoryJSON, 0, len(items))
		for _, c := range items {
			out = append(out, categoryJSON{ID: c.ID, ParentID: c.ParentID, Name: c.Name, Slug: c.Slug})
		}
		writeJSON(w, map[string]interface{}{"items": out}, http.StatusOK)
	}
}

// CreateCategoryH handles POST /v1/categories (platform admins/operators only).
func CreateCategoryH(categories CategoryStore, users UserRoleChecker) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := auth.RequireUser(w, r)
		if !ok {
			return
		}
		if u, err := users.FindByID(r.Context(), userID); err != nil || !(u.HasRole(domain.RoleAdmin) || u.HasRole(domain.RoleOperator)) {
			writeJSON(w, ErrorResponse{Error: Error{Code: "FORBIDDEN", Message: "Only platform operators can manage categories"}}, http.StatusForbidden)
			return
		}
		var req struct {
			Name     string  `json:"name"`
			Slug     string  `json:"slug"`
			ParentID *string `json:"parent_id"`
		}
		if err := readJSONBody(r, &req); err != nil || strings.TrimSpace(req.Name) == "" || strings.TrimSpace(req.Slug) == "" {
			writeJSON(w, ErrorResponse{Error: Error{Code: "INVALID_REQUEST", Message: "name and slug are required"}}, http.StatusBadRequest)
			return
		}
		now := time.Now().Unix()
		c := &domain.Category{
			ID:        domain.NewID(),
			ParentID:  req.ParentID,
			Name:      strings.TrimSpace(req.Name),
			Slug:      strings.ToLower(strings.TrimSpace(req.Slug)),
			Status:    domain.CategoryStatusActive,
			Scope:     domain.CategoryScopeGlobal,
			CreatedAt: now,
			UpdatedAt: now,
		}
		if err := categories.Create(r.Context(), c); err != nil {
			writeJSON(w, ErrorResponse{Error: Error{Code: "CATEGORY_CREATE_FAILED", Message: "Could not create category (duplicate slug?)"}}, http.StatusConflict)
			return
		}
		writeJSON(w, categoryJSON{ID: c.ID, ParentID: c.ParentID, Name: c.Name, Slug: c.Slug}, http.StatusCreated)
	}
}

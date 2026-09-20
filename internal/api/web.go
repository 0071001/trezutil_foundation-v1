/* Trezutil Web Handler — serves the frontend SPA and proxies API requests */

package api

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// WebConfig holds web server configuration.
type WebConfig struct {
	WebRoot string // Path to web/ directory
}

// NewWebHandler creates a new web handler.
func NewWebHandler(config WebConfig) *WebHandler {
	return &WebHandler{config: config}
}

type WebHandler struct {
	config WebConfig
}

// ServeHTTP serves static files from the web directory with SPA fallback.
func (h *WebHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	// API proxy — forward /api/ requests to backend services
	if strings.HasPrefix(path, "/api/") {
		h.handleAPIProxy(w, r)
		return
	}

	// Try to serve static file
	if path != "/" {
		filePath := filepath.Join(h.config.WebRoot, path)
		if _, err := os.Stat(filePath); err == nil {
			http.ServeFile(w, r, filePath)
			return
		}
	}

	// SPA fallback — serve index.html for all other routes
	indexPath := filepath.Join(h.config.WebRoot, "index.html")
	if _, err := os.Stat(indexPath); err == nil {
		http.ServeFile(w, r, indexPath)
		return
	}

	writeJSON(w, ErrorResponse{Error: Error{Code: "WEB_ROOT_MISSING", Message: "Web root not configured"}}, http.StatusInternalServerError)
}

func (h *WebHandler) handleAPIProxy(w http.ResponseWriter, r *http.Request) {
	// In production, proxy to backend services.
	// For now, return a helpful message.
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `{"status":"ok","message":"API proxy ready"}`)
}

// WebContext provides web-specific context for handlers.
type WebContext struct {
	UserID string
	Path   string
}

// UserFromContext returns the user ID from context if set.
func UserFromContext(ctx context.Context) string {
	if userID, ok := ctx.Value("user_id").(string); ok {
		return userID
	}
	return ""
}

// SetUserIDContext adds user ID to context.
func SetUserIDContext(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, "user_id", userID)
}

// IsWebRequest returns true if the request originates from the web client.
func IsWebRequest(r *http.Request) bool {
	accept := r.Header.Get("Accept")
	return strings.Contains(accept, "text/html") ||
		strings.Contains(r.URL.Path, ".html") ||
		r.URL.Path == "/"
}

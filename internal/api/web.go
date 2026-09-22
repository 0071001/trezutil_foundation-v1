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

// HandleAPIRoutes routes API requests to the appropriate handlers.
func (h *WebHandler) HandleAPIRoutes(
	inventoryHandler InventoryHandlerInterface,
	fulfillmentHandler FulfillmentHandlerInterface,
	orderHandler OrderHandlerInterface,
	providerHandler ProviderHandlerInterface,
	verticalPackHandler VerticalPackHandlerInterface,
	w http.ResponseWriter, r *http.Request) {

	path := r.URL.Path
	method := r.Method

	// Inventory routes
	if strings.HasPrefix(path, "/api/v1/inventory") {
		switch {
		case path == "/api/v1/inventory" && method == "GET":
			inventoryHandler.ListInventory(w, r)
			return
		case strings.HasPrefix(path, "/api/v1/inventory/") && method == "GET":
			inventoryHandler.GetInventory(w, r)
			return
		}
	}

	// Fulfillment routes
	if strings.HasPrefix(path, "/api/v1/fulfillments") {
		switch {
		case strings.HasPrefix(path, "/api/v1/fulfillments/") && strings.HasSuffix(path, "/transition") && method == "POST":
			fulfillmentHandler.TransitionFulfillment(w, r)
			return
		case strings.HasPrefix(path, "/api/v1/fulfillments/") && strings.HasSuffix(path, "/assign") && method == "POST":
			fulfillmentHandler.AssignFulfillment(w, r)
			return
		case strings.HasPrefix(path, "/api/v1/fulfillments/") && strings.HasSuffix(path, "/tracking") && method == "POST":
			fulfillmentHandler.UpdateTracking(w, r)
			return
		case strings.HasPrefix(path, "/api/v1/fulfillments/") && method == "GET":
			fulfillmentHandler.GetFulfillment(w, r)
			return
		}
	}

	// Order fulfillment route
	if strings.HasPrefix(path, "/api/v1/orders/") && strings.HasSuffix(path, "/fulfillment") && method == "GET" {
		fulfillmentHandler.GetFulfillmentByOrder(w, r)
		return
	}

	// Order confirm route
	if strings.HasPrefix(path, "/api/v1/orders/") && strings.HasSuffix(path, "/confirm") && method == "POST" {
		orderHandler.ConfirmOrder(w, r)
		return
	}

	// Reservation fulfillment route
	if strings.HasPrefix(path, "/api/v1/reservations/") && strings.HasSuffix(path, "/fulfillment") && method == "GET" {
		fulfillmentHandler.GetFulfillmentByReservation(w, r)
		return
	}

	// Provider routes
	if strings.HasPrefix(path, "/api/v1/providers") {
		switch {
		case path == "/api/v1/providers" && method == "GET":
			providerHandler.ListProvidersH(w, r)
			return
		case path == "/api/v1/providers" && method == "POST":
			providerHandler.CreateProviderH(w, r)
			return
		case strings.HasPrefix(path, "/api/v1/providers/") && strings.HasSuffix(path, "/enable") && method == "POST":
			providerHandler.EnableProviderH(w, r)
			return
		case strings.HasPrefix(path, "/api/v1/providers/") && strings.HasSuffix(path, "/disable") && method == "POST":
			providerHandler.DisableProviderH(w, r)
			return
		case strings.HasPrefix(path, "/api/v1/providers/") && strings.HasSuffix(path, "/capabilities") && method == "GET":
			providerHandler.GetProviderCapabilitiesH(w, r)
			return
		case strings.HasPrefix(path, "/api/v1/providers/") && method == "GET":
			providerHandler.GetProviderH(w, r)
			return
		}
	}

	// Vertical pack routes
	if strings.HasPrefix(path, "/api/v1/packs") {
		HandleVerticalPacksRoutes(verticalPackHandler, w, r)
		return
	}

	// Not found
	writeJSON(w, ErrorResponse{Error: Error{Code: "NOT_FOUND", Message: "Route not found"}}, http.StatusNotFound)
}

// APIHandler is a minimal interface for routing API requests.
type APIHandler interface {
	ServeHTTP(w http.ResponseWriter, r *http.Request)
}

// InventoryHandlerInterface defines inventory API methods.
type InventoryHandlerInterface interface {
	ListInventory(w http.ResponseWriter, r *http.Request)
	GetInventory(w http.ResponseWriter, r *http.Request)
}

// FulfillmentHandlerInterface defines fulfillment API methods.
type FulfillmentHandlerInterface interface {
	GetFulfillment(w http.ResponseWriter, r *http.Request)
	GetFulfillmentByOrder(w http.ResponseWriter, r *http.Request)
	GetFulfillmentByReservation(w http.ResponseWriter, r *http.Request)
	TransitionFulfillment(w http.ResponseWriter, r *http.Request)
	AssignFulfillment(w http.ResponseWriter, r *http.Request)
	UpdateTracking(w http.ResponseWriter, r *http.Request)
}

// OrderHandlerInterface defines order API methods.
type OrderHandlerInterface interface {
	ConfirmOrder(w http.ResponseWriter, r *http.Request)
}

// ProviderHandlerInterface defines provider API methods.
type ProviderHandlerInterface interface {
	ListProvidersH(w http.ResponseWriter, r *http.Request)
	GetProviderH(w http.ResponseWriter, r *http.Request)
	CreateProviderH(w http.ResponseWriter, r *http.Request)
	EnableProviderH(w http.ResponseWriter, r *http.Request)
	DisableProviderH(w http.ResponseWriter, r *http.Request)
	GetProviderCapabilitiesH(w http.ResponseWriter, r *http.Request)
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

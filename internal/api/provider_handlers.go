package api

import (
	"fmt"
	"net/http"

	"github.com/trezutil/trezutil/internal/application/provider"
	"github.com/trezutil/trezutil/internal/domain"
	"github.com/trezutil/trezutil/packages/auth"
)

type ProviderHandler struct {
	providerUseCases ProviderUseCases
}

func NewProviderHandler(providerUseCases ProviderUseCases) *ProviderHandler {
	return &ProviderHandler{providerUseCases: providerUseCases}
}

func (h *ProviderHandler) ListProvidersH(w http.ResponseWriter, r *http.Request) {
	actorID := auth.UserID(r.Context())
	if actorID == "" {
		writeJSON(w, ErrorResponse{Error: Error{Code: "MISSING_USER_ID", Message: "User ID required"}}, http.StatusUnauthorized)
		return
	}

	orgID := r.URL.Query().Get("organization_id")
	if orgID == "" {
		writeJSON(w, ErrorResponse{Error: Error{Code: "MISSING_ORGANIZATION_ID", Message: "organization_id is required"}}, http.StatusBadRequest)
		return
	}

	offset, limit := 0, 20
	fmt.Sscanf(r.URL.Query().Get("offset"), "%d", &offset)
	fmt.Sscanf(r.URL.Query().Get("limit"), "%d", &limit)
	if limit == 0 {
		limit = 20
	}

	providers, err := h.providerUseCases.ListProviders(r.Context(), actorID, orgID, offset, limit)
	if err != nil {
		switch err {
		case domain.ErrInsufficientPermissions:
			writeJSON(w, ErrorResponse{Error: Error{Code: "FORBIDDEN", Message: "Insufficient permissions"}}, http.StatusForbidden)
		case domain.ErrOrganizationNotFound:
			writeJSON(w, ErrorResponse{Error: Error{Code: "ORGANIZATION_NOT_FOUND", Message: "Organization not found"}}, http.StatusNotFound)
		default:
			writeFailure(w, "PROVIDER_LIST_FAILED", err)
		}
		return
	}

	writeJSON(w, map[string]interface{}{
		"data": providers,
		"pagination": map[string]interface{}{
			"offset": offset,
			"limit":  limit,
			"total":  len(providers),
		},
	}, http.StatusOK)
}

func (h *ProviderHandler) GetProviderH(w http.ResponseWriter, r *http.Request) {
	providerID := r.URL.Query().Get("provider_id")
	if providerID == "" {
		writeJSON(w, ErrorResponse{Error: Error{Code: "MISSING_PROVIDER_ID", Message: "Provider ID required"}}, http.StatusBadRequest)
		return
	}

	actorID := auth.UserID(r.Context())
	if actorID == "" {
		writeJSON(w, ErrorResponse{Error: Error{Code: "MISSING_USER_ID", Message: "User ID required"}}, http.StatusUnauthorized)
		return
	}

	orgID := r.URL.Query().Get("organization_id")
	if orgID == "" {
		writeJSON(w, ErrorResponse{Error: Error{Code: "MISSING_ORGANIZATION_ID", Message: "organization_id is required"}}, http.StatusBadRequest)
		return
	}

	result, err := h.providerUseCases.GetProvider(r.Context(), actorID, orgID, providerID)
	if err != nil {
		switch err {
		case domain.ErrProviderNotFound:
			writeJSON(w, ErrorResponse{Error: Error{Code: "PROVIDER_NOT_FOUND", Message: "Provider not found"}}, http.StatusNotFound)
		case domain.ErrInsufficientPermissions:
			writeJSON(w, ErrorResponse{Error: Error{Code: "FORBIDDEN", Message: "Insufficient permissions"}}, http.StatusForbidden)
		case domain.ErrOrganizationNotFound:
			writeJSON(w, ErrorResponse{Error: Error{Code: "ORGANIZATION_NOT_FOUND", Message: "Organization not found"}}, http.StatusNotFound)
		default:
			writeFailure(w, "PROVIDER_GET_FAILED", err)
		}
		return
	}

	writeJSON(w, result, http.StatusOK)
}

func (h *ProviderHandler) CreateProviderH(w http.ResponseWriter, r *http.Request) {
	actorID := auth.UserID(r.Context())
	if actorID == "" {
		writeJSON(w, ErrorResponse{Error: Error{Code: "MISSING_USER_ID", Message: "User ID required"}}, http.StatusUnauthorized)
		return
	}

	orgID := r.URL.Query().Get("organization_id")
	if orgID == "" {
		writeJSON(w, ErrorResponse{Error: Error{Code: "MISSING_ORGANIZATION_ID", Message: "organization_id is required"}}, http.StatusBadRequest)
		return
	}

	var req struct {
		Code         string   `json:"code"`
		Name         string   `json:"name"`
		Type         string   `json:"type"`
		Capabilities []string `json:"capabilities"`
		SecretRef    string   `json:"secret_ref"`
		Priority     int      `json:"priority"`
	}
	if err := readJSONBody(r, &req); err != nil {
		writeJSON(w, ErrorResponse{Error: Error{Code: "INVALID_REQUEST", Message: "Invalid request format"}}, http.StatusBadRequest)
		return
	}

	if req.Code == "" || req.Name == "" {
		writeJSON(w, ErrorResponse{Error: Error{Code: "INVALID_REQUEST", Message: "code and name are required"}}, http.StatusBadRequest)
		return
	}

	createReq := provider.CreateProviderRequest{
		Code:         req.Code,
		Name:         req.Name,
		Type:         req.Type,
		Capabilities: req.Capabilities,
		SecretRef:    req.SecretRef,
		Priority:     req.Priority,
	}

	result, err := h.providerUseCases.CreateProvider(r.Context(), actorID, orgID, createReq)
	if err != nil {
		switch err {
		case domain.ErrInsufficientPermissions:
			writeJSON(w, ErrorResponse{Error: Error{Code: "FORBIDDEN", Message: "Insufficient permissions"}}, http.StatusForbidden)
		case domain.ErrOrganizationNotFound:
			writeJSON(w, ErrorResponse{Error: Error{Code: "ORGANIZATION_NOT_FOUND", Message: "Organization not found"}}, http.StatusNotFound)
		default:
			writeFailure(w, "PROVIDER_CREATE_FAILED", err)
		}
		return
	}

	writeJSON(w, result, http.StatusCreated)
}

func (h *ProviderHandler) EnableProviderH(w http.ResponseWriter, r *http.Request) {
	providerID := r.URL.Query().Get("provider_id")
	if providerID == "" {
		writeJSON(w, ErrorResponse{Error: Error{Code: "MISSING_PROVIDER_ID", Message: "Provider ID required"}}, http.StatusBadRequest)
		return
	}

	actorID := auth.UserID(r.Context())
	if actorID == "" {
		writeJSON(w, ErrorResponse{Error: Error{Code: "MISSING_USER_ID", Message: "User ID required"}}, http.StatusUnauthorized)
		return
	}

	orgID := r.URL.Query().Get("organization_id")
	if orgID == "" {
		writeJSON(w, ErrorResponse{Error: Error{Code: "MISSING_ORGANIZATION_ID", Message: "organization_id is required"}}, http.StatusBadRequest)
		return
	}

	err := h.providerUseCases.EnableProvider(r.Context(), actorID, orgID, providerID)
	if err != nil {
		switch err {
		case domain.ErrProviderNotFound:
			writeJSON(w, ErrorResponse{Error: Error{Code: "PROVIDER_NOT_FOUND", Message: "Provider not found"}}, http.StatusNotFound)
		case domain.ErrInsufficientPermissions:
			writeJSON(w, ErrorResponse{Error: Error{Code: "FORBIDDEN", Message: "Insufficient permissions"}}, http.StatusForbidden)
		default:
			writeFailure(w, "PROVIDER_ENABLE_FAILED", err)
		}
		return
	}

	writeJSON(w, map[string]string{"provider_id": providerID, "status": "enabled"}, http.StatusOK)
}

func (h *ProviderHandler) DisableProviderH(w http.ResponseWriter, r *http.Request) {
	providerID := r.URL.Query().Get("provider_id")
	if providerID == "" {
		writeJSON(w, ErrorResponse{Error: Error{Code: "MISSING_PROVIDER_ID", Message: "Provider ID required"}}, http.StatusBadRequest)
		return
	}

	actorID := auth.UserID(r.Context())
	if actorID == "" {
		writeJSON(w, ErrorResponse{Error: Error{Code: "MISSING_USER_ID", Message: "User ID required"}}, http.StatusUnauthorized)
		return
	}

	orgID := r.URL.Query().Get("organization_id")
	if orgID == "" {
		writeJSON(w, ErrorResponse{Error: Error{Code: "MISSING_ORGANIZATION_ID", Message: "organization_id is required"}}, http.StatusBadRequest)
		return
	}

	err := h.providerUseCases.DisableProvider(r.Context(), actorID, orgID, providerID)
	if err != nil {
		switch err {
		case domain.ErrProviderNotFound:
			writeJSON(w, ErrorResponse{Error: Error{Code: "PROVIDER_NOT_FOUND", Message: "Provider not found"}}, http.StatusNotFound)
		case domain.ErrInsufficientPermissions:
			writeJSON(w, ErrorResponse{Error: Error{Code: "FORBIDDEN", Message: "Insufficient permissions"}}, http.StatusForbidden)
		default:
			writeFailure(w, "PROVIDER_DISABLE_FAILED", err)
		}
		return
	}

	writeJSON(w, map[string]string{"provider_id": providerID, "status": "disabled"}, http.StatusOK)
}

func (h *ProviderHandler) GetProviderCapabilitiesH(w http.ResponseWriter, r *http.Request) {
	providerID := r.URL.Query().Get("provider_id")
	if providerID == "" {
		writeJSON(w, ErrorResponse{Error: Error{Code: "MISSING_PROVIDER_ID", Message: "Provider ID required"}}, http.StatusBadRequest)
		return
	}

	actorID := auth.UserID(r.Context())
	if actorID == "" {
		writeJSON(w, ErrorResponse{Error: Error{Code: "MISSING_USER_ID", Message: "User ID required"}}, http.StatusUnauthorized)
		return
	}

	orgID := r.URL.Query().Get("organization_id")
	if orgID == "" {
		writeJSON(w, ErrorResponse{Error: Error{Code: "MISSING_ORGANIZATION_ID", Message: "organization_id is required"}}, http.StatusBadRequest)
		return
	}

	capabilities, err := h.providerUseCases.GetProviderCapabilities(r.Context(), actorID, orgID, providerID)
	if err != nil {
		switch err {
		case domain.ErrProviderNotFound:
			writeJSON(w, ErrorResponse{Error: Error{Code: "PROVIDER_NOT_FOUND", Message: "Provider not found"}}, http.StatusNotFound)
		case domain.ErrInsufficientPermissions:
			writeJSON(w, ErrorResponse{Error: Error{Code: "FORBIDDEN", Message: "Insufficient permissions"}}, http.StatusForbidden)
		default:
			writeFailure(w, "PROVIDER_CAPABILITIES_FAILED", err)
		}
		return
	}

	writeJSON(w, map[string]interface{}{
		"capabilities": capabilities,
	}, http.StatusOK)
}

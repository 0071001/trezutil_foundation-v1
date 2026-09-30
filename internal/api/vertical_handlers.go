package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/trezutil/trezutil/internal/application/vertical"
	"github.com/trezutil/trezutil/internal/domain"
)

type VerticalPackUseCases interface {
	ListPacks() ([]*domain.VerticalPack, error)
	GetPack(code string) (*domain.VerticalPack, error)
	EnablePack(orgID, packCode string) error
	DisablePack(orgID, packCode string) error
	ListOrgPacks(orgID string) ([]domain.PackCode, error)
	IsActionSupported(orgID string, action domain.CommerceActionType) bool
}

type VerticalPackService struct {
	registry *vertical.PackRegistry
	resolver *vertical.PackResolver
	policies *vertical.PolicyEngine
}

func NewVerticalPackService(registry *vertical.PackRegistry, resolver *vertical.PackResolver, policies *vertical.PolicyEngine) *VerticalPackService {
	return &VerticalPackService{
		registry: registry,
		resolver: resolver,
		policies: policies,
	}
}

func (s *VerticalPackService) ListPacks() ([]*domain.VerticalPack, error) {
	return s.registry.GetAll(), nil
}

func (s *VerticalPackService) GetPack(code string) (*domain.VerticalPack, error) {
	pack, ok := s.registry.Get(domain.PackCode(code))
	if !ok {
		return nil, ErrNotFound
	}
	return pack, nil
}

func (s *VerticalPackService) EnablePack(orgID, packCode string) error {
	return s.resolver.EnablePackForOrg(orgID, domain.PackCode(packCode))
}

func (s *VerticalPackService) DisablePack(orgID, packCode string) error {
	s.resolver.DisablePackForOrg(orgID, domain.PackCode(packCode))
	return nil
}

func (s *VerticalPackService) ListOrgPacks(orgID string) ([]domain.PackCode, error) {
	return s.resolver.GetAvailablePacks(orgID), nil
}

func (s *VerticalPackService) IsActionSupported(orgID string, action domain.CommerceActionType) bool {
	return s.resolver.IsActionSupported(orgID, action)
}

var ErrNotFound = &APIError{Code: "NOT_FOUND", Message: "resource not found"}

type APIError struct {
	Code    string      `json:"code"`
	Message string      `json:"message"`
	Details interface{} `json:"details,omitempty"`
}

func (e *APIError) Error() string {
	return e.Message
}

func NewVerticalPackHandler(service *VerticalPackService) *VerticalPackHandler {
	return &VerticalPackHandler{service: service}
}

type VerticalPackHandler struct {
	service *VerticalPackService
}

func (h *VerticalPackHandler) ListPacks(w http.ResponseWriter, r *http.Request) {
	packs, err := h.service.ListPacks()
	if err != nil {
		writeFailure(w, "PACKS_LIST_FAILED", err)
		return
	}

	response := make([]VerticalPackResponse, 0, len(packs))
	for _, pack := range packs {
		response = append(response, toPackResponse(pack))
	}

	writeJSON(w, response, http.StatusOK)
}

func (h *VerticalPackHandler) GetPack(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	prefix := "/api/v1/packs/"
	code := strings.TrimPrefix(path, prefix)

	if code == "" || code == prefix {
		writeJSON(w, ErrorResponse{Error: Error{Code: "MISSING_PACK_CODE", Message: "Pack code is required"}}, http.StatusBadRequest)
		return
	}

	pack, err := h.service.GetPack(code)
	if err != nil {
		if err == ErrNotFound {
			writeJSON(w, ErrorResponse{Error: Error{Code: "PACK_NOT_FOUND", Message: "Pack not found"}}, http.StatusNotFound)
		} else {
			writeFailure(w, "PACK_FETCH_FAILED", err)
		}
		return
	}

	writeJSON(w, toPackResponse(pack), http.StatusOK)
}

func (h *VerticalPackHandler) EnablePack(w http.ResponseWriter, r *http.Request) {
	orgID := r.URL.Query().Get("org_id")
	if orgID == "" {
		writeJSON(w, ErrorResponse{Error: Error{Code: "MISSING_ORG_ID", Message: "org_id query parameter is required"}}, http.StatusBadRequest)
		return
	}

	path := r.URL.Path
	prefix := "/api/v1/packs/"
	suffix := "/enable"
	code := ""
	if strings.HasPrefix(path, prefix) && strings.HasSuffix(path, suffix) {
		code = path[len(prefix) : len(path)-len(suffix)]
	}
	if code == "" {
		writeJSON(w, ErrorResponse{Error: Error{Code: "MISSING_PACK_CODE", Message: "Pack code is required in path"}}, http.StatusBadRequest)
		return
	}

	if err := h.service.EnablePack(orgID, code); err != nil {
		writeJSON(w, ErrorResponse{Error: Error{Code: "PACK_ENABLE_FAILED", Message: "The pack could not be enabled"}}, http.StatusBadRequest)
		return
	}

	writeJSON(w, map[string]interface{}{"status": "enabled", "pack_code": code, "org_id": orgID}, http.StatusOK)
}

func (h *VerticalPackHandler) DisablePack(w http.ResponseWriter, r *http.Request) {
	orgID := r.URL.Query().Get("org_id")
	if orgID == "" {
		writeJSON(w, ErrorResponse{Error: Error{Code: "MISSING_ORG_ID", Message: "org_id query parameter is required"}}, http.StatusBadRequest)
		return
	}

	path := r.URL.Path
	prefix := "/api/v1/packs/"
	suffix := "/disable"
	code := ""
	if strings.HasPrefix(path, prefix) && strings.HasSuffix(path, suffix) {
		code = path[len(prefix) : len(path)-len(suffix)]
	}
	if code == "" {
		writeJSON(w, ErrorResponse{Error: Error{Code: "MISSING_PACK_CODE", Message: "Pack code is required in path"}}, http.StatusBadRequest)
		return
	}

	if err := h.service.DisablePack(orgID, code); err != nil {
		writeFailure(w, "PACK_DISABLE_FAILED", err)
		return
	}

	writeJSON(w, map[string]interface{}{"status": "disabled", "pack_code": code, "org_id": orgID}, http.StatusOK)
}

func (h *VerticalPackHandler) ListOrgPacks(w http.ResponseWriter, r *http.Request) {
	orgID := r.URL.Query().Get("org_id")
	if orgID == "" {
		writeJSON(w, ErrorResponse{Error: Error{Code: "MISSING_ORG_ID", Message: "org_id query parameter is required"}}, http.StatusBadRequest)
		return
	}

	packs, err := h.service.ListOrgPacks(orgID)
	if err != nil {
		writeFailure(w, "ORG_PACKS_LIST_FAILED", err)
		return
	}

	packCodes := make([]string, 0, len(packs))
	for _, code := range packs {
		packCodes = append(packCodes, string(code))
	}

	writeJSON(w, map[string]interface{}{"org_id": orgID, "packs": packCodes}, http.StatusOK)
}

func (h *VerticalPackHandler) CheckAction(w http.ResponseWriter, r *http.Request) {
	orgID := r.URL.Query().Get("org_id")
	if orgID == "" {
		writeJSON(w, ErrorResponse{Error: Error{Code: "MISSING_ORG_ID", Message: "org_id query parameter is required"}}, http.StatusBadRequest)
		return
	}

	action := r.URL.Query().Get("action")
	if action == "" {
		writeJSON(w, ErrorResponse{Error: Error{Code: "MISSING_ACTION", Message: "action query parameter is required"}}, http.StatusBadRequest)
		return
	}

	allowed := h.service.IsActionSupported(orgID, domain.CommerceActionType(action))

	writeJSON(w, map[string]interface{}{
		"org_id":     orgID,
		"action":     action,
		"allowed":    allowed,
		"checked_at": time.Now().Unix(),
	}, http.StatusOK)
}

type VerticalPackResponse struct {
	ID               string                 `json:"id"`
	Code             string                 `json:"code"`
	Name             string                 `json:"name"`
	Description      string                 `json:"description"`
	Version          PackVersionResponse    `json:"version"`
	Status           string                 `json:"status"`
	Capabilities     []string               `json:"capabilities"`
	SupportedActions []string               `json:"supported_actions"`
	Policies         []PackPolicyResponse   `json:"policies"`
	Workflow         *PackWorkflowResponse  `json:"workflow,omitempty"`
	UIMetadata       PackUIMetadataResponse `json:"ui_metadata"`
	AIProfile        string                 `json:"ai_profile"`
	CreatedAt        int64                  `json:"created_at"`
	UpdatedAt        int64                  `json:"updated_at"`
}

type PackVersionResponse struct {
	Major int `json:"major"`
	Minor int `json:"minor"`
	Patch int `json:"patch"`
}

type PackPolicyResponse struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type PackWorkflowStepResponse struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Action      string   `json:"action"`
	Required    bool     `json:"required"`
	NextSteps   []string `json:"next_steps"`
}

type PackWorkflowResponse struct {
	Name      string                     `json:"name"`
	EntryStep string                     `json:"entry_step"`
	Steps     []PackWorkflowStepResponse `json:"steps"`
}

type PackUIMetadataResponse struct {
	RoutePrefix string `json:"route_prefix"`
	Icon        string `json:"icon"`
	Color       string `json:"color"`
	Order       int    `json:"order"`
}

func toPackResponse(pack *domain.VerticalPack) VerticalPackResponse {
	resp := VerticalPackResponse{
		ID:          pack.ID,
		Code:        string(pack.Code),
		Name:        pack.Name,
		Description: pack.Description,
		Version: PackVersionResponse{
			Major: pack.Version.Major,
			Minor: pack.Version.Minor,
			Patch: pack.Version.Patch,
		},
		Status:           string(pack.Status),
		Capabilities:     make([]string, len(pack.Capabilities)),
		SupportedActions: make([]string, len(pack.SupportedActions)),
		Policies:         make([]PackPolicyResponse, len(pack.Policies)),
		UIMetadata: PackUIMetadataResponse{
			RoutePrefix: pack.UIMetadata.RoutePrefix,
			Icon:        pack.UIMetadata.Icon,
			Color:       pack.UIMetadata.Color,
			Order:       pack.UIMetadata.Order,
		},
		AIProfile: string(pack.AIProfile),
		CreatedAt: pack.CreatedAt,
		UpdatedAt: pack.UpdatedAt,
	}

	for i, c := range pack.Capabilities {
		resp.Capabilities[i] = string(c)
	}
	for i, a := range pack.SupportedActions {
		resp.SupportedActions[i] = string(a)
	}
	for i, p := range pack.Policies {
		resp.Policies[i] = PackPolicyResponse{Key: p.Key, Value: p.Value}
	}

	if pack.Workflow != nil {
		resp.Workflow = &PackWorkflowResponse{
			Name:      pack.Workflow.Name,
			EntryStep: pack.Workflow.EntryStep,
			Steps:     make([]PackWorkflowStepResponse, len(pack.Workflow.Steps)),
		}
		for i, s := range pack.Workflow.Steps {
			resp.Workflow.Steps[i] = PackWorkflowStepResponse{
				Name:        s.Name,
				Description: s.Description,
				Action:      string(s.Action),
				Required:    s.Required,
				NextSteps:   s.NextSteps,
			}
		}
	}

	return resp
}

func HandleVerticalPacksRoutes(
	handler VerticalPackHandlerInterface,
	w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	method := r.Method

	if strings.HasPrefix(path, "/api/v1/packs") {
		switch {
		case path == "/api/v1/packs" && method == "GET":
			handler.ListPacks(w, r)
			return
		case strings.HasPrefix(path, "/api/v1/packs/") && strings.HasSuffix(path, "/enable") && method == "POST":
			handler.EnablePack(w, r)
			return
		case strings.HasPrefix(path, "/api/v1/packs/") && strings.HasSuffix(path, "/disable") && method == "POST":
			handler.DisablePack(w, r)
			return
		case path == "/api/v1/packs/org" && method == "GET":
			handler.ListOrgPacks(w, r)
			return
		case strings.HasPrefix(path, "/api/v1/packs/org/") && method == "GET":
			handler.ListOrgPacks(w, r)
			return
		case path == "/api/v1/packs/action" && method == "GET":
			handler.CheckAction(w, r)
			return
		case strings.HasPrefix(path, "/api/v1/packs/") && method == "GET":
			handler.GetPack(w, r)
			return
		}
	}
}

type VerticalPackHandlerInterface interface {
	ListPacks(w http.ResponseWriter, r *http.Request)
	GetPack(w http.ResponseWriter, r *http.Request)
	EnablePack(w http.ResponseWriter, r *http.Request)
	DisablePack(w http.ResponseWriter, r *http.Request)
	ListOrgPacks(w http.ResponseWriter, r *http.Request)
	CheckAction(w http.ResponseWriter, r *http.Request)
}

func init() {
	_ = json.Marshal
}

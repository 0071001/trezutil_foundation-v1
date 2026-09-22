package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/trezutil/trezutil/internal/application/provider"
	"github.com/trezutil/trezutil/internal/domain"
)

type mockProviderUseCases struct {
	response *provider.ProviderResponse
	err      error
	caps     []string
}

func (m *mockProviderUseCases) CreateProvider(ctx context.Context, actorID string, orgID string, req provider.CreateProviderRequest) (*provider.ProviderResponse, error) {
	return m.response, m.err
}

func (m *mockProviderUseCases) GetProvider(ctx context.Context, actorID string, orgID string, providerID string) (*provider.ProviderResponse, error) {
	return m.response, m.err
}

func (m *mockProviderUseCases) ListProviders(ctx context.Context, actorID string, orgID string, offset, limit int) ([]*provider.ProviderResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	return []*provider.ProviderResponse{m.response}, nil
}

func (m *mockProviderUseCases) EnableProvider(ctx context.Context, actorID string, orgID string, providerID string) error {
	return m.err
}

func (m *mockProviderUseCases) DisableProvider(ctx context.Context, actorID string, orgID string, providerID string) error {
	return m.err
}

func (m *mockProviderUseCases) GetProviderCapabilities(ctx context.Context, actorID string, orgID string, providerID string) ([]string, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.caps, nil
}

func testProviderResponse() *provider.ProviderResponse {
	return &provider.ProviderResponse{
		ID:             "prov-1",
		OrganizationID: "org-1",
		Code:           "test-prov",
		Name:           "Test Provider",
		Type:           "hotel",
		Status:         string(domain.ProviderStatusActive),
		Capabilities:   []string{"AVAILABILITY", "RESERVATION"},
		HealthStatus:   string(domain.ProviderHealthHealthy),
	}
}

func TestProviderHandler_ListProviders(t *testing.T) {
	tests := []struct {
		name           string
		actorID        string
		orgID          string
		mockErr        error
		expectedStatus int
	}{
		{
			name:           "missing user_id",
			actorID:        "",
			orgID:          "org-1",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "missing organization_id",
			actorID:        "user-1",
			orgID:          "",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "unauthorized",
			actorID:        "user-1",
			orgID:          "org-1",
			mockErr:        domain.ErrInsufficientPermissions,
			expectedStatus: http.StatusForbidden,
		},
		{
			name:           "success",
			actorID:        "user-1",
			orgID:          "org-1",
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uc := &mockProviderUseCases{response: testProviderResponse(), err: tt.mockErr}
			handler := NewProviderHandler(uc)
			req := httptest.NewRequest(http.MethodGet, "/v1/providers?user_id="+tt.actorID+"&organization_id="+tt.orgID, nil)
			w := httptest.NewRecorder()

			handler.ListProvidersH(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}
		})
	}
}

func TestProviderHandler_GetProvider(t *testing.T) {
	tests := []struct {
		name           string
		actorID        string
		orgID          string
		providerID     string
		mockErr        error
		expectedStatus int
	}{
		{
			name:           "missing provider_id",
			actorID:        "user-1",
			orgID:          "org-1",
			providerID:     "",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "missing user_id",
			actorID:        "",
			orgID:          "org-1",
			providerID:     "prov-1",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "not found",
			actorID:        "user-1",
			orgID:          "org-1",
			providerID:     "prov-1",
			mockErr:        domain.ErrProviderNotFound,
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "success",
			actorID:        "user-1",
			orgID:          "org-1",
			providerID:     "prov-1",
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uc := &mockProviderUseCases{response: testProviderResponse(), err: tt.mockErr}
			handler := NewProviderHandler(uc)
			req := httptest.NewRequest(http.MethodGet, "/v1/providers?provider_id="+tt.providerID+"&user_id="+tt.actorID+"&organization_id="+tt.orgID, nil)
			w := httptest.NewRecorder()

			handler.GetProviderH(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}
		})
	}
}

func TestProviderHandler_CreateProvider(t *testing.T) {
	tests := []struct {
		name           string
		actorID        string
		orgID          string
		body           string
		mockErr        error
		expectedStatus int
	}{
		{
			name:           "missing user_id",
			actorID:        "",
			orgID:          "org-1",
			body:           `{"code":"test","name":"Test"}`,
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "missing organization_id",
			actorID:        "user-1",
			orgID:          "",
			body:           `{"code":"test","name":"Test"}`,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "missing code and name",
			actorID:        "user-1",
			orgID:          "org-1",
			body:           `{}`,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "forbidden",
			actorID:        "user-1",
			orgID:          "org-1",
			body:           `{"code":"test","name":"Test"}`,
			mockErr:        domain.ErrInsufficientPermissions,
			expectedStatus: http.StatusForbidden,
		},
		{
			name:           "success",
			actorID:        "user-1",
			orgID:          "org-1",
			body:           `{"code":"test","name":"Test","type":"hotel","capabilities":["AVAILABILITY"]}`,
			expectedStatus: http.StatusCreated,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uc := &mockProviderUseCases{response: testProviderResponse(), err: tt.mockErr}
			handler := NewProviderHandler(uc)
			req := httptest.NewRequest(http.MethodPost, "/v1/providers?user_id="+tt.actorID+"&organization_id="+tt.orgID, strings.NewReader(tt.body))
			w := httptest.NewRecorder()

			handler.CreateProviderH(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}
		})
	}
}

func TestProviderHandler_EnableDisable(t *testing.T) {
	t.Run("enable success", func(t *testing.T) {
		uc := &mockProviderUseCases{}
		handler := NewProviderHandler(uc)
		req := httptest.NewRequest(http.MethodPost, "/v1/providers/enable?provider_id=prov-1&user_id=user-1&organization_id=org-1", nil)
		w := httptest.NewRecorder()

		handler.EnableProviderH(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
		}
	})

	t.Run("disable success", func(t *testing.T) {
		uc := &mockProviderUseCases{}
		handler := NewProviderHandler(uc)
		req := httptest.NewRequest(http.MethodPost, "/v1/providers/disable?provider_id=prov-1&user_id=user-1&organization_id=org-1", nil)
		w := httptest.NewRecorder()

		handler.DisableProviderH(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
		}
	})

	t.Run("enable not found", func(t *testing.T) {
		uc := &mockProviderUseCases{err: domain.ErrProviderNotFound}
		handler := NewProviderHandler(uc)
		req := httptest.NewRequest(http.MethodPost, "/v1/providers/enable?provider_id=prov-1&user_id=user-1&organization_id=org-1", nil)
		w := httptest.NewRecorder()

		handler.EnableProviderH(w, req)
		if w.Code != http.StatusNotFound {
			t.Errorf("Expected status %d, got %d", http.StatusNotFound, w.Code)
		}
	})
}

func TestProviderHandler_GetCapabilities(t *testing.T) {
	uc := &mockProviderUseCases{caps: []string{"AVAILABILITY", "RESERVATION"}}
	handler := NewProviderHandler(uc)
	req := httptest.NewRequest(http.MethodGet, "/v1/providers/capabilities?provider_id=prov-1&user_id=user-1&organization_id=org-1", nil)
	w := httptest.NewRecorder()

	handler.GetProviderCapabilitiesH(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status %d, got %d", http.StatusOK, w.Code)
	}
}

func TestProviderHandler_SecretsNotReturned(t *testing.T) {
	uc := &mockProviderUseCases{response: testProviderResponse()}
	handler := NewProviderHandler(uc)
	req := httptest.NewRequest(http.MethodGet, "/v1/providers?provider_id=prov-1&user_id=user-1&organization_id=org-1", nil)
	w := httptest.NewRecorder()

	handler.GetProviderH(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	body := w.Body.String()
	if strings.Contains(body, "SecretRef") || strings.Contains(body, "secret") {
		t.Error("secrets should not be returned in API response")
	}
}

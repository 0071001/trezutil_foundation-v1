package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rs/zerolog"
	"github.com/trezutil/trezutil/internal/application/vertical"
	"github.com/trezutil/trezutil/internal/domain"
)

type fakeOrgPackRepoFn struct{}

func (fakeOrgPackRepoFn) Create(_ interface{}, _ *domain.OrganizationPack) error {
	return nil
}
func (fakeOrgPackRepoFn) FindByOrgAndPack(_ interface{}, _ string, _ domain.PackCode) (*domain.OrganizationPack, error) {
	return nil, nil
}
func (fakeOrgPackRepoFn) ListByOrg(_ interface{}, _ string) ([]domain.OrganizationPack, error) {
	return nil, nil
}
func (fakeOrgPackRepoFn) Enable(_ interface{}, _ string, _ domain.PackCode) error  { return nil }
func (fakeOrgPackRepoFn) Disable(_ interface{}, _ string, _ domain.PackCode) error { return nil }
func (fakeOrgPackRepoFn) IsPackEnabled(_ interface{}, _ string, _ domain.PackCode) (bool, error) {
	return false, nil
}

func newTestVerticalService() *VerticalPackService {
	logger := zerolog.Nop()
	registry := vertical.NewPackRegistry(vertical.PackRegistryConfig{Logger: logger})
	vertical.LoadAllPacks(registry)

	policies := vertical.NewPolicyEngine(vertical.PolicyEngineConfig{Logger: logger})

	var fakeRepo domain.OrganizationPackRepository = fakeOrgPackRepoFn{}

	resolver := vertical.NewPackResolver(vertical.PackResolverConfig{
		Logger:   logger,
		Registry: registry,
		Policies: policies,
		PackRepo: fakeRepo,
	})

	return NewVerticalPackService(registry, resolver, policies)
}

func TestVerticalPackHandler_ListPacks(t *testing.T) {
	service := newTestVerticalService()
	handler := NewVerticalPackHandler(service)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/packs", nil)
	w := httptest.NewRecorder()

	handler.ListPacks(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response []VerticalPackResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(response) != 3 {
		t.Fatalf("expected 3 packs, got %d", len(response))
	}

	codes := make(map[string]bool)
	for _, p := range response {
		codes[p.Code] = true
	}
	if !codes["ECOMMERCE"] || !codes["BOOKING"] || !codes["SERVICES"] {
		t.Errorf("expected ECOMMERCE, BOOKING, SERVICES packs, got: %v", codes)
	}
}

func TestVerticalPackHandler_GetPack(t *testing.T) {
	service := newTestVerticalService()
	handler := NewVerticalPackHandler(service)

	t.Run("existing pack", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/packs/ECOMMERCE", nil)
		w := httptest.NewRecorder()

		handler.GetPack(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, w.Code)
		}

		var resp VerticalPackResponse
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if resp.Code != "ECOMMERCE" {
			t.Errorf("expected code ECOMMERCE, got %s", resp.Code)
		}
		if resp.AIProfile != "COMMERCE_AI" {
			t.Errorf("expected AIProfile COMMERCE_AI, got %s", resp.AIProfile)
		}
		if len(resp.Capabilities) == 0 {
			t.Error("expected non-empty capabilities")
		}
		if resp.Workflow == nil {
			t.Error("expected workflow to be present")
		}
	})

	t.Run("non-existent pack", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/packs/NONEXISTENT", nil)
		w := httptest.NewRecorder()

		handler.GetPack(w, req)

		if w.Code != http.StatusNotFound {
			t.Fatalf("expected status %d, got %d", http.StatusNotFound, w.Code)
		}
	})
}

func TestVerticalPackHandler_EnablePack(t *testing.T) {
	service := newTestVerticalService()
	handler := NewVerticalPackHandler(service)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/packs/ECOMMERCE/enable?org_id=org-1", nil)
	w := httptest.NewRecorder()

	handler.EnablePack(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["status"] != "enabled" {
		t.Errorf("expected status 'enabled', got %v", resp["status"])
	}
}

func TestVerticalPackHandler_EnablePack_MissingOrgID(t *testing.T) {
	service := newTestVerticalService()
	handler := NewVerticalPackHandler(service)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/packs/ECOMMERCE/enable", nil)
	w := httptest.NewRecorder()

	handler.EnablePack(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestVerticalPackHandler_EnablePack_UnknownPack(t *testing.T) {
	service := newTestVerticalService()
	handler := NewVerticalPackHandler(service)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/packs/UNKNOWN/enable?org_id=org-1", nil)
	w := httptest.NewRecorder()

	handler.EnablePack(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestVerticalPackHandler_DisablePack(t *testing.T) {
	service := newTestVerticalService()
	handler := NewVerticalPackHandler(service)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/packs/ECOMMERCE/disable?org_id=org-1", nil)
	w := httptest.NewRecorder()

	handler.DisablePack(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, w.Code)
	}
}

func TestVerticalPackHandler_ListOrgPacks(t *testing.T) {
	service := newTestVerticalService()
	handler := NewVerticalPackHandler(service)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/packs/org?org_id=org-1", nil)
	w := httptest.NewRecorder()

	handler.ListOrgPacks(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["org_id"] != "org-1" {
		t.Errorf("expected org_id 'org-1', got %v", resp["org_id"])
	}
}

func TestVerticalPackHandler_ListOrgPacks_MissingOrgID(t *testing.T) {
	service := newTestVerticalService()
	handler := NewVerticalPackHandler(service)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/packs/org", nil)
	w := httptest.NewRecorder()

	handler.ListOrgPacks(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestVerticalPackHandler_CheckAction(t *testing.T) {
	service := newTestVerticalService()
	handler := NewVerticalPackHandler(service)

	service.EnablePack("org-1", string(domain.PackECommerce))

	t.Run("supported action", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/packs/action?org_id=org-1&action=CREATE_ORDER", nil)
		w := httptest.NewRecorder()

		handler.CheckAction(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, w.Code)
		}

		var resp map[string]interface{}
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if resp["allowed"] != true {
			t.Errorf("expected action to be allowed")
		}
	})

	t.Run("unsupported action", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/packs/action?org_id=org-1&action=CREATE_RESERVATION", nil)
		w := httptest.NewRecorder()

		handler.CheckAction(w, req)

		var resp map[string]interface{}
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if resp["allowed"] != false {
			t.Errorf("expected action to not be allowed")
		}
	})

	t.Run("missing org_id", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/packs/action?action=CREATE_ORDER", nil)
		w := httptest.NewRecorder()

		handler.CheckAction(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected status %d, got %d", http.StatusBadRequest, w.Code)
		}
	})
}

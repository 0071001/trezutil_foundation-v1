package postgres

import (
	"testing"

	"github.com/trezutil/trezutil/internal/domain"
)

func TestProviderConfigRoundTrip(t *testing.T) {
	in := domain.ProviderConfig{
		Endpoints: map[string]string{"api": "https://api.example"},
		Settings:  map[string]string{"publishable_key": `pk_"quoted"`, "capture_method": "manual"},
	}
	out := decodeProviderConfig(encodeProviderConfig(in))
	if out.Endpoints["api"] != "https://api.example" || out.Settings["publishable_key"] != `pk_"quoted"` || out.Settings["capture_method"] != "manual" {
		t.Fatalf("round trip lost data: %+v", out)
	}
	legacy := decodeProviderConfig(`{"endpoints.api":"https://a","settings.mode":"x"}`)
	if legacy.Endpoints["api"] != "https://a" || legacy.Settings["mode"] != "x" {
		t.Fatalf("legacy format not decoded: %+v", legacy)
	}
}

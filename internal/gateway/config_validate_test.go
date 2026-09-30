package gateway

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestConfigValidateRequiresExplicitSafeCORSWhenStrict(t *testing.T) {
	cases := []struct {
		name    string
		env     string
		strict  bool
		wantErr bool
	}{
		{"strict, unset", "", true, true},
		{"strict, wildcard", "*", true, true},
		{"strict, wildcard in list", "https://a.test, *", true, true},
		{"strict, placeholder", "https://trezutil.example", true, true},
		{"strict, explicit origin", "https://app.trezutil.com", true, false},
		{"development, unset", "", false, false},
		{"development, wildcard", "*", false, false},
	}
	for _, c := range cases {
		t.Setenv("CORS_ALLOWED_ORIGINS", c.env)
		err := LoadConfig().Validate(c.strict)
		if (err != nil) != c.wantErr {
			t.Errorf("%s: err = %v, wantErr %v", c.name, err, c.wantErr)
		}
	}
}

func TestSecurityHeaders(t *testing.T) {
	h := securityHeaders(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	for k, want := range map[string]string{"X-Content-Type-Options": "nosniff", "X-Frame-Options": "DENY", "Referrer-Policy": "strict-origin-when-cross-origin"} {
		if rec.Header().Get(k) != want {
			t.Errorf("%s = %q", k, rec.Header().Get(k))
		}
	}
	if rec.Header().Get("Strict-Transport-Security") != "" {
		t.Error("HSTS must not be sent over plain HTTP")
	}
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Forwarded-Proto", "https")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Header().Get("Strict-Transport-Security") == "" {
		t.Error("HSTS expected behind the TLS proxy")
	}
}

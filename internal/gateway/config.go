// Package gateway implements the unified, external API boundary for Trezutil.
//
// The gateway exposes a single, stable namespace (/api/v1/*) to browsers and
// external clients, proxies requests to the appropriate internal services,
// serves the frontend SPA, and centralizes cross-cutting concerns such as
// correlation IDs, CORS, timeouts and error normalization. Internal services
// may keep their own route conventions (e.g. /v1/* vs /api/v1/*); the gateway
// reconciles the two without modifying the frontend contract.
package gateway

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"
)

// Upstream describes a reverse-proxied backend service.
type Upstream struct {
	Name string
	URL  *url.URL
	// StripAPI strips the leading "/api" segment before forwarding to the
	// upstream. Internal services that mount under /v1/* (e.g. search-service)
	// require this; services already mounted under /api/v1/* (e.g.
	// conversation-service) leave it false to preserve the path verbatim.
	StripAPI bool
	// Timeout is the per-upstream deadline for proxied requests. If zero, the
	// gateway-wide UpstreamTimeout is used.
	Timeout time.Duration
}

// Config holds the gateway configuration. All upstream URLs are supplied by the
// environment so that no localhost ports are hardcoded in production code.
type Config struct {
	Addr            string        // listen address, e.g. ":8080"
	WebRoot         string        // filesystem path to the SPA (web/)
	AllowedOrigins  []string      // explicit CORS allowlist (never "*")
	RequestTimeout  time.Duration // total request deadline at the edge
	UpstreamTimeout time.Duration // default upstream proxy deadline
	MaxBodyBytes    int64         // maximum accepted request body (default 1 MiB)
	Upstreams       map[string]*Upstream
}

// LoadConfig builds a Config from environment variables with safe defaults
// for local development. Production deployments must set the upstream URLs.
func LoadConfig() Config {
	return Config{
		Addr:            env("GATEWAY_PORT", ":8080"),
		WebRoot:         env("WEB_ROOT", "./web/frontend/dist"),
		AllowedOrigins:  splitCSV(env("CORS_ALLOWED_ORIGINS", "http://localhost:8080")),
		RequestTimeout:  durationEnv("GATEWAY_TIMEOUT", 30*time.Second),
		UpstreamTimeout: durationEnv("GATEWAY_UPSTREAM_TIMEOUT", 30*time.Second),
		Upstreams:       defaultUpstreams(),
	}
}

// Validate rejects a configuration that must not serve real traffic. With
// strict set (any APP_ENV except development/test) the CORS allowlist must be
// set explicitly (the development default is never used), must not contain
// "*", and must not still hold the documentation placeholder.
func (c Config) Validate(strict bool) error {
	if !strict {
		return nil
	}
	if strings.TrimSpace(os.Getenv("CORS_ALLOWED_ORIGINS")) == "" || len(c.AllowedOrigins) == 0 {
		return errors.New("CORS_ALLOWED_ORIGINS must be set to the public origin(s) of the web app")
	}
	for _, o := range c.AllowedOrigins {
		switch {
		case o == "*":
			return errors.New(`CORS_ALLOWED_ORIGINS must not contain "*"`)
		case strings.Contains(strings.ToLower(o), "example"):
			return fmt.Errorf("CORS_ALLOWED_ORIGINS still holds the documentation placeholder (%s)", o)
		}
	}
	return nil
}

func defaultUpstreams() map[string]*Upstream {
	return map[string]*Upstream{
		"conversation": {
			Name:     "conversation",
			URL:      mustParseURL(env("CONVERSATION_SERVICE_URL", "http://127.0.0.1:8091")),
			StripAPI: false,
		},
		"search": {
			Name:     "search",
			URL:      mustParseURL(env("SEARCH_SERVICE_URL", "http://127.0.0.1:8081")),
			StripAPI: true,
		},
		"payment": {
			Name:     "payment",
			URL:      mustParseURL(env("PAYMENT_SERVICE_URL", "http://127.0.0.1:8083")),
			StripAPI: false,
		},
		"commerce": {
			Name:     "commerce",
			URL:      mustParseURL(env("COMMERCE_SERVICE_URL", "http://127.0.0.1:8085")),
			StripAPI: false,
		},
		"identity": {
			Name:     "identity",
			URL:      mustParseURL(env("IDENTITY_SERVICE_URL", "http://127.0.0.1:8086")),
			StripAPI: false,
		},
	}
}

func mustParseURL(v string) *url.URL {
	u, err := url.Parse(v)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return nil
	}
	return u
}

func env(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}

func durationEnv(key string, def time.Duration) time.Duration {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return def
}

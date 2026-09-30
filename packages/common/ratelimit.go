package common

import (
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

// WindowLimiter is a fixed-window, in-process rate limiter keyed by string.
// It is intended for coarse abuse protection on single-instance endpoints
// (e.g. OTP issuance); multi-instance deployments should front it with an
// edge limiter as well.
type WindowLimiter struct {
	mu      sync.Mutex
	limit   int
	window  time.Duration
	buckets map[string]*window
	now     func() time.Time
}

type window struct {
	start time.Time
	count int
}

// NewWindowLimiter allows up to limit events per key per window.
func NewWindowLimiter(limit int, per time.Duration) *WindowLimiter {
	return &WindowLimiter{limit: limit, window: per, buckets: map[string]*window{}, now: time.Now}
}

// Allow records an event for key and reports whether it is within the limit.
func (l *WindowLimiter) Allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	if len(l.buckets) > 10000 {
		for k, b := range l.buckets {
			if now.Sub(b.start) >= l.window {
				delete(l.buckets, k)
			}
		}
	}
	b, ok := l.buckets[key]
	if !ok || now.Sub(b.start) >= l.window {
		l.buckets[key] = &window{start: now, count: 1}
		return true
	}
	if b.count >= l.limit {
		return false
	}
	b.count++
	return true
}

// TrustedProxyHops is the number of proxies in front of the gateway that
// append to X-Forwarded-For (TRUSTED_PROXY_HOPS, default 0, clamped to 0..5).
func TrustedProxyHops() int {
	n, err := strconv.Atoi(strings.TrimSpace(os.Getenv("TRUSTED_PROXY_HOPS")))
	if err != nil || n < 0 {
		return 0
	}
	if n > 5 {
		return 5
	}
	return n
}

// ClientIP returns the client address for rate limiting and logs.
//
// The gateway's reverse proxy appends the address of its own peer to
// X-Forwarded-For, so the last entry cannot be forged by the client. When a
// TLS-terminating proxy sits in front of the gateway (TRUSTED_PROXY_HOPS=1,
// the documented production topology) that last entry is the proxy, and the
// real client is the entry before it; without the setting every user would
// share the proxy's address and one per-IP limit. Entries further left are
// client-controlled and never trusted beyond the configured hops.
func ClientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		i := len(parts) - 1 - TrustedProxyHops()
		if i < 0 {
			i = 0
		}
		if ip := strings.TrimSpace(parts[i]); ip != "" {
			return ip
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

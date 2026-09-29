package common

import (
	"net"
	"net/http"
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

// ClientIP returns the client address as seen by the nearest trusted proxy.
// The gateway's reverse proxy appends the peer address to X-Forwarded-For, so
// the last entry is the one the client cannot forge; earlier entries are
// client-controlled and ignored.
func ClientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		if last := strings.TrimSpace(parts[len(parts)-1]); last != "" {
			return last
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

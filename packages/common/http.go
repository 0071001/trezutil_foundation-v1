package common

import (
	"net/http"
	"os"
	"strconv"
	"strings"
)

// DefaultMaxBodyBytes bounds request bodies accepted by public-facing handlers.
const DefaultMaxBodyBytes int64 = 1 << 20 // 1 MiB

// LimitBody caps every request body at maxBytes so a client cannot exhaust
// memory with an oversized JSON payload or webhook body.
func LimitBody(maxBytes int64, next http.Handler) http.Handler {
	if maxBytes <= 0 {
		maxBytes = DefaultMaxBodyBytes
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Body != nil {
			r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
		}
		next.ServeHTTP(w, r)
	})
}

// Getenv returns the environment variable or def when unset/blank.
func Getenv(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

// GetenvInt returns the integer environment variable or def when unset/invalid.
func GetenvInt(key string, def int) int {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

package domain

import (
	"crypto/rand"
	"fmt"
)

// NewID returns a random RFC 4122 version 4 UUID string.
//
// Aggregates are persisted into UUID primary-key columns, so constructors must
// assign a valid identifier up front: an empty string is rejected by
// PostgreSQL and would make the aggregate impossible to reference (e.g. a
// capacity hold recorded before the reservation row exists).
func NewID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = b[6]&0x0f | 0x40 // Version 4
	b[8] = b[8]&0x3f | 0x80 // Variant RFC 4122
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

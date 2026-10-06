package domain

import "context"

// AuditEvent is an audit entry as stored: what the administration console lists.
type AuditEvent struct {
	ID         int64
	OccurredAt int64
	ActorID    string
	// ActorName is the actor's display name, resolved when the event is read ("" for the system or a removed account).
	ActorName  string
	Action     string
	TargetType string
	TargetID   string
	RequestID  string
	Metadata   map[string]interface{}
}

// AuditFilter narrows a listing of audit events. Empty fields match everything.
type AuditFilter struct {
	// Action matches the start of the action name: "order." lists every order event.
	Action     string
	ActorID    string
	TargetType string
	TargetID   string
	// From and To bound OccurredAt (Unix seconds, inclusive); 0 leaves that side open.
	From int64
	To   int64
}

// AuditStore is the durable, append-only record of audit entries.
type AuditStore interface {
	// Append stores one entry. The application can add events and read them, never change or erase them.
	Append(ctx context.Context, entry AuditEntry) error
	// List returns one page, newest first, and the number of events that match the filter.
	List(ctx context.Context, filter AuditFilter, limit, offset int) ([]AuditEvent, int, error)
}

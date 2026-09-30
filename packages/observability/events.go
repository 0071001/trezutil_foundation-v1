package observability

import (
	"context"
	"strings"

	"github.com/rs/zerolog/log"
)

// The helpers below are the single place where a critical failure is both
// logged (structured, with the request id) and counted, so every service
// reports failures the same way. They never fail and never log secrets:
// only ids and normalized reasons.

// PaymentFailed records a payment that ended in failure (stage: create,
// capture, provider_decline, webhook…).
func PaymentFailed(ctx context.Context, paymentID, orderID, stage, reason string) {
	PaymentFailures.Inc(stage)
	log.Error().Str("event", "payment_failed").Str("payment_id", paymentID).Str("order_id", orderID).
		Str("stage", stage).Str("reason", truncate(reason, 200)).
		Str("request_id", RequestIDFromContext(ctx)).Msg("payment failed")
}

// ReservationFailed records a reservation request that was refused or failed.
// reason is a short normalized word (conflict, unavailable, capacity, invalid,
// forbidden, unknown_availability, error); refusals are expected outcomes and
// logged as warnings, real errors as errors.
func ReservationFailed(ctx context.Context, listingID, reason string, err error) {
	ReservationFailures.Inc(reason)
	event := log.Warn()
	if reason == "error" {
		event = log.Error()
	}
	if err != nil {
		event = event.Err(err)
	}
	event.Str("event", "reservation_failed").Str("listing_id", listingID).Str("reason", reason).
		Str("request_id", RequestIDFromContext(ctx)).Msg("reservation failed")
}

// ProviderFailed counts one failed provider attempt (the orchestrator logs it
// with full context).
func ProviderFailed(provider, capability, errorCode string) {
	ProviderFailures.Inc(provider, capability, errorCode)
}

// NormalizeRoute turns a request path into a bounded metric label: identifier
// segments (UUIDs, numbers, long tokens) become ":id".
func NormalizeRoute(path string) string {
	if path == "" || path == "/" {
		return "/"
	}
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) > 8 {
		parts = parts[:8]
	}
	for i, p := range parts {
		if isIdentifier(p) {
			parts[i] = ":id"
		}
	}
	return "/" + strings.Join(parts, "/")
}

func isIdentifier(s string) bool {
	if len(s) > 24 {
		return true
	}
	digits, hex := 0, 0
	for _, c := range s {
		switch {
		case c >= '0' && c <= '9':
			digits++
			hex++
		case (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F') || c == '-':
			hex++
		}
	}
	if digits == len(s) && len(s) > 0 {
		return true
	}
	return len(s) >= 8 && hex == len(s) && digits > 0
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

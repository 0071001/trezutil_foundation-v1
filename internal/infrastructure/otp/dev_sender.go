// Package otp contains OTP delivery adapters that are not tied to a channel.
package otp

import (
	"context"

	"github.com/rs/zerolog"
)

// DevLogSender writes sign-in codes to the service log. It exists only so the
// login flow can be exercised locally without a messaging provider; services
// must refuse to use it unless APP_ENV=development.
type DevLogSender struct {
	Logger zerolog.Logger
}

// Send implements domain.OTPProvider.
func (s DevLogSender) Send(ctx context.Context, destination string, code string) error {
	s.Logger.Warn().Str("phone", destination).Str("otp_code", code).
		Msg("DEVELOPMENT ONLY: OTP code logged instead of delivered")
	return nil
}

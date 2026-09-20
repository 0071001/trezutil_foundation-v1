package domain

import (
	"errors"
	"regexp"
	"strings"
)

// ErrInvalidPhone is returned when a phone number cannot be normalized.
var ErrInvalidPhone = errors.New("invalid phone number")

// phoneRegex matches E.164 phone numbers
var phoneRegex = regexp.MustCompile(`^\+[1-9]\d{1,14}$`)

// NormalizePhone normalizes a phone number to E.164 format.
// It handles various input formats common in the DRC and Africa.
func NormalizePhone(phone string) (string, error) {
	if phone == "" {
		return "", ErrInvalidPhone
	}

	// Remove all non-digit characters except leading +
	cleaned := regexp.MustCompile(`[^\d+]`).ReplaceAllString(phone, "")

	// If it starts with +, validate E.164 format
	if strings.HasPrefix(cleaned, "+") {
		if phoneRegex.MatchString(cleaned) {
			return cleaned, nil
		}
		return "", ErrInvalidPhone
	}

	// If it starts with 0 (e.g., 0812345678), replace with country code
	if strings.HasPrefix(cleaned, "0") {
		cleaned = "+243" + cleaned[1:]
		if phoneRegex.MatchString(cleaned) {
			return cleaned, nil
		}
		return "", ErrInvalidPhone
	}

	// If it starts with country code without + (e.g., 243812345678)
	if strings.HasPrefix(cleaned, "243") {
		cleaned = "+" + cleaned
		if phoneRegex.MatchString(cleaned) {
			return cleaned, nil
		}
		return "", ErrInvalidPhone
	}

	// If it's just a local number (9 digits in DRC), prepend country code
	if len(cleaned) == 9 {
		cleaned = "+243" + cleaned
		if phoneRegex.MatchString(cleaned) {
			return cleaned, nil
		}
		return "", ErrInvalidPhone
	}

	return "", ErrInvalidPhone
}

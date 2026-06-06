package gateway

import (
	"github.com/devrix/devrix/internal/shared/errors"
)

const (
	// MaxMessageLength is the maximum allowed message length
	MaxMessageLength = 64000
)

// InputValidator validates inbound messages
type InputValidator struct{}

// NewInputValidator creates a new InputValidator
func NewInputValidator() *InputValidator {
	return &InputValidator{}
}

// ValidateMessage validates a message and returns an error if invalid
func (v *InputValidator) ValidateMessage(content string) error {
	if content == "" {
		return errors.NewMessageEmptyError()
	}

	if len(content) > MaxMessageLength {
		return errors.WithCode(
			"COMM_MESSAGE_TOO_LONG",
			"message too long",
			errors.ErrMessageTooLong,
		)
	}

	return nil
}

// ValidateSessionID validates a session ID format
func (v *InputValidator) ValidateSessionID(sessionID string) error {
	if sessionID == "" {
		return errors.NewSessionNotFoundError(sessionID)
	}

	// Basic format check (sess_timestamp_random)
	if len(sessionID) < 10 {
		return errors.NewSessionNotFoundError(sessionID)
	}

	return nil
}

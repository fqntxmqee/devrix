package errors

import (
	"errors"
	"fmt"
)

// Config errors (6000-6999)
var (
	ErrConfigNotFound      = errors.New("config file not found")
	ErrConfigParseFailed   = errors.New("config parse failed")
	ErrConfigValidationErr = errors.New("config validation error")
)

// ConfigError wraps a config error with code and message
type ConfigError struct {
	Code    string
	Message string
	Err     error
}

func (e *ConfigError) Error() string {
	return e.Message
}

func (e *ConfigError) Unwrap() error {
	return e.Err
}

// WithCode creates a new ConfigError with the given code
func WithCode(code, message string, err error) *ConfigError {
	return &ConfigError{
		Code:    code,
		Message: message,
		Err:     err,
	}
}

// NewConfigNotFoundError creates a config not found error
func NewConfigNotFoundError(path string) *ConfigError {
	return WithCode(
		"CONFIG_NOT_FOUND_6001",
		fmt.Sprintf("config file not found: %s", path),
		ErrConfigNotFound,
	)
}

// NewConfigParseError creates a config parse error
func NewConfigParseError(path string, err error) *ConfigError {
	return WithCode(
		"CONFIG_PARSE_FAILED_6002",
		fmt.Sprintf("failed to parse config file %s: %v", path, err),
		ErrConfigParseFailed,
	)
}

// NewConfigValidationError creates a config validation error
func NewConfigValidationError(field string, reason string) *ConfigError {
	return WithCode(
		"CONFIG_VALIDATION_6003",
		fmt.Sprintf("config validation failed for %s: %s", field, reason),
		ErrConfigValidationErr,
	)
}

// ErrorCode extracts the error code from an error
func ErrorCode(err error) string {
	var ce *ConfigError
	if errors.As(err, &ce) {
		return ce.Code
	}
	return ""
}

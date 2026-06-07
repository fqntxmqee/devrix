package logger

import (
	"regexp"
	"strings"
)

// Redactor redacts sensitive information from log entries
type Redactor struct {
	patterns []string
	compiled []*regexp.Regexp
}

// NewRedactor creates a new redactor with the given patterns
func NewRedactor(patterns []string) *Redactor {
	compiled := make([]*regexp.Regexp, 0, len(patterns))
	for _, p := range patterns {
		// Case-insensitive matching
		pattern := regexp.MustCompile("(?i)" + regexp.QuoteMeta(p))
		compiled = append(compiled, pattern)
	}
	
	return &Redactor{
		patterns: patterns,
		compiled:  compiled,
	}
}

// Redact redacts sensitive information from a string
func (r *Redactor) Redact(s string) string {
	if r == nil {
		return s
	}
	
	for _, pattern := range r.compiled {
		s = pattern.ReplaceAllString(s, "[REDACTED]")
	}
	
	return s
}

// RedactMap redacts sensitive information from a map
func (r *Redactor) RedactMap(m map[string]interface{}) map[string]interface{} {
	if r == nil {
		return m
	}
	
	result := make(map[string]interface{}, len(m))
	for k, v := range m {
		if r.isSensitive(k) {
			result[k] = "[REDACTED]"
		} else if str, ok := v.(string); ok {
			result[k] = r.Redact(str)
		} else if m2, ok := v.(map[string]interface{}); ok {
			result[k] = r.RedactMap(m2)
		} else {
			result[k] = v
		}
	}
	return result
}

// isSensitive checks if a key matches sensitive patterns
func (r *Redactor) isSensitive(key string) bool {
	if r == nil {
		return false
	}
	
	keyLower := strings.ToLower(key)
	for _, pattern := range r.compiled {
		if pattern.MatchString(keyLower) {
			return true
		}
	}
	return false
}

// DefaultRedactor creates a redactor with default sensitive patterns
func DefaultRedactor() *Redactor {
	return NewRedactor([]string{
		"password",
		"passwd",
		"pwd",
		"token",
		"api_key",
		"apikey",
		"secret",
		"private_key",
		"privatekey",
		"access_token",
		"accesstoken",
		"refresh_token",
		"refreshtoken",
		"authorization",
		"auth",
		"credential",
		"ssn",
		"credit_card",
		"creditcard",
	})
}

// SensitiveKeys returns a list of default sensitive keys
func SensitiveKeys() []string {
	return []string{
		"password",
		"token",
		"api_key",
		"secret",
		"private_key",
		"access_token",
		"refresh_token",
		"authorization",
		"credential",
		"ssn",
		"credit_card",
	}
}
